// Package queue publica y consume trabajos en asynq (ADR-0004).
//
// La publicación ocurre siempre DESPUÉS del commit de la transacción que
// registró el trabajo en platform.job_runs. Si falla, la fila queda en 'queued'
// y jobs.reaper la recupera.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
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
	client *asynq.Client
}

func NewPublisher(addr, password string, db int) *Publisher {
	return &Publisher{client: asynq.NewClient(asynq.RedisClientOpt{
		Addr: addr, Password: password, DB: db,
	})}
}

func (p *Publisher) Close() error { return p.client.Close() }

func (p *Publisher) Publish(ctx context.Context, j jobs.Job, traceID string) error {
	body, err := json.Marshal(Payload{JobKey: j.Key, TraceID: traceID, Data: j.Payload})
	if err != nil {
		return fmt.Errorf("serializar tarea %s: %w", j.Key, err)
	}
	_, err = p.client.EnqueueContext(ctx, asynq.NewTask(j.Type, body),
		asynq.Queue(j.Queue),
		asynq.MaxRetry(jobs.MaxRetry),
		asynq.Timeout(30*time.Minute),
	)
	if err != nil {
		return fmt.Errorf("encolar %s: %w", j.Key, err)
	}
	return nil
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
