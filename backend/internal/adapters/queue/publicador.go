package queue

import (
	"context"

	"mooc/backend/internal/platform/jobs"
)

// Publicador deja un trabajo ya registrado en platform.job_runs en manos del
// transporte. Hay dos: asynq sobre Redis (local y Entrega 3) y Pub/Sub
// (Cloud Run, Entrega 4). Los módulos no saben cuál corre (ADR-0020).
type Publicador interface {
	Publish(ctx context.Context, j jobs.Job, traceID string) error
	// Republicar dice además si el trabajo entró de verdad al transporte, o si
	// ya estaba ahí esperando. Lo usa el reaper para no contar copias.
	Republicar(ctx context.Context, j jobs.Job, traceID string) (bool, error)
	Close() error
}

var _ Publicador = (*Publisher)(nil)
