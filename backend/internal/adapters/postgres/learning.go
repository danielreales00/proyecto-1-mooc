package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/modules/learning"
	"mooc/backend/internal/platform/dbx"
)

type LearningStore struct{ pool *pgxpool.Pool }

func NewLearningStore(pool *pgxpool.Pool) *LearningStore { return &LearningStore{pool: pool} }

func (s *LearningStore) DB() dbx.DB { return s.pool }

func (s *LearningStore) WithinTx(ctx context.Context, fn func(context.Context, dbx.DB) error) error {
	return withinTx(ctx, s.pool, fn)
}

func nf(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return learning.ErrNotFound
	}
	return err
}

// SearchCatalog solo devuelve cursos con una versión publicada.
// SearchCatalog pide un elemento MÁS del límite: si aparece, hay página
// siguiente, y así se sabe sin contar el total (que en una tabla grande es la
// consulta más cara).
func (s *LearningStore) SearchCatalog(ctx context.Context, db dbx.DB, q, categoria, idioma string,
	limit int, desde *learning.Cursor) ([]learning.CatalogCourse, error) {

	var fechaDesde any
	var idDesde any
	if desde != nil {
		fechaDesde, idDesde = desde.Fecha, desde.ID
	}
	rows, err := db.Query(ctx, `
		SELECT c.id, c.slug, c.title, c.summary, coalesce(c.category,''), coalesce(c.language,''),
		       v.version_number, c.created_at,
		       (SELECT count(*) FROM progress.enrollments e
		         WHERE e.course_id = c.id AND e.status='active')
		  FROM authoring.courses c
		  JOIN authoring.course_versions v ON v.id = c.current_version_id
		 WHERE c.status = 'published'
		   AND ($1 = '' OR to_tsvector('spanish', c.title || ' ' || c.summary) @@ plainto_tsquery('spanish', $1))
		   AND ($2 = '' OR c.category = $2)
		   AND ($3 = '' OR c.language = $3)
		   AND ($5::timestamptz IS NULL OR (c.created_at, c.id) < ($5, $6::uuid))
		 ORDER BY c.created_at DESC, c.id DESC
		 LIMIT $4`, q, categoria, idioma, limit+1, fechaDesde, idDesde)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []learning.CatalogCourse
	for rows.Next() {
		var c learning.CatalogCourse
		if err := rows.Scan(&c.ID, &c.Slug, &c.Title, &c.Summary, &c.Category,
			&c.Language, &c.VersionNumber, &c.CreatedAt, &c.EnrolledCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *LearningStore) CatalogBySlug(ctx context.Context, db dbx.DB, slug string) (learning.CatalogCourse, error) {
	var c learning.CatalogCourse
	var versionID uuid.UUID
	err := db.QueryRow(ctx, `
		SELECT c.id, c.slug, c.title, c.summary, coalesce(c.category,''), coalesce(c.language,''),
		       v.version_number, v.id,
		       (SELECT count(*) FROM progress.enrollments e
		         WHERE e.course_id = c.id AND e.status='active')
		  FROM authoring.courses c
		  JOIN authoring.course_versions v ON v.id = c.current_version_id
		 WHERE c.slug = $1 AND c.status = 'published'`, slug).
		Scan(&c.ID, &c.Slug, &c.Title, &c.Summary, &c.Category, &c.Language,
			&c.VersionNumber, &versionID, &c.EnrolledCount)
	if err != nil {
		return learning.CatalogCourse{}, nf(err)
	}

	// Esquema público: solo títulos, sin contenido ni binarios.
	rows, err := db.Query(ctx, `
		SELECT m.title, coalesce(u.title,'')
		  FROM authoring.modules m
		  LEFT JOIN authoring.units u ON u.module_id = m.id
		 WHERE m.course_version_id = $1
		 ORDER BY m.position, u.position`, versionID)
	if err != nil {
		return learning.CatalogCourse{}, err
	}
	defer rows.Close()
	indice := map[string]int{}
	for rows.Next() {
		var modulo, unidad string
		if err := rows.Scan(&modulo, &unidad); err != nil {
			return learning.CatalogCourse{}, err
		}
		i, ok := indice[modulo]
		if !ok {
			indice[modulo] = len(c.Outline)
			c.Outline = append(c.Outline, learning.OutlineModule{Title: modulo})
			i = len(c.Outline) - 1
		}
		if unidad != "" {
			c.Outline[i].Units = append(c.Outline[i].Units, unidad)
		}
	}
	return c, rows.Err()
}

func (s *LearningStore) CurrentPublishedVersion(ctx context.Context, db dbx.DB, courseID uuid.UUID) (uuid.UUID, int, float64, float64, error) {
	var id uuid.UUID
	var n int
	var pct, nota float64
	err := db.QueryRow(ctx, `
		SELECT v.id, v.version_number,
		       coalesce((v.approval_criteria->>'required_completion_pct')::numeric, 100),
		       coalesce((v.approval_criteria->>'min_quiz_score')::numeric, 0)
		  FROM authoring.courses c
		  JOIN authoring.course_versions v ON v.id = c.current_version_id
		 WHERE c.id = $1 AND v.status = 'published'`, courseID).Scan(&id, &n, &pct, &nota)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, 0, 0, 0, learning.ErrNoPublicado
	}
	return id, n, pct, nota, err
}

const colsInscripcion = `id, user_id, course_id, status, state, progress_pct,
	approved_version_id, enrolled_at, withdrawn_at, completed_at, approved_at`

func scanInscripcion(row pgx.Row) (learning.Enrollment, error) {
	var e learning.Enrollment
	err := row.Scan(&e.ID, &e.UserID, &e.CourseID, &e.Status, &e.State, &e.ProgressPct,
		&e.ApprovedVersionID, &e.EnrolledAt, &e.WithdrawnAt, &e.CompletedAt, &e.ApprovedAt)
	if err != nil {
		return learning.Enrollment{}, nf(err)
	}
	return e, nil
}

// UpsertEnrollment crea o reactiva. Reinscribirse conserva el progreso: la
// fila existente se reactiva, no se sustituye (RF-10).
func (s *LearningStore) UpsertEnrollment(ctx context.Context, db dbx.DB, e learning.Enrollment) (learning.Enrollment, error) {
	return scanInscripcion(db.QueryRow(ctx, `
		INSERT INTO progress.enrollments (id, user_id, course_id, status, state, enrolled_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (user_id, course_id) DO UPDATE
		   SET status = 'active', withdrawn_at = NULL
		RETURNING `+colsInscripcion,
		e.ID, e.UserID, e.CourseID, e.Status, e.State, e.EnrolledAt))
}

func (s *LearningStore) EnrollmentByID(ctx context.Context, db dbx.DB, id uuid.UUID) (learning.Enrollment, error) {
	return scanInscripcion(db.QueryRow(ctx, `SELECT `+colsInscripcion+` FROM progress.enrollments WHERE id=$1`, id))
}

func (s *LearningStore) EnrollmentsByUser(ctx context.Context, db dbx.DB, userID uuid.UUID) ([]learning.Enrollment, error) {
	rows, err := db.Query(ctx, `SELECT `+colsInscripcion+` FROM progress.enrollments WHERE user_id=$1 ORDER BY enrolled_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []learning.Enrollment
	for rows.Next() {
		e, err := scanInscripcion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *LearningStore) UpdateEnrollment(ctx context.Context, db dbx.DB, e learning.Enrollment) error {
	_, err := db.Exec(ctx, `
		UPDATE progress.enrollments
		   SET status=$2, state=$3, progress_pct=$4, approved_version_id=$5,
		       withdrawn_at=$6, completed_at=$7, approved_at=$8
		 WHERE id=$1`,
		e.ID, e.Status, e.State, e.ProgressPct, e.ApprovedVersionID,
		e.WithdrawnAt, e.CompletedAt, e.ApprovedAt)
	return err
}

func (s *LearningStore) ResourcesOfVersion(ctx context.Context, db dbx.DB, versionID uuid.UUID) ([]learning.ResourceRef, error) {
	rows, err := db.Query(ctx, `
		SELECT r.stable_id, r.title, r.type, r.visible, r.required,
		       a.duration_seconds, r.content_md, r.asset_id
		  FROM authoring.resources r
		  LEFT JOIN media.assets a ON a.id = r.asset_id
		  JOIN authoring.units u ON u.id = r.unit_id
		  JOIN authoring.modules m ON m.id = u.module_id
		 WHERE r.course_version_id = $1 AND r.visible
		 ORDER BY m.position, u.position, r.position`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []learning.ResourceRef
	for rows.Next() {
		var r learning.ResourceRef
		if err := rows.Scan(&r.StableID, &r.Title, &r.Type, &r.Visible, &r.Required,
			&r.Duration, &r.ContentMD, &r.AssetID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

const colsAvance = `enrollment_id, resource_stable_id, state, dwell_ms_total,
	last_position_seconds, max_position_seconds, opened_at, completed_at, updated_at`

func (s *LearningStore) ProgressOf(ctx context.Context, db dbx.DB, enrollmentID uuid.UUID) (map[uuid.UUID]learning.ResourceProgress, error) {
	rows, err := db.Query(ctx, `SELECT `+colsAvance+` FROM progress.resource_progress WHERE enrollment_id=$1`, enrollmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]learning.ResourceProgress{}
	for rows.Next() {
		var p learning.ResourceProgress
		if err := rows.Scan(&p.EnrollmentID, &p.ResourceStableID, &p.State, &p.DwellMsTotal,
			&p.LastPositionSeconds, &p.MaxPositionSeconds, &p.OpenedAt, &p.CompletedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out[p.ResourceStableID] = p
	}
	return out, rows.Err()
}

func (s *LearningStore) ProgressOne(ctx context.Context, db dbx.DB, enrollmentID, stableID uuid.UUID) (learning.ResourceProgress, error) {
	var p learning.ResourceProgress
	err := db.QueryRow(ctx, `SELECT `+colsAvance+`
		  FROM progress.resource_progress WHERE enrollment_id=$1 AND resource_stable_id=$2`,
		enrollmentID, stableID).
		Scan(&p.EnrollmentID, &p.ResourceStableID, &p.State, &p.DwellMsTotal,
			&p.LastPositionSeconds, &p.MaxPositionSeconds, &p.OpenedAt, &p.CompletedAt, &p.UpdatedAt)
	if err != nil {
		return learning.ResourceProgress{}, nf(err)
	}
	return p, nil
}

func (s *LearningStore) UpsertProgress(ctx context.Context, db dbx.DB, p learning.ResourceProgress) error {
	_, err := db.Exec(ctx, `
		INSERT INTO progress.resource_progress
		    (enrollment_id, resource_stable_id, state, dwell_ms_total,
		     last_position_seconds, max_position_seconds, opened_at, completed_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (enrollment_id, resource_stable_id) DO UPDATE
		   SET state=$3, dwell_ms_total=$4, last_position_seconds=$5,
		       max_position_seconds=$6, opened_at=coalesce(progress.resource_progress.opened_at,$7),
		       completed_at=coalesce(progress.resource_progress.completed_at,$8), updated_at=$9`,
		p.EnrollmentID, p.ResourceStableID, p.State, p.DwellMsTotal,
		p.LastPositionSeconds, p.MaxPositionSeconds, p.OpenedAt, p.CompletedAt, p.UpdatedAt)
	return err
}

// InsertEvent guarda la evidencia cruda, aceptada o no. Las rechazadas también
// se conservan: son la prueba de CA-05.
func (s *LearningStore) InsertEvent(ctx context.Context, db dbx.DB, enrollmentID, stableID uuid.UUID,
	kind string, pos *float64, clientTS any, aceptada bool, motivo string) error {
	var ts *time.Time
	if t, ok := clientTS.(*time.Time); ok {
		ts = t
	}
	_, err := db.Exec(ctx, `
		INSERT INTO progress.progress_events
		    (enrollment_id, resource_stable_id, kind, position_seconds, client_ts, accepted, reject_reason)
		VALUES ($1,$2,$3,$4,$5,$6,nullif($7,''))`,
		enrollmentID, stableID, kind, pos, ts, aceptada, motivo)
	return err
}

// MinQuizScoreCumplida comprueba que todo quiz obligatorio de la versión tenga
// un intento enviado con nota suficiente.
func (s *LearningStore) MinQuizScoreCumplida(ctx context.Context, db dbx.DB, enrollmentID, versionID uuid.UUID, minima float64) (bool, error) {
	var pendientes int
	err := db.QueryRow(ctx, `
		SELECT count(*)
		  FROM authoring.resources r
		 WHERE r.course_version_id = $2
		   AND r.type = 'quiz' AND r.visible AND r.required
		   AND NOT EXISTS (
		       SELECT 1 FROM assessment.quiz_attempts a
		        WHERE a.enrollment_id = $1
		          AND a.quiz_stable_id = r.stable_id
		          AND a.status IN ('submitted','expired_submitted')
		          AND a.score >= $3)`, enrollmentID, versionID, minima).Scan(&pendientes)
	if err != nil {
		return false, err
	}
	return pendientes == 0, nil
}
