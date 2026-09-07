package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/modules/authoring"
	"mooc/backend/internal/platform/dbx"
)

type AuthoringStore struct{ pool *pgxpool.Pool }

func NewAuthoringStore(pool *pgxpool.Pool) *AuthoringStore { return &AuthoringStore{pool: pool} }

func (s *AuthoringStore) DB() dbx.DB { return s.pool }

func (s *AuthoringStore) WithinTx(ctx context.Context, fn func(context.Context, dbx.DB) error) error {
	return withinTx(ctx, s.pool, fn)
}

// withinTx se comparte entre los stores de este paquete.
func withinTx(ctx context.Context, pool *pgxpool.Pool, fn func(context.Context, dbx.DB) error) error {
	tx, err := pool.Begin(ctx)
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

func traducir(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// restrict_violation es el que usa el trigger de inmutabilidad.
		if pgErr.Code == "2F004" || pgErr.Code == "P0001" ||
			(pgErr.Message != "" && contiene(pgErr.Message, "inmutable")) {
			return authoring.ErrInmutable
		}
	}
	return err
}

func contiene(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

const colsCurso = `id, slug, title, summary, coalesce(category,''), coalesce(language,''),
	owner_id, status, current_version_id, created_at, updated_at`

func scanCurso(row pgx.Row) (authoring.Course, error) {
	var c authoring.Course
	err := row.Scan(&c.ID, &c.Slug, &c.Title, &c.Summary, &c.Category, &c.Language,
		&c.OwnerID, &c.Status, &c.CurrentVersionID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return authoring.Course{}, traducir(err)
	}
	return c, nil
}

func (s *AuthoringStore) InsertCourse(ctx context.Context, db dbx.DB, c authoring.Course) error {
	_, err := db.Exec(ctx, `
		INSERT INTO authoring.courses
		    (id, slug, title, summary, category, language, owner_id, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,nullif($5,''),nullif($6,''),$7,$8,$9,$10)`,
		c.ID, c.Slug, c.Title, c.Summary, c.Category, c.Language, c.OwnerID, c.Status, c.CreatedAt, c.UpdatedAt)
	return traducir(err)
}

func (s *AuthoringStore) UpdateCourse(ctx context.Context, db dbx.DB, c authoring.Course) error {
	_, err := db.Exec(ctx, `
		UPDATE authoring.courses
		   SET title=$2, summary=$3, category=nullif($4,''), language=nullif($5,''), updated_at=$6
		 WHERE id=$1`,
		c.ID, c.Title, c.Summary, c.Category, c.Language, c.UpdatedAt)
	return traducir(err)
}

func (s *AuthoringStore) CourseByID(ctx context.Context, db dbx.DB, id uuid.UUID) (authoring.Course, error) {
	return scanCurso(db.QueryRow(ctx, `SELECT `+colsCurso+` FROM authoring.courses WHERE id=$1`, id))
}

func (s *AuthoringStore) CourseBySlug(ctx context.Context, db dbx.DB, slug string) (authoring.Course, error) {
	return scanCurso(db.QueryRow(ctx, `SELECT `+colsCurso+` FROM authoring.courses WHERE slug=$1`, slug))
}

func (s *AuthoringStore) CoursesByOwner(ctx context.Context, db dbx.DB, owner *uuid.UUID, limit int) ([]authoring.Course, error) {
	rows, err := db.Query(ctx, `
		SELECT `+colsCurso+` FROM authoring.courses
		 WHERE ($1::uuid IS NULL OR owner_id = $1)
		 ORDER BY updated_at DESC LIMIT $2`, owner, limit)
	if err != nil {
		return nil, traducir(err)
	}
	defer rows.Close()
	var out []authoring.Course
	for rows.Next() {
		c, err := scanCurso(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *AuthoringStore) SetCurrentVersion(ctx context.Context, db dbx.DB, courseID, versionID uuid.UUID, status string) error {
	_, err := db.Exec(ctx, `
		UPDATE authoring.courses SET current_version_id=$2, status=$3, updated_at=now() WHERE id=$1`,
		courseID, versionID, status)
	return traducir(err)
}

const colsVersion = `id, course_id, version_number, status, approval_criteria, published_at, created_at`

func scanVersion(row pgx.Row) (authoring.CourseVersion, error) {
	var v authoring.CourseVersion
	var crit []byte
	if err := row.Scan(&v.ID, &v.CourseID, &v.VersionNumber, &v.Status, &crit, &v.PublishedAt, &v.CreatedAt); err != nil {
		return authoring.CourseVersion{}, traducir(err)
	}
	if len(crit) > 0 {
		_ = json.Unmarshal(crit, &v.ApprovalCriteria)
	}
	return v, nil
}

func (s *AuthoringStore) InsertVersion(ctx context.Context, db dbx.DB, v authoring.CourseVersion) error {
	crit, err := json.Marshal(v.ApprovalCriteria)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `
		INSERT INTO authoring.course_versions
		    (id, course_id, version_number, status, approval_criteria, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		v.ID, v.CourseID, v.VersionNumber, v.Status, crit, v.CreatedAt)
	return traducir(err)
}

func (s *AuthoringStore) VersionByNumber(ctx context.Context, db dbx.DB, courseID uuid.UUID, n int) (authoring.CourseVersion, error) {
	return scanVersion(db.QueryRow(ctx,
		`SELECT `+colsVersion+` FROM authoring.course_versions WHERE course_id=$1 AND version_number=$2`, courseID, n))
}

func (s *AuthoringStore) VersionByID(ctx context.Context, db dbx.DB, id uuid.UUID) (authoring.CourseVersion, error) {
	return scanVersion(db.QueryRow(ctx, `SELECT `+colsVersion+` FROM authoring.course_versions WHERE id=$1`, id))
}

func (s *AuthoringStore) VersionsOf(ctx context.Context, db dbx.DB, courseID uuid.UUID) ([]authoring.CourseVersion, error) {
	rows, err := db.Query(ctx,
		`SELECT `+colsVersion+` FROM authoring.course_versions WHERE course_id=$1 ORDER BY version_number DESC`, courseID)
	if err != nil {
		return nil, traducir(err)
	}
	defer rows.Close()
	var out []authoring.CourseVersion
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *AuthoringStore) SetVersionStatus(ctx context.Context, db dbx.DB, versionID uuid.UUID, status string, by *uuid.UUID) error {
	_, err := db.Exec(ctx, `
		UPDATE authoring.course_versions
		   SET status=$2,
		       published_at = CASE WHEN $2='published' THEN coalesce(published_at, now()) ELSE published_at END,
		       published_by = CASE WHEN $2='published' THEN $3 ELSE published_by END
		 WHERE id=$1`, versionID, status, by)
	return traducir(err)
}

// Tree carga el árbol completo en tres consultas, con el estado del binario y
// la salud del quiz ya resueltos: así ValidarPublicacion no necesita tocar la
// base y puede probarse sin ella.
func (s *AuthoringStore) Tree(ctx context.Context, db dbx.DB, versionID uuid.UUID) ([]authoring.Module, error) {
	rows, err := db.Query(ctx,
		`SELECT id, course_version_id, stable_id, position, title
		   FROM authoring.modules WHERE course_version_id=$1 ORDER BY position`, versionID)
	if err != nil {
		return nil, traducir(err)
	}
	var modulos []authoring.Module
	indiceModulo := map[uuid.UUID]int{}
	for rows.Next() {
		var m authoring.Module
		if err := rows.Scan(&m.ID, &m.CourseVersionID, &m.StableID, &m.Position, &m.Title); err != nil {
			rows.Close()
			return nil, err
		}
		indiceModulo[m.ID] = len(modulos)
		modulos = append(modulos, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(modulos) == 0 {
		return nil, nil
	}

	rows, err = db.Query(ctx,
		`SELECT id, module_id, course_version_id, stable_id, position, title
		   FROM authoring.units WHERE course_version_id=$1 ORDER BY position`, versionID)
	if err != nil {
		return nil, traducir(err)
	}
	indiceUnidad := map[uuid.UUID][2]int{}
	for rows.Next() {
		var u authoring.Unit
		if err := rows.Scan(&u.ID, &u.ModuleID, &u.CourseVersionID, &u.StableID, &u.Position, &u.Title); err != nil {
			rows.Close()
			return nil, err
		}
		mi, ok := indiceModulo[u.ModuleID]
		if !ok {
			continue
		}
		indiceUnidad[u.ID] = [2]int{mi, len(modulos[mi].Units)}
		modulos[mi].Units = append(modulos[mi].Units, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = db.Query(ctx, `
		SELECT r.id, r.unit_id, r.course_version_id, r.stable_id, r.position, r.title, r.type,
		       r.visible, r.required, r.downloadable, r.content_md, r.asset_refs, r.asset_id,
		       r.external_url, r.updated_at,
		       coalesce(a.status, ''),
		       coalesce(q.n_preguntas, 0) > 0,
		       coalesce(q.n_preguntas, 0) > 0 AND coalesce(q.n_preguntas, 0) = coalesce(q.n_con_correcta, 0)
		  FROM authoring.resources r
		  LEFT JOIN media.assets a ON a.id = r.asset_id
		  LEFT JOIN (
		        SELECT qq.quiz_id, z.resource_id,
		               count(*) AS n_preguntas,
		               count(*) FILTER (WHERE EXISTS (
		                   SELECT 1 FROM assessment.quiz_options o
		                    WHERE o.question_id = qq.id AND o.is_correct)) AS n_con_correcta
		          FROM assessment.quiz_questions qq
		          JOIN assessment.quizzes z ON z.id = qq.quiz_id
		         GROUP BY qq.quiz_id, z.resource_id
		  ) q ON q.resource_id = r.id
		 WHERE r.course_version_id=$1
		 ORDER BY r.position`, versionID)
	if err != nil {
		return nil, traducir(err)
	}
	defer rows.Close()
	for rows.Next() {
		var r authoring.Resource
		if err := rows.Scan(&r.ID, &r.UnitID, &r.CourseVersionID, &r.StableID, &r.Position, &r.Title,
			&r.Type, &r.Visible, &r.Required, &r.Downloadable, &r.ContentMD, &r.AssetRefs, &r.AssetID,
			&r.ExternalURL, &r.UpdatedAt, &r.AssetStatus, &r.HasQuestions, &r.QuizAllOptionsOK); err != nil {
			return nil, err
		}
		idx, ok := indiceUnidad[r.UnitID]
		if !ok {
			continue
		}
		modulos[idx[0]].Units[idx[1]].Resources = append(modulos[idx[0]].Units[idx[1]].Resources, r)
	}
	return modulos, rows.Err()
}

func (s *AuthoringStore) InsertModule(ctx context.Context, db dbx.DB, m authoring.Module) error {
	_, err := db.Exec(ctx, `
		INSERT INTO authoring.modules (id, course_version_id, stable_id, position, title)
		VALUES ($1,$2,$3,$4,$5)`, m.ID, m.CourseVersionID, m.StableID, m.Position, m.Title)
	return traducir(err)
}

func (s *AuthoringStore) InsertUnit(ctx context.Context, db dbx.DB, u authoring.Unit) error {
	_, err := db.Exec(ctx, `
		INSERT INTO authoring.units (id, module_id, course_version_id, stable_id, position, title)
		VALUES ($1,$2,$3,$4,$5,$6)`, u.ID, u.ModuleID, u.CourseVersionID, u.StableID, u.Position, u.Title)
	return traducir(err)
}

// refs evita enviar NULL a una columna NOT NULL: un recurso sin binarios tiene
// un arreglo vacío, no ausencia de arreglo.
func refs(v []uuid.UUID) []uuid.UUID {
	if v == nil {
		return []uuid.UUID{}
	}
	return v
}

func (s *AuthoringStore) InsertResource(ctx context.Context, db dbx.DB, r authoring.Resource) error {
	_, err := db.Exec(ctx, `
		INSERT INTO authoring.resources
		    (id, unit_id, course_version_id, stable_id, position, title, type,
		     visible, required, downloadable, content_md, asset_refs, asset_id, external_url, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		r.ID, r.UnitID, r.CourseVersionID, r.StableID, r.Position, r.Title, r.Type,
		r.Visible, r.Required, r.Downloadable, r.ContentMD, refs(r.AssetRefs), r.AssetID, r.ExternalURL, r.UpdatedAt)
	return traducir(err)
}

func (s *AuthoringStore) UpdateResource(ctx context.Context, db dbx.DB, r authoring.Resource) error {
	_, err := db.Exec(ctx, `
		UPDATE authoring.resources
		   SET title=$2, visible=$3, required=$4, downloadable=$5,
		       content_md=$6, asset_refs=$7, asset_id=$8, external_url=$9, updated_at=$10
		 WHERE id=$1`,
		r.ID, r.Title, r.Visible, r.Required, r.Downloadable,
		r.ContentMD, refs(r.AssetRefs), r.AssetID, r.ExternalURL, r.UpdatedAt)
	return traducir(err)
}

const colsRecurso = `id, unit_id, course_version_id, stable_id, position, title, type,
	visible, required, downloadable, content_md, asset_refs, asset_id, external_url, updated_at`

func (s *AuthoringStore) ResourceByID(ctx context.Context, db dbx.DB, id uuid.UUID) (authoring.Resource, error) {
	var r authoring.Resource
	err := db.QueryRow(ctx, `SELECT `+colsRecurso+` FROM authoring.resources WHERE id=$1`, id).
		Scan(&r.ID, &r.UnitID, &r.CourseVersionID, &r.StableID, &r.Position, &r.Title, &r.Type,
			&r.Visible, &r.Required, &r.Downloadable, &r.ContentMD, &r.AssetRefs, &r.AssetID,
			&r.ExternalURL, &r.UpdatedAt)
	if err != nil {
		return authoring.Resource{}, traducir(err)
	}
	return r, nil
}

func (s *AuthoringStore) ModuleByID(ctx context.Context, db dbx.DB, id uuid.UUID) (authoring.Module, error) {
	var m authoring.Module
	err := db.QueryRow(ctx,
		`SELECT id, course_version_id, stable_id, position, title FROM authoring.modules WHERE id=$1`, id).
		Scan(&m.ID, &m.CourseVersionID, &m.StableID, &m.Position, &m.Title)
	if err != nil {
		return authoring.Module{}, traducir(err)
	}
	return m, nil
}

func (s *AuthoringStore) UnitByID(ctx context.Context, db dbx.DB, id uuid.UUID) (authoring.Unit, error) {
	var u authoring.Unit
	err := db.QueryRow(ctx,
		`SELECT id, module_id, course_version_id, stable_id, position, title FROM authoring.units WHERE id=$1`, id).
		Scan(&u.ID, &u.ModuleID, &u.CourseVersionID, &u.StableID, &u.Position, &u.Title)
	if err != nil {
		return authoring.Unit{}, traducir(err)
	}
	return u, nil
}

// NextPosition calcula la posición siguiente. El nombre de tabla y de columna
// vienen de constantes del propio código, nunca de la petición.
func (s *AuthoringStore) NextPosition(ctx context.Context, db dbx.DB, tabla, columnaPadre string, padre uuid.UUID) (int, error) {
	permitidas := map[string]map[string]bool{
		"authoring.modules":   {"course_version_id": true},
		"authoring.units":     {"module_id": true},
		"authoring.resources": {"unit_id": true},
	}
	if !permitidas[tabla][columnaPadre] {
		return 0, fmt.Errorf("combinación de tabla y columna no permitida: %s.%s", tabla, columnaPadre)
	}
	var pos int
	err := db.QueryRow(ctx,
		fmt.Sprintf(`SELECT coalesce(max(position),0)+1 FROM %s WHERE %s=$1`, tabla, columnaPadre), padre).Scan(&pos)
	if err != nil {
		return 0, traducir(err)
	}
	return pos, nil
}

func (s *AuthoringStore) InsertRevision(ctx context.Context, db dbx.DB, resourceID, authorID uuid.UUID, contenido string) error {
	if _, err := db.Exec(ctx, `
		INSERT INTO authoring.content_revisions (id, resource_id, content_md, author_id)
		VALUES ($1,$2,$3,$4)`, uuid.New(), resourceID, contenido, authorID); err != nil {
		return traducir(err)
	}
	// Se conservan las últimas 20 (RF-04).
	_, err := db.Exec(ctx, `
		DELETE FROM authoring.content_revisions
		 WHERE resource_id=$1 AND id NOT IN (
		       SELECT id FROM authoring.content_revisions
		        WHERE resource_id=$1 ORDER BY created_at DESC LIMIT 20)`, resourceID)
	return traducir(err)
}

// CloneVersion copia el árbol conservando los stable_id, que es lo que permite
// que el progreso del estudiante sobreviva a la versión nueva (ADR-0007). Los
// binarios no se duplican: los recursos apuntan al mismo asset_id.
func (s *AuthoringStore) CloneVersion(ctx context.Context, db dbx.DB, origen, destino uuid.UUID) error {
	if _, err := db.Exec(ctx, `
		INSERT INTO authoring.modules (id, course_version_id, stable_id, position, title)
		SELECT gen_random_uuid(), $2, stable_id, position, title
		  FROM authoring.modules WHERE course_version_id=$1`, origen, destino); err != nil {
		return traducir(err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO authoring.units (id, module_id, course_version_id, stable_id, position, title)
		SELECT gen_random_uuid(), nm.id, $2, u.stable_id, u.position, u.title
		  FROM authoring.units u
		  JOIN authoring.modules om ON om.id = u.module_id AND om.course_version_id = $1
		  JOIN authoring.modules nm ON nm.course_version_id = $2 AND nm.stable_id = om.stable_id
		 WHERE u.course_version_id = $1`, origen, destino); err != nil {
		return traducir(err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO authoring.resources
		    (id, unit_id, course_version_id, stable_id, position, title, type,
		     visible, required, downloadable, content_md, asset_refs, asset_id, external_url, updated_at)
		SELECT gen_random_uuid(), nu.id, $2, r.stable_id, r.position, r.title, r.type,
		       r.visible, r.required, r.downloadable, r.content_md, r.asset_refs, r.asset_id,
		       r.external_url, now()
		  FROM authoring.resources r
		  JOIN authoring.units ou ON ou.id = r.unit_id AND ou.course_version_id = $1
		  JOIN authoring.units nu ON nu.course_version_id = $2 AND nu.stable_id = ou.stable_id
		 WHERE r.course_version_id = $1`, origen, destino); err != nil {
		return traducir(err)
	}
	return nil
}
