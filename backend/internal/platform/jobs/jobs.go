// Package jobs implementa el registro de trabajos en PostgreSQL: el patrón
// outbox del ADR-0004 y el reclamo atómico del ADR-0008.
//
// La cola vive en Redis, pero la verdad sobre qué trabajos existen y en qué
// estado están vive en PostgreSQL. Es lo que permite que perder Redis no
// pierda trabajo (recomendación §11 del enunciado).
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"mooc/backend/internal/platform/dbx"
)

// Colas por prioridad (ADR-0004).
const (
	QueueCritical = "critical"
	QueueDefault  = "default"
	QueueBulk     = "bulk"
)

// Catálogo de tipos de trabajo. Ver disenos/trabajos-asincronos.md.
const (
	TypeEmailSend = "email.send"
)

// Estados de platform.job_runs.
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusDead      = "dead"
)

// MaxRetry es el número de reintentos antes de la DLQ (RNF-06, enunciado §6).
const MaxRetry = 3

// Job es un trabajo por encolar.
type Job struct {
	// Key es determinista: identifica la intención, no el momento (ADR-0008).
	Key     string
	Type    string
	Queue   string
	Payload map[string]any
}

func (j Job) payloadJSON() ([]byte, error) {
	if j.Payload == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(j.Payload)
}

// Record inserta la fila del trabajo. Debe ejecutarse en la MISMA transacción
// que el cambio de dominio que lo origina; publicar en la cola va después del
// commit. Si esa publicación falla, jobs.reaper recupera la fila 'queued'.
//
// Si la clave ya existe no se toca nada: encolar dos veces el mismo trabajo no
// reabre uno que ya terminó.
func Record(ctx context.Context, db dbx.DB, j Job) error {
	payload, err := j.payloadJSON()
	if err != nil {
		return fmt.Errorf("serializar payload de %s: %w", j.Key, err)
	}
	_, err = db.Exec(ctx, `
		INSERT INTO platform.job_runs (job_key, type, queue, status, attempt, payload)
		VALUES ($1, $2, $3, 'queued', 0, $4)
		ON CONFLICT (job_key) DO NOTHING`,
		j.Key, j.Type, j.Queue, payload)
	if err != nil {
		return fmt.Errorf("registrar trabajo %s: %w", j.Key, err)
	}
	return nil
}

// TryClaim reclama la ejecución de forma atómica. Devuelve false cuando otro
// worker ya lo tiene o cuando el trabajo ya terminó: es lo que hace que una
// entrega duplicada no produzca dos salidas (CA-03).
func TryClaim(ctx context.Context, db dbx.DB, jobKey string) (bool, error) {
	var claimed bool
	err := db.QueryRow(ctx, `
		UPDATE platform.job_runs
		   SET status       = 'running',
		       attempt      = attempt + 1,
		       heartbeat_at = now(),
		       started_at   = coalesce(started_at, now())
		 WHERE job_key = $1
		   AND status IN ('queued', 'failed')
		RETURNING true`, jobKey).Scan(&claimed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Ninguna fila reclamable: el trabajo ya terminó o lo tiene otro.
			return false, nil
		}
		return false, fmt.Errorf("reclamar trabajo %s: %w", jobKey, err)
	}
	return claimed, nil
}

// Heartbeat señala que el trabajo sigue vivo. Sin esto, jobs.reaper lo
// reencolaría a los 5 minutos (ADR-0004).
func Heartbeat(ctx context.Context, db dbx.DB, jobKey string) error {
	_, err := db.Exec(ctx,
		`UPDATE platform.job_runs SET heartbeat_at = now() WHERE job_key = $1`, jobKey)
	return err
}

func Succeed(ctx context.Context, db dbx.DB, jobKey string) error {
	_, err := db.Exec(ctx, `
		UPDATE platform.job_runs
		   SET status = 'succeeded', finished_at = now(), last_error = NULL
		 WHERE job_key = $1`, jobKey)
	return err
}

// Fail marca el intento fallido. El trabajo vuelve a 'queued' para que asynq lo
// reintente con backoff; TryClaim lo aceptará de nuevo porque 'failed' está en
// la lista de estados reclamables.
func Fail(ctx context.Context, db dbx.DB, jobKey, reason string) error {
	_, err := db.Exec(ctx, `
		UPDATE platform.job_runs
		   SET status = 'failed', last_error = $2
		 WHERE job_key = $1`, jobKey, reason)
	return err
}

// Kill mueve el trabajo a la DLQ. A partir de aquí solo lo revive un reencolado
// administrativo, que conserva la misma job_key (SEG-4).
func Kill(ctx context.Context, db dbx.DB, jobKey, reason string) error {
	_, err := db.Exec(ctx, `
		UPDATE platform.job_runs
		   SET status = 'dead', finished_at = now(), last_error = $2
		 WHERE job_key = $1`, jobKey, reason)
	return err
}
