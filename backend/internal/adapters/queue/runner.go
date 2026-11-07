package queue

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/platform/jobs"
	"mooc/backend/internal/platform/metrics"
)

// Handler es lo que escribe cada módulo: recibe los datos del trabajo y hace
// el trabajo. No sabe nada de reclamos, heartbeats ni reintentos.
type Handler func(ctx context.Context, data map[string]any) error

// Runner envuelve un Handler con las garantías del ADR-0008: reclamo atómico
// antes de trabajar, heartbeat mientras trabaja, y registro del resultado.
type Runner struct {
	db  *pgxpool.Pool
	log *slog.Logger

	// activos cuenta los trabajos que aún no registraron su final, para que
	// el worker no salga antes de anotarlo (ver Esperar).
	activos sync.WaitGroup
}

func NewRunner(db *pgxpool.Pool, log *slog.Logger) *Runner {
	return &Runner{db: db, log: log}
}

// Resultado es lo que pasó con una entrega, sin importar el transporte. asynq y
// la suscripción push de Pub/Sub lo traducen a su manera de confirmar o
// reintentar.
type Resultado int

const (
	// Completado: el trabajo terminó y su estado quedó persistido.
	Completado Resultado = iota
	// YaTerminado: otra entrega ya lo completó o lo mandó a la DLQ. Se confirma
	// sin repetirlo (CA-03).
	YaTerminado
	// EnCurso: otro worker lo tiene reclamado y sigue latiendo. Se reintenta
	// más tarde; si ese worker muere, el latido vence y esta entrega lo reclama.
	EnCurso
	// Fallido: el manejador falló o no se pudo reclamar. Se reintenta.
	Fallido
	// Ilegible: el cuerpo no es un trabajo. Reintentar no lo arregla.
	Ilegible
)

// Ejecutar es el núcleo de una entrega: reclamo atómico, latido mientras
// trabaja y registro del resultado (ADR-0008). Devuelve el error del
// manejador cuando el resultado es Fallido.
func (r *Runner) Ejecutar(ctx context.Context, jobType string, h Handler, cuerpo []byte) (Resultado, error) {
	r.activos.Add(1)
	defer r.activos.Done()

	var p Payload
	if err := json.Unmarshal(cuerpo, &p); err != nil || p.JobKey == "" {
		r.log.Error("payload ilegible", "type", jobType, "error", err)
		return Ilegible, err
	}
	log := r.log.With("job_key", p.JobKey, "type", jobType, "trace_id", p.TraceID)

	claimed, err := jobs.TryClaim(ctx, r.db, p.JobKey)
	if err != nil {
		return Fallido, err // transitorio: que el transporte reintente
	}
	if !claimed {
		return r.noReclamado(ctx, log, jobType, p.JobKey)
	}

	stop := r.heartbeat(ctx, p.JobKey)
	defer stop()

	log.Info("trabajo iniciado")
	start := time.Now()
	if err := h(ctx, p.Data); err != nil {
		_ = jobs.Fail(context.WithoutCancel(ctx), r.db, p.JobKey, err.Error())
		metrics.JobsProcessed.WithLabelValues(jobType, "error").Inc()
		metrics.JobDuration.WithLabelValues(jobType).Observe(time.Since(start).Seconds())
		if errors.Is(ctx.Err(), context.Canceled) {
			// Lo cortó el apagado del worker o la cancelación del transporte,
			// no un fallo suyo: queda reclamable y se reintenta en otra
			// instancia (ADR-0019, D6). El vencimiento del Timeout de la tarea
			// es DeadlineExceeded y sí cuenta como fallo.
			log.Warn("trabajo interrumpido; vuelve a la cola",
				"error", err, "duration_ms", time.Since(start).Milliseconds())
			return Fallido, err
		}
		log.Error("trabajo fallido", "error", err, "duration_ms", time.Since(start).Milliseconds())
		return Fallido, err
	}

	if err := jobs.Succeed(context.WithoutCancel(ctx), r.db, p.JobKey); err != nil {
		// Sin el registro no se confirma: la reentrega lo encontrará en
		// 'running', y al vencer el latido lo repetirá de forma idempotente.
		log.Error("no se pudo marcar como exitoso", "error", err)
		return Fallido, err
	}
	metrics.JobsProcessed.WithLabelValues(jobType, "ok").Inc()
	metrics.JobDuration.WithLabelValues(jobType).Observe(time.Since(start).Seconds())
	log.Info("trabajo completado", "duration_ms", time.Since(start).Milliseconds())
	return Completado, nil
}

// noReclamado decide qué hacer con una entrega que no pudo reclamar el trabajo.
func (r *Runner) noReclamado(ctx context.Context, log *slog.Logger, jobType, jobKey string) (Resultado, error) {
	estado, existe, err := jobs.Estado(ctx, r.db, jobKey)
	if err != nil {
		return Fallido, err
	}
	metrics.JobsDuplicateSkipped.WithLabelValues(jobType).Inc()
	if !existe || jobs.Terminado(estado) {
		log.Info("entrega repetida de un trabajo terminado; se confirma sin repetirlo", "estado", estado)
		return YaTerminado, nil
	}
	log.Info("el trabajo lo tiene otro worker; se reintentará", "estado", estado)
	return EnCurso, nil
}

// Matar manda el trabajo a la DLQ cuando se agotaron sus entregas, y deja el
// rastro que exige el enunciado §6: la métrica que dispara la alerta y el log.
func (r *Runner) Matar(ctx context.Context, jobType string, cuerpo []byte, intentos int, causa error) {
	var p Payload
	_ = json.Unmarshal(cuerpo, &p)
	motivo := "reintentos agotados"
	if causa != nil {
		motivo = causa.Error()
	}
	if err := jobs.Kill(context.WithoutCancel(ctx), r.db, p.JobKey, motivo); err != nil {
		r.log.Error("no se pudo mover a la DLQ", "job_key", p.JobKey, "error", err)
	}
	metrics.JobsDeadLetter.WithLabelValues(jobType).Inc()
	r.log.Error("ALERTA: trabajo en la dead-letter queue",
		"job_key", p.JobKey, "type", jobType, "attempts", intentos, "error", motivo)
}

// Wrap adapta Ejecutar a asynq. Una entrega repetida se descarta como antes:
// asynq ya garantiza que no hay dos copias vivas de la tarea (TaskID).
func (r *Runner) Wrap(jobType string, h Handler) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		res, err := r.Ejecutar(ctx, jobType, h, t.Payload())
		switch res {
		case Ilegible:
			return asynq.SkipRetry
		case Fallido:
			return err
		default:
			return nil
		}
	}
}

// Esperar bloquea hasta que todos los trabajos en curso hayan registrado su
// final, o hasta plazo. Devuelve false si se agotó el tiempo.
func (r *Runner) Esperar(plazo time.Duration) bool {
	listo := make(chan struct{})
	go func() {
		r.activos.Wait()
		close(listo)
	}()
	select {
	case <-listo:
		return true
	case <-time.After(plazo):
		return false
	}
}

func (r *Runner) heartbeat(ctx context.Context, jobKey string) func() {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				if err := jobs.Heartbeat(context.WithoutCancel(ctx), r.db, jobKey); err != nil {
					r.log.Warn("heartbeat fallido", "job_key", jobKey, "error", err)
				}
			}
		}
	}()
	return func() { close(done) }
}

// OnRetriesExhausted mueve el trabajo a la DLQ y deja el rastro que exige el
// enunciado §6: "tras tres reintentos fallidos, el trabajo llega a la DLQ y
// emite una alerta".
func (r *Runner) OnRetriesExhausted() asynq.ErrorHandlerFunc {
	return func(ctx context.Context, t *asynq.Task, err error) {
		retried, _ := asynq.GetRetryCount(ctx)
		maxRetry, _ := asynq.GetMaxRetry(ctx)
		if retried < maxRetry {
			return // aún quedan reintentos; el backoff sigue su curso
		}
		r.Matar(ctx, t.Type(), t.Payload(), retried+1, err)
	}
}
