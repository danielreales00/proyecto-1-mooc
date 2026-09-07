package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/modules/badges"
	"mooc/backend/internal/platform/dbx"
)

type BadgeStore struct{ pool *pgxpool.Pool }

func NewBadgeStore(pool *pgxpool.Pool) *BadgeStore { return &BadgeStore{pool: pool} }

func (s *BadgeStore) DB() dbx.DB { return s.pool }

// IssueForEnrollment usa ON CONFLICT DO NOTHING sobre la restricción única de
// enrollment_id: una segunda emisión no crea una segunda insignia (CA-07).
func (s *BadgeStore) IssueForEnrollment(ctx context.Context, db dbx.DB, b badges.Badge) (badges.Badge, bool, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `
		INSERT INTO badges.badges
		    (id, enrollment_id, user_id, course_id, course_version_id, public_code, image_key, issued_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (enrollment_id) DO NOTHING
		RETURNING id`,
		b.ID, b.EnrollmentID, b.UserID, b.CourseID, b.CourseVersionID,
		b.PublicCode, b.ImageKey, b.IssuedAt).Scan(&id)

	if errors.Is(err, pgx.ErrNoRows) {
		existente, err2 := s.byEnrollment(ctx, db, b.EnrollmentID)
		return existente, false, err2
	}
	if err != nil {
		return badges.Badge{}, false, err
	}
	b.ID = id
	return b, true, nil
}

const colsInsignia = `b.id, b.enrollment_id, b.user_id, b.course_id, b.course_version_id,
	b.public_code, b.image_key, b.issued_at, b.revoked_at, b.revocation_reason,
	u.full_name, c.title, v.version_number`

func scanInsignia(row pgx.Row) (badges.Badge, error) {
	var b badges.Badge
	err := row.Scan(&b.ID, &b.EnrollmentID, &b.UserID, &b.CourseID, &b.CourseVersionID,
		&b.PublicCode, &b.ImageKey, &b.IssuedAt, &b.RevokedAt, &b.RevocationReason,
		&b.StudentName, &b.CourseTitle, &b.VersionNumber)
	if errors.Is(err, pgx.ErrNoRows) {
		return badges.Badge{}, badges.ErrNotFound
	}
	return b, err
}

const desdeInsignia = `
	  FROM badges.badges b
	  JOIN identity.users u ON u.id = b.user_id
	  JOIN authoring.courses c ON c.id = b.course_id
	  JOIN authoring.course_versions v ON v.id = b.course_version_id`

func (s *BadgeStore) byEnrollment(ctx context.Context, db dbx.DB, enrollmentID uuid.UUID) (badges.Badge, error) {
	return scanInsignia(db.QueryRow(ctx, `SELECT `+colsInsignia+desdeInsignia+` WHERE b.enrollment_id=$1`, enrollmentID))
}

func (s *BadgeStore) ByPublicCode(ctx context.Context, db dbx.DB, codigo string) (badges.Badge, error) {
	return scanInsignia(db.QueryRow(ctx, `SELECT `+colsInsignia+desdeInsignia+` WHERE b.public_code=$1`, codigo))
}

func (s *BadgeStore) ByID(ctx context.Context, db dbx.DB, id uuid.UUID) (badges.Badge, error) {
	return scanInsignia(db.QueryRow(ctx, `SELECT `+colsInsignia+desdeInsignia+` WHERE b.id=$1`, id))
}

func (s *BadgeStore) ByUser(ctx context.Context, db dbx.DB, userID uuid.UUID) ([]badges.Badge, error) {
	rows, err := db.Query(ctx, `SELECT `+colsInsignia+desdeInsignia+` WHERE b.user_id=$1 ORDER BY b.issued_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []badges.Badge
	for rows.Next() {
		b, err := scanInsignia(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *BadgeStore) Revoke(ctx context.Context, db dbx.DB, id, by uuid.UUID, motivo string) (badges.Badge, error) {
	if _, err := db.Exec(ctx, `
		UPDATE badges.badges
		   SET revoked_at = coalesce(revoked_at, now()), revoked_by=$2, revocation_reason=$3
		 WHERE id=$1`, id, by, motivo); err != nil {
		return badges.Badge{}, err
	}
	return s.ByID(ctx, db, id)
}

func (s *BadgeStore) EnrollmentSnapshot(ctx context.Context, db dbx.DB, enrollmentID uuid.UUID) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	var userID, courseID, versionID uuid.UUID
	err := db.QueryRow(ctx, `
		SELECT e.user_id, e.course_id, coalesce(e.approved_version_id, c.current_version_id)
		  FROM progress.enrollments e
		  JOIN authoring.courses c ON c.id = e.course_id
		 WHERE e.id=$1`, enrollmentID).Scan(&userID, &courseID, &versionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, uuid.Nil, badges.ErrNotFound
	}
	return userID, courseID, versionID, err
}
