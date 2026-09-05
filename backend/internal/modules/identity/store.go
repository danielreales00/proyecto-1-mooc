package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"mooc/backend/internal/platform/dbx"
)

var (
	ErrNotFound     = errors.New("identity: no encontrado")
	ErrEmailTaken   = errors.New("identity: el correo ya está registrado")
	ErrTokenInvalid = errors.New("identity: token inválido, consumido o expirado")
)

// Store es el puerto de persistencia. Lo implementa adapters/postgres.
//
// Cada método recibe el ejecutor, de modo que el servicio decide la frontera
// de la transacción: es ahí donde vive la unidad de trabajo del negocio.
type Store interface {
	// WithinTx ejecuta fn en una transacción y confirma si devuelve nil.
	WithinTx(ctx context.Context, fn func(ctx context.Context, tx dbx.DB) error) error

	// DB devuelve el ejecutor sin transacción, para lecturas sueltas.
	DB() dbx.DB

	InsertUser(ctx context.Context, db dbx.DB, u User) error
	UserByEmail(ctx context.Context, db dbx.DB, email string) (User, error)
	UserByID(ctx context.Context, db dbx.DB, id uuid.UUID) (User, error)
	MarkEmailVerified(ctx context.Context, db dbx.DB, userID uuid.UUID, at time.Time) error

	InsertOneTimeToken(ctx context.Context, db dbx.DB, t OneTimeToken) error
	// ConsumeOneTimeToken marca el token como usado y lo devuelve. Falla con
	// ErrTokenInvalid si no existe, ya se consumió o expiró.
	ConsumeOneTimeToken(ctx context.Context, db dbx.DB, purpose string, hash []byte, now time.Time) (OneTimeToken, error)

	InsertSession(ctx context.Context, db dbx.DB, s Session) error
	RevokeSession(ctx context.Context, db dbx.DB, id uuid.UUID, at time.Time) error
}

// SessionStore es el puerto de sesiones. Lo implementa adapters/sessions sobre
// Redis, que es quien manda sobre la validez de una sesión (ADR-0006).
type SessionStore interface {
	Create(ctx context.Context, token string, s Session, ttl time.Duration) error
	Lookup(ctx context.Context, token string) (Session, error)
	Delete(ctx context.Context, token string) error
	DeleteAllForUser(ctx context.Context, userID uuid.UUID) error
}

// Publisher es el puerto de la cola. La publicación ocurre después del commit.
type Publisher interface {
	Publish(ctx context.Context, j JobRef, traceID string) error
}

// JobRef evita que este módulo importe el paquete de colas para una firma.
type JobRef struct {
	Key     string
	Type    string
	Queue   string
	Payload map[string]any
}
