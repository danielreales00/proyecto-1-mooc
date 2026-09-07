package queue

import (
	"context"
	"encoding/json"
	"log/slog"
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
}

func NewRunner(db *pgxpool.Pool, log *slog.Logger) *Runner {
	return &Runner{db: db, log: log}
}

func (r *Runner) Wrap(jobType string, h Handler) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p Payload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			// Un payload ilegible no mejora con reintentos.
			r.log.Error("payload ilegible", "type", jobType, "error", err)
			return asynq.SkipRetry
		}

		log := r.log.With("job_key", p.JobKey, "type", jobType, "trace_id", p.TraceID)

		claimed, err := jobs.TryClaim(ctx, r.db, p.JobKey)
		if err != nil {
			return err // transitorio: que asynq reintente
		}
		if !claimed {
			// Otro worker lo tiene, o ya terminó. Entrega duplicada: se descarta
			// sin producir una segunda salida (CA-03).
			metrics.JobsDuplicateSkipped.WithLabelValues(jobType).Inc()
			log.Info("trabajo duplicado descartado")
			return nil
		}

		stop := r.heartbeat(ctx, p.JobKey)
		defer stop()

		start := time.Now()
		if err := h(ctx, p.Data); err != nil {
			_ = jobs.Fail(context.WithoutCancel(ctx), r.db, p.JobKey, err.Error())
			metrics.JobsProcessed.WithLabelValues(jobType, "error").Inc()
			metrics.JobDuration.WithLabelValues(jobType).Observe(time.Since(start).Seconds())
			log.Error("trabajo fallido", "error", err, "duration_ms", time.Since(start).Milliseconds())
			return err
		}

		if err := jobs.Succeed(context.WithoutCancel(ctx), r.db, p.JobKey); err != nil {
			log.Error("no se pudo marcar como exitoso", "error", err)
		}
		metrics.JobsProcessed.WithLabelValues(jobType, "ok").Inc()
		metrics.JobDuration.WithLabelValues(jobType).Observe(time.Since(start).Seconds())
		log.Info("trabajo completado", "duration_ms", time.Since(start).Milliseconds())
		return nil
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

		var p Payload
		_ = json.Unmarshal(t.Payload(), &p)
		if killErr := jobs.Kill(context.WithoutCancel(ctx), r.db, p.JobKey, err.Error()); killErr != nil {
			r.log.Error("no se pudo mover a la DLQ", "job_key", p.JobKey, "error", killErr)
		}
		// Esta métrica es la que dispara la alerta que exige el §6 del
		// enunciado: "tras tres reintentos fallidos, el trabajo llega a la DLQ
		// y emite una alerta".
		metrics.JobsDeadLetter.WithLabelValues(t.Type()).Inc()
		r.log.Error("ALERTA: trabajo en la dead-letter queue",
			"job_key", p.JobKey,
			"type", t.Type(),
			"attempts", retried+1,
			"error", err)
	}
}
