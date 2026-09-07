package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/modules/admin"
	"mooc/backend/internal/platform/dbx"
)

type AdminStore struct{ pool *pgxpool.Pool }

func NewAdminStore(pool *pgxpool.Pool) *AdminStore { return &AdminStore{pool: pool} }

func (s *AdminStore) DB() dbx.DB { return s.pool }

func (s *AdminStore) WithinTx(ctx context.Context, fn func(context.Context, dbx.DB) error) error {
	return withinTx(ctx, s.pool, fn)
}

func nfad(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return admin.ErrNotFound
	}
	return err
}

const colsUsuarioAdmin = `id, email, full_name, role, status, email_verified_at, created_at, updated_at`

func scanUsuario(row pgx.Row) (admin.Usuario, error) {
	var u admin.Usuario
	err := row.Scan(&u.ID, &u.Email, &u.FullName, &u.Role, &u.Status,
		&u.EmailVerifiedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return admin.Usuario{}, nfad(err)
	}
	return u, nil
}

func (s *AdminStore) ListarUsuarios(ctx context.Context, db dbx.DB, f admin.FiltroUsuarios) ([]admin.Usuario, error) {
	rows, err := db.Query(ctx, `
		SELECT `+colsUsuarioAdmin+`
		  FROM identity.users
		 WHERE ($1 = '' OR role = $1)
		   AND ($2 = '' OR status = $2)
		   AND ($3 = '' OR email ILIKE '%'||$3||'%' OR full_name ILIKE '%'||$3||'%')
		 ORDER BY created_at DESC
		 LIMIT $4`, f.Role, f.Status, f.Buscar, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []admin.Usuario
	for rows.Next() {
		u, err := scanUsuario(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *AdminStore) UsuarioPorID(ctx context.Context, db dbx.DB, id uuid.UUID) (admin.Usuario, error) {
	return scanUsuario(db.QueryRow(ctx, `SELECT `+colsUsuarioAdmin+` FROM identity.users WHERE id=$1`, id))
}

func (s *AdminStore) InsertarUsuario(ctx context.Context, db dbx.DB, u admin.Usuario, hash string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO identity.users
		    (id, email, email_verified_at, password_hash, full_name, role, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8)`,
		u.ID, u.Email, u.EmailVerifiedAt, hash, u.FullName, u.Role, u.Status, u.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return admin.ErrCorreoEnUso
		}
		return err
	}
	return nil
}

// OtrosAdminsActivos se consulta con FOR UPDATE sobre las filas implicadas: sin
// el bloqueo, dos peticiones simultáneas verían cada una al otro administrador
// y degradarían a los dos.
func (s *AdminStore) OtrosAdminsActivos(ctx context.Context, db dbx.DB, excepto uuid.UUID) (int, error) {
	var n int
	err := db.QueryRow(ctx, `
		SELECT count(*) FROM (
		    SELECT id FROM identity.users
		     WHERE role='admin' AND status='active' AND id <> $1
		     FOR UPDATE
		) t`, excepto).Scan(&n)
	return n, err
}

func (s *AdminStore) ActualizarUsuario(ctx context.Context, db dbx.DB, id uuid.UUID, c admin.CambioDeUsuario) (admin.Usuario, error) {
	return scanUsuario(db.QueryRow(ctx, `
		UPDATE identity.users
		   SET role       = coalesce($2, role),
		       status     = coalesce($3, status),
		       updated_at = now()
		 WHERE id = $1
		RETURNING `+colsUsuarioAdmin, id, c.Role, c.Status))
}

func (s *AdminStore) InsertarTokenInvitacion(ctx context.Context, db dbx.DB, id, userID uuid.UUID, hash []byte, ttlHoras int) error {
	_, err := db.Exec(ctx, `
		INSERT INTO identity.one_time_tokens (id, user_id, purpose, token_sha256, expires_at)
		VALUES ($1, $2, 'password_reset', $3, now() + make_interval(hours => $4))`,
		id, userID, hash, ttlHoras)
	return err
}

func (s *AdminStore) SesionesDe(ctx context.Context, db dbx.DB, userID uuid.UUID) ([]admin.Sesion, error) {
	rows, err := db.Query(ctx, `
		SELECT id, ip, user_agent, created_at, last_seen_at, expires_at, revoked_at
		  FROM identity.sessions WHERE user_id=$1 ORDER BY created_at DESC LIMIT 50`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []admin.Sesion
	for rows.Next() {
		var s admin.Sesion
		if err := rows.Scan(&s.ID, &s.IP, &s.UserAgent, &s.CreatedAt,
			&s.LastSeenAt, &s.ExpiresAt, &s.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (s *AdminStore) RevocarSesionesDe(ctx context.Context, db dbx.DB, userID uuid.UUID) (int, error) {
	tag, err := db.Exec(ctx,
		`UPDATE identity.sessions SET revoked_at = now() WHERE user_id=$1 AND revoked_at IS NULL`, userID)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (s *AdminStore) Auditoria(ctx context.Context, db dbx.DB, f admin.FiltroAuditoria) ([]admin.EventoAuditoria, error) {
	rows, err := db.Query(ctx, `
		SELECT id, occurred_at, actor_id, actor_role, action, entity_type, entity_id, ip, trace_id, metadata
		  FROM audit.events
		 WHERE ($1::uuid IS NULL OR actor_id = $1)
		   AND ($2 = '' OR action = $2)
		   AND ($3 = '' OR entity_type = $3)
		   AND ($4::uuid IS NULL OR entity_id = $4)
		 ORDER BY id DESC
		 LIMIT $5`, f.ActorID, f.Action, f.EntityType, f.EntityID, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []admin.EventoAuditoria
	for rows.Next() {
		var e admin.EventoAuditoria
		var meta []byte
		if err := rows.Scan(&e.ID, &e.OccurredAt, &e.ActorID, &e.ActorRole, &e.Action,
			&e.EntityType, &e.EntityID, &e.IP, &e.TraceID, &meta); err != nil {
			return nil, err
		}
		if len(meta) > 0 {
			_ = json.Unmarshal(meta, &e.Metadata)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

const colsTrabajo = `id, job_key, type, queue, status, attempt, last_error,
	heartbeat_at, created_at, finished_at`

func scanTrabajo(row pgx.Row) (admin.Trabajo, error) {
	var t admin.Trabajo
	err := row.Scan(&t.ID, &t.JobKey, &t.Type, &t.Queue, &t.Status, &t.Attempt,
		&t.LastError, &t.HeartbeatAt, &t.CreatedAt, &t.FinishedAt)
	if err != nil {
		return admin.Trabajo{}, nfad(err)
	}
	return t, nil
}

func (s *AdminStore) Trabajos(ctx context.Context, db dbx.DB, f admin.FiltroTrabajos) ([]admin.Trabajo, error) {
	rows, err := db.Query(ctx, `
		SELECT `+colsTrabajo+`
		  FROM platform.job_runs
		 WHERE ($1 = '' OR status = $1)
		   AND ($2 = '' OR type = $2)
		 ORDER BY id DESC LIMIT $3`, f.Status, f.Type, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []admin.Trabajo
	for rows.Next() {
		t, err := scanTrabajo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *AdminStore) TrabajoPorID(ctx context.Context, db dbx.DB, id int64) (admin.Trabajo, error) {
	return scanTrabajo(db.QueryRow(ctx, `SELECT `+colsTrabajo+` FROM platform.job_runs WHERE id=$1`, id))
}

// ReencolarTrabajo devuelve la fila a 'queued' CONSERVANDO la job_key. El
// created_at se retrasa para que el reaper la recoja de inmediato si la
// publicación en la cola fallara.
func (s *AdminStore) ReencolarTrabajo(ctx context.Context, db dbx.DB, id int64) (admin.Trabajo, error) {
	return scanTrabajo(db.QueryRow(ctx, `
		UPDATE platform.job_runs
		   SET status='queued', last_error=NULL, finished_at=NULL,
		       created_at = now() - interval '1 hour'
		 WHERE id=$1
		RETURNING `+colsTrabajo, id))
}
