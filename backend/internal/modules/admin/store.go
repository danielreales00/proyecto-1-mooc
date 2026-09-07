package admin

import (
	"context"

	"github.com/google/uuid"

	"mooc/backend/internal/platform/dbx"
)

// FiltroUsuarios acota el listado.
type FiltroUsuarios struct {
	Role   string
	Status string
	Buscar string
	Limit  int
}

// FiltroAuditoria acota la consulta del rastro.
type FiltroAuditoria struct {
	ActorID    *uuid.UUID
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	Limit      int
}

type FiltroTrabajos struct {
	Status string
	Type   string
	Limit  int
}

type Store interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, tx dbx.DB) error) error
	DB() dbx.DB

	ListarUsuarios(ctx context.Context, db dbx.DB, f FiltroUsuarios) ([]Usuario, error)
	UsuarioPorID(ctx context.Context, db dbx.DB, id uuid.UUID) (Usuario, error)
	InsertarUsuario(ctx context.Context, db dbx.DB, u Usuario, passwordHash string) error
	ActualizarUsuario(ctx context.Context, db dbx.DB, id uuid.UUID, c CambioDeUsuario) (Usuario, error)

	// OtrosAdminsActivos cuenta los administradores activos DISTINTOS del
	// indicado. Se consulta dentro de la transacción del cambio, no antes: de
	// otro modo dos peticiones simultáneas podrían degradar a los dos últimos.
	OtrosAdminsActivos(ctx context.Context, db dbx.DB, excepto uuid.UUID) (int, error)

	InsertarTokenInvitacion(ctx context.Context, db dbx.DB, id, userID uuid.UUID, hash []byte, ttlHoras int) error

	SesionesDe(ctx context.Context, db dbx.DB, userID uuid.UUID) ([]Sesion, error)
	RevocarSesionesDe(ctx context.Context, db dbx.DB, userID uuid.UUID) (int, error)

	Auditoria(ctx context.Context, db dbx.DB, f FiltroAuditoria) ([]EventoAuditoria, error)

	Trabajos(ctx context.Context, db dbx.DB, f FiltroTrabajos) ([]Trabajo, error)
	TrabajoPorID(ctx context.Context, db dbx.DB, id int64) (Trabajo, error)
	ReencolarTrabajo(ctx context.Context, db dbx.DB, id int64) (Trabajo, error)
}

// RevocadorDeSesiones es el puerto hacia el almacén de sesiones de Redis, que
// es quien manda sobre la validez (ADR-0006). Lo satisface identity.
type RevocadorDeSesiones interface {
	DeleteAllForUser(ctx context.Context, userID uuid.UUID) error
}

// Publicador publica en la cola tras el commit.
type Publicador interface {
	Publish(ctx context.Context, key, tipo, cola string, payload map[string]any, traceID string) error
}
