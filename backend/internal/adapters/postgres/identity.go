package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/modules/identity"
	"mooc/backend/internal/platform/dbx"
)

// IdentityStore implementa identity.Store (ADR-0001: los adaptadores
// implementan los puertos del dominio).
type IdentityStore struct {
	pool *pgxpool.Pool
}

func NewIdentityStore(pool *pgxpool.Pool) *IdentityStore { return &IdentityStore{pool: pool} }

func (s *IdentityStore) DB() dbx.DB { return s.pool }

func (s *IdentityStore) WithinTx(ctx context.Context, fn func(ctx context.Context, tx dbx.DB) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("abrir transacción: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("confirmar transacción: %w", err)
	}
	return nil
}

const userColumns = `id, email, email_verified_at, password_hash, full_name, role, status, created_at, updated_at`

func scanUser(row pgx.Row) (identity.User, error) {
	var u identity.User
	err := row.Scan(&u.ID, &u.Email, &u.EmailVerifiedAt, &u.PasswordHash,
		&u.FullName, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.User{}, identity.ErrNotFound
		}
		return identity.User{}, err
	}
	return u, nil
}

func (s *IdentityStore) InsertUser(ctx context.Context, db dbx.DB, u identity.User) error {
	_, err := db.Exec(ctx, `
		INSERT INTO identity.users
		    (id, email, password_hash, full_name, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		u.ID, u.Email, u.PasswordHash, u.FullName, u.Role, u.Status, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return identity.ErrEmailTaken
		}
		return fmt.Errorf("insertar usuario: %w", err)
	}
	return nil
}

func (s *IdentityStore) UserByEmail(ctx context.Context, db dbx.DB, email string) (identity.User, error) {
	return scanUser(db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM identity.users WHERE email = $1`, email))
}

func (s *IdentityStore) UserByID(ctx context.Context, db dbx.DB, id uuid.UUID) (identity.User, error) {
	return scanUser(db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM identity.users WHERE id = $1`, id))
}

func (s *IdentityStore) MarkEmailVerified(ctx context.Context, db dbx.DB, userID uuid.UUID, at time.Time) error {
	tag, err := db.Exec(ctx, `
		UPDATE identity.users
		   SET email_verified_at = coalesce(email_verified_at, $2), updated_at = $2
		 WHERE id = $1`, userID, at)
	if err != nil {
		return fmt.Errorf("marcar correo verificado: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return identity.ErrNotFound
	}
	return nil
}

func (s *IdentityStore) InsertOneTimeToken(ctx context.Context, db dbx.DB, t identity.OneTimeToken) error {
	_, err := db.Exec(ctx, `
		INSERT INTO identity.one_time_tokens (id, user_id, purpose, token_sha256, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		t.ID, t.UserID, t.Purpose, t.TokenSHA256, t.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insertar token de un solo uso: %w", err)
	}
	return nil
}

// ConsumeOneTimeToken marca y devuelve el token en una sola sentencia: dos
// peticiones simultáneas con el mismo token solo pueden ganar una.
func (s *IdentityStore) ConsumeOneTimeToken(ctx context.Context, db dbx.DB, purpose string, hash []byte, now time.Time) (identity.OneTimeToken, error) {
	var t identity.OneTimeToken
	err := db.QueryRow(ctx, `
		UPDATE identity.one_time_tokens
		   SET consumed_at = $4
		 WHERE purpose = $1
		   AND token_sha256 = $2
		   AND consumed_at IS NULL
		   AND expires_at > $3
		RETURNING id, user_id, purpose, token_sha256, expires_at, consumed_at`,
		purpose, hash, now, now).
		Scan(&t.ID, &t.UserID, &t.Purpose, &t.TokenSHA256, &t.ExpiresAt, &t.ConsumedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.OneTimeToken{}, identity.ErrTokenInvalid
		}
		return identity.OneTimeToken{}, fmt.Errorf("consumir token: %w", err)
	}
	return t, nil
}

func (s *IdentityStore) InsertSession(ctx context.Context, db dbx.DB, sess identity.Session) error {
	_, err := db.Exec(ctx, `
		INSERT INTO identity.sessions
		    (id, user_id, token_sha256, ip, user_agent, created_at, last_seen_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		sess.ID, sess.UserID, sess.TokenSHA256, nullIfEmpty(sess.IP), nullIfEmpty(sess.UserAgent),
		sess.CreatedAt, sess.LastSeenAt, sess.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insertar sesión: %w", err)
	}
	return nil
}

func (s *IdentityStore) RevokeSession(ctx context.Context, db dbx.DB, id uuid.UUID, at time.Time) error {
	_, err := db.Exec(ctx,
		`UPDATE identity.sessions SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`, id, at)
	if err != nil {
		return fmt.Errorf("revocar sesión: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
