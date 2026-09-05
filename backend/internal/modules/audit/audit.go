// Package audit registra el rastro inmutable de eventos (RF-02, CE-02).
//
// La inmutabilidad la garantiza PostgreSQL con un trigger, no este paquete:
// la regla vive donde están los datos (ADR-0003).
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"mooc/backend/internal/platform/dbx"
)

// Acciones auditadas. La lista crece con cada módulo.
const (
	ActionUserRegistered = "user.registered"
	ActionEmailVerified  = "user.email_verified"
	ActionLoginSucceeded = "auth.login_succeeded"
	ActionLoginFailed    = "auth.login_failed"
	ActionLogout         = "auth.logout"
	ActionSessionRevoked = "auth.session_revoked"
)

type Event struct {
	ActorID    *uuid.UUID
	ActorRole  string
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	IP         string
	UserAgent  string
	TraceID    string
	Metadata   map[string]any
}

type Recorder struct {
	log *slog.Logger
}

func NewRecorder(log *slog.Logger) *Recorder { return &Recorder{log: log} }

// Record inserta el evento con el ejecutor que reciba: pasar la transacción del
// llamante hace que el evento y el cambio auditado se confirmen juntos.
func (r *Recorder) Record(ctx context.Context, db dbx.DB, e Event) error {
	meta := []byte("{}")
	if e.Metadata != nil {
		b, err := json.Marshal(e.Metadata)
		if err != nil {
			return fmt.Errorf("serializar metadata de auditoría: %w", err)
		}
		meta = b
	}
	_, err := db.Exec(ctx, `
		INSERT INTO audit.events
		    (actor_id, actor_role, action, entity_type, entity_id, ip, user_agent, trace_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		e.ActorID, e.ActorRole, e.Action, e.EntityType, e.EntityID,
		nullIfEmpty(e.IP), nullIfEmpty(e.UserAgent), nullIfEmpty(e.TraceID), meta)
	if err != nil {
		return fmt.Errorf("registrar evento de auditoría %s: %w", e.Action, err)
	}
	return nil
}

// RecordBestEffort audita sin propagar el error. Se usa donde perder el evento
// es preferible a fallar la operación del usuario (por ejemplo, un login
// fallido). Todo lo demás debe usar Record dentro de la transacción.
func (r *Recorder) RecordBestEffort(ctx context.Context, db dbx.DB, e Event) {
	if err := r.Record(ctx, db, e); err != nil {
		r.log.Error("no se pudo auditar", "action", e.Action, "error", err)
	}
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
