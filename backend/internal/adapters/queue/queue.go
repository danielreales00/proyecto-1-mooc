// Package queue publica y consume trabajos en asynq (ADR-0004).
//
// La publicación ocurre siempre DESPUÉS del commit de la transacción que
// registró el trabajo en platform.job_runs. Si falla, la fila queda en 'queued'
// y jobs.reaper la recupera.
package queue

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"mooc/backend/internal/platform/jobs"
)

// Payload es lo que viaja dentro de la tarea de asynq. job_key es lo que el
// worker usa para reclamar la ejecución (ADR-0008); trace_id es lo que enlaza
// la traza del worker con la de la petición que lo originó (ADR-0009).
type Payload struct {
	JobKey  string         `json:"job_key"`
	TraceID string         `json:"trace_id,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

type Publisher struct {
	client    *asynq.Client
	inspector *asynq.Inspector
}

// NewPublisher abre el cliente de la cola.
//
// `useTLS` lo exige Memorystore con cifrado en tránsito. Va aquí y no solo en
// rediscli porque asynq abre su propia conexión: sin esto, las sesiones
// hablarían cifrado y la cola no conectaría, que es un fallo raro de
// diagnosticar —todo parece bien hasta que ningún trabajo se ejecuta—.
func NewPublisher(addr, password string, db int, useTLS bool) *Publisher {
	opt := OpcionesRedis(addr, password, db, useTLS)
	return &Publisher{client: asynq.NewClient(opt), inspector: asynq.NewInspector(opt)}
}

// OpcionesRedis construye la configuración que comparten el cliente y el
// servidor de asynq.
func OpcionesRedis(addr, password string, db int, useTLS bool) asynq.RedisClientOpt {
	opt := asynq.RedisClientOpt{Addr: addr, Password: password, DB: db}
	if useTLS {
		// Sin InsecureSkipVerify: un TLS que no verifica no es TLS.
		opt.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: hostDe(addr)}
	}
	return opt
}

// hostDe recorta el puerto: el nombre del certificado es el host, no host:puerto.
func hostDe(addr string) string {
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return addr[:i]
	}
	return addr
}

func (p *Publisher) Close() error {
	return errors.Join(p.inspector.Close(), p.client.Close())
}

// Publish deja el trabajo en la cola. Si ya estaba esperando o en curso, no
// hace nada: para quien publica, el trabajo está en la cola igual.
func (p *Publisher) Publish(ctx context.Context, j jobs.Job, traceID string) error {
	_, err := p.Republicar(ctx, j, traceID)
	return err
}

// Republicar es Publish diciendo si el trabajo entró de verdad en la cola.
// Devuelve false cuando ya estaba en Redis, esperando o en curso.
//
// El ID de la tarea en asynq es la job_key, así que la cola no admite dos
// copias vivas del mismo trabajo. Sin eso, el reaper republicaba cada 30 s lo
// que seguía esperando turno en `bulk`: en la Entrega 2 fueron 2.554 entregas
// duplicadas, y en la Entrega 3 inflarían la señal de escalado de los workers.
//
// Una tarea terminada no deja rastro (asynq la borra al completarla), pero una
// archivada sí: es la que agotó sus reintentos. Esa se borra y se vuelve a
// encolar, que es lo que pide el reencolado administrativo desde la DLQ.
func (p *Publisher) Republicar(ctx context.Context, j jobs.Job, traceID string) (bool, error) {
	body, err := json.Marshal(Payload{JobKey: j.Key, TraceID: traceID, Data: j.Payload})
	if err != nil {
		return false, fmt.Errorf("serializar tarea %s: %w", j.Key, err)
	}
	encolar := func() error {
		_, err := p.client.EnqueueContext(ctx, asynq.NewTask(j.Type, body),
			asynq.TaskID(j.Key),
			asynq.Queue(j.Queue),
			asynq.MaxRetry(jobs.MaxRetry),
			asynq.Timeout(30*time.Minute),
		)
		return err
	}

	err = encolar()
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		info, ierr := p.inspector.GetTaskInfo(j.Queue, j.Key)
		switch {
		case errors.Is(ierr, asynq.ErrTaskNotFound):
			// Terminó entre los dos pasos: ya no hay conflicto.
			err = encolar()
		case ierr != nil:
			return false, fmt.Errorf("consultar %s en la cola: %w", j.Key, ierr)
		case sigueViva(info.State):
			return false, nil
		default:
			if derr := p.inspector.DeleteTask(j.Queue, j.Key); derr != nil &&
				!errors.Is(derr, asynq.ErrTaskNotFound) {
				return false, fmt.Errorf("retirar %s archivada: %w", j.Key, derr)
			}
			err = encolar()
		}
	}
	if err != nil {
		return false, fmt.Errorf("encolar %s: %w", j.Key, err)
	}
	return true, nil
}

// sigueViva dice si una tarea con ese estado acabará ejecutándose sin que
// nadie la vuelva a publicar.
func sigueViva(s asynq.TaskState) bool {
	switch s {
	case asynq.TaskStatePending, asynq.TaskStateActive, asynq.TaskStateScheduled,
		asynq.TaskStateRetry, asynq.TaskStateAggregating:
		return true
	}
	return false
}

// ParseQueues traduce "critical=6,default=3,bulk=1" al mapa de pesos de asynq.
func ParseQueues(spec string) map[string]int {
	out := map[string]int{}
	for _, part := range splitAndTrim(spec, ',') {
		name, weight, ok := cut(part, '=')
		if !ok {
			continue
		}
		// Recortar los dos lados: "critical = 6" es una forma razonable de
		// escribirlo en un .env y no debe descartarse en silencio.
		name, weight = trim(name), trim(weight)
		w := 0
		for _, r := range weight {
			if r < '0' || r > '9' {
				w = 0
				break
			}
			w = w*10 + int(r-'0')
		}
		if name != "" && w > 0 {
			out[name] = w
		}
	}
	if len(out) == 0 {
		out[jobs.QueueDefault] = 1
	}
	return out
}

func splitAndTrim(s string, sep rune) []string {
	var out []string
	start := 0
	for i, r := range s {
		if r == sep {
			out = append(out, trim(s[start:i]))
			start = i + 1
		}
	}
	out = append(out, trim(s[start:]))
	return out
}

func cut(s string, sep rune) (string, string, bool) {
	for i, r := range s {
		if r == sep {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
