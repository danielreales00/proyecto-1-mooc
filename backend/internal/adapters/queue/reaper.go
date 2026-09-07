package queue

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/platform/jobs"
	"mooc/backend/internal/platform/metrics"
)

// Reaper cierra la otra mitad de la garantía del outbox (ADR-0004).
//
// El trabajo se registra en PostgreSQL en la misma transacción que el cambio
// de dominio, y se publica en la cola DESPUÉS del commit. Si esa publicación
// falla —Redis caído, proceso muerto entre commit y publish— la fila queda en
// 'queued' y nadie la ejecutaría. El reaper la recupera.
//
// Recupera dos situaciones:
//
//  1. 'queued' con antigüedad suficiente: se registró pero nunca llegó a la
//     cola, o el reencolado administrativo la devolvió a ese estado.
//  2. 'running' sin heartbeat reciente: el worker que la tenía murió a medias.
//
// Republicar es seguro porque el reclamo atómico impide la doble ejecución
// (ADR-0008).
type Reaper struct {
	db        *pgxpool.Pool
	publisher *Publisher
	log       *slog.Logger

	// EdadMinima evita reencolar algo que se acaba de publicar y aún no ha
	// sido recogido por ningún worker.
	EdadMinima time.Duration
	// SinHeartbeat es el tiempo tras el cual un trabajo 'running' se considera
	// huérfano.
	SinHeartbeat time.Duration
}

func NewReaper(db *pgxpool.Pool, p *Publisher, log *slog.Logger) *Reaper {
	return &Reaper{
		db: db, publisher: p, log: log,
		EdadMinima:   30 * time.Second,
		SinHeartbeat: 5 * time.Minute,
	}
}

type huerfano struct {
	key     string
	tipo    string
	cola    string
	payload map[string]any
}

// Run recupera los trabajos huérfanos. Devuelve cuántos reencoló.
func (r *Reaper) Run(ctx context.Context) (int, error) {
	rows, err := r.db.Query(ctx, `
		SELECT job_key, type, queue, payload
		  FROM platform.job_runs
		 WHERE (status = 'queued' AND created_at < now() - $1::interval)
		    OR (status = 'running' AND heartbeat_at < now() - $2::interval)
		 ORDER BY created_at
		 LIMIT 100`,
		r.EdadMinima.String(), r.SinHeartbeat.String())
	if err != nil {
		return 0, err
	}

	var pendientes []huerfano
	for rows.Next() {
		var h huerfano
		var payload []byte
		if err := rows.Scan(&h.key, &h.tipo, &h.cola, &payload); err != nil {
			rows.Close()
			return 0, err
		}
		if len(payload) > 0 {
			_ = json.Unmarshal(payload, &h.payload)
		}
		pendientes = append(pendientes, h)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	recuperados := 0
	for _, h := range pendientes {
		// Un 'running' huérfano vuelve a 'queued' para que TryClaim lo acepte.
		if _, err := r.db.Exec(ctx, `
			UPDATE platform.job_runs SET status='queued'
			 WHERE job_key=$1 AND status='running'`, h.key); err != nil {
			r.log.Error("no se pudo devolver el trabajo a la cola", "job_key", h.key, "error", err)
			continue
		}
		if err := r.publisher.Publish(ctx, jobs.Job{
			Key: h.key, Type: h.tipo, Queue: h.cola, Payload: h.payload,
		}, ""); err != nil {
			r.log.Error("no se pudo republicar el trabajo", "job_key", h.key, "error", err)
			continue
		}
		recuperados++
		metrics.JobsReaped.Inc()
		r.log.Info("trabajo huérfano recuperado", "job_key", h.key, "type", h.tipo)
	}
	return recuperados, nil
}

// Start arranca el ciclo periódico. Se detiene con el contexto.
func (r *Reaper) Start(ctx context.Context, cada time.Duration) {
	t := time.NewTicker(cada)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				n, err := r.Run(ctx)
				if err != nil {
					r.log.Error("el reaper falló", "error", err)
					continue
				}
				if n > 0 {
					r.log.Info("reaper: trabajos recuperados", "n", n)
				}
			}
		}
	}()
}
