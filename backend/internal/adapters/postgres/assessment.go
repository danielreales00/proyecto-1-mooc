package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/modules/assessment"
	"mooc/backend/internal/platform/dbx"
)

type AssessmentStore struct{ pool *pgxpool.Pool }

func NewAssessmentStore(pool *pgxpool.Pool) *AssessmentStore { return &AssessmentStore{pool: pool} }

func (s *AssessmentStore) DB() dbx.DB { return s.pool }

func (s *AssessmentStore) WithinTx(ctx context.Context, fn func(context.Context, dbx.DB) error) error {
	return withinTx(ctx, s.pool, fn)
}

func nfa(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return assessment.ErrNotFound
	}
	return err
}

// ResourceOwner devuelve el dueño del curso, la versión y su estado, para que
// el servicio decida permiso e inmutabilidad sin conocer el esquema.
func (s *AssessmentStore) ResourceOwner(ctx context.Context, db dbx.DB, resourceID uuid.UUID) (uuid.UUID, uuid.UUID, string, error) {
	var owner, versionID uuid.UUID
	var estado string
	err := db.QueryRow(ctx, `
		SELECT c.owner_id, v.id, v.status
		  FROM authoring.resources r
		  JOIN authoring.course_versions v ON v.id = r.course_version_id
		  JOIN authoring.courses c ON c.id = v.course_id
		 WHERE r.id = $1`, resourceID).Scan(&owner, &versionID, &estado)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", nfa(err)
	}
	return owner, versionID, estado, nil
}

// UpsertQuiz reemplaza el quiz completo: es un PUT, no un PATCH.
func (s *AssessmentStore) UpsertQuiz(ctx context.Context, db dbx.DB, q assessment.Quiz) (assessment.Quiz, error) {
	err := withinTx(ctx, s.pool, func(ctx context.Context, tx dbx.DB) error {
		var existente uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM assessment.quizzes WHERE resource_id=$1`, q.ResourceID).Scan(&existente)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if _, err := tx.Exec(ctx, `
				INSERT INTO assessment.quizzes
				    (id, resource_id, course_version_id, stable_id, max_attempts,
				     time_limit_seconds, pass_score, feedback_policy, shuffle, partial_credit)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
				q.ID, q.ResourceID, q.CourseVersionID, q.StableID, q.MaxAttempts,
				q.TimeLimitSeconds, q.PassScore, q.FeedbackPolicy, q.Shuffle, q.PartialCredit); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			q.ID = existente
			if _, err := tx.Exec(ctx, `
				UPDATE assessment.quizzes
				   SET max_attempts=$2, time_limit_seconds=$3, pass_score=$4,
				       feedback_policy=$5, shuffle=$6, partial_credit=$7
				 WHERE id=$1`,
				q.ID, q.MaxAttempts, q.TimeLimitSeconds, q.PassScore,
				q.FeedbackPolicy, q.Shuffle, q.PartialCredit); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `DELETE FROM assessment.quiz_questions WHERE quiz_id=$1`, q.ID); err != nil {
				return err
			}
		}

		for _, p := range q.Questions {
			if _, err := tx.Exec(ctx, `
				INSERT INTO assessment.quiz_questions
				    (id, quiz_id, stable_id, position, statement_md, kind, points, explanation_md)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
				p.ID, q.ID, p.StableID, p.Position, p.StatementMD, p.Kind, p.Points, p.ExplanationMD); err != nil {
				return err
			}
			for _, o := range p.Options {
				if _, err := tx.Exec(ctx, `
					INSERT INTO assessment.quiz_options
					    (id, question_id, stable_id, position, text_md, is_correct)
					VALUES ($1,$2,$3,$4,$5,$6)`,
					o.ID, p.ID, o.StableID, o.Position, o.TextMD, o.IsCorrect); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return assessment.Quiz{}, err
	}
	return s.QuizByResource(ctx, db, q.ResourceID)
}

func (s *AssessmentStore) cargarQuiz(ctx context.Context, db dbx.DB, quizID uuid.UUID, q assessment.Quiz) (assessment.Quiz, error) {
	rows, err := db.Query(ctx, `
		SELECT p.id, p.stable_id, p.position, p.statement_md, p.kind, p.points, p.explanation_md,
		       o.id, o.stable_id, o.position, o.text_md, o.is_correct
		  FROM assessment.quiz_questions p
		  JOIN assessment.quiz_options o ON o.question_id = p.id
		 WHERE p.quiz_id = $1
		 ORDER BY p.position, o.position`, quizID)
	if err != nil {
		return assessment.Quiz{}, err
	}
	defer rows.Close()

	indice := map[uuid.UUID]int{}
	for rows.Next() {
		var p assessment.Question
		var o assessment.Option
		if err := rows.Scan(&p.ID, &p.StableID, &p.Position, &p.StatementMD, &p.Kind, &p.Points,
			&p.ExplanationMD, &o.ID, &o.StableID, &o.Position, &o.TextMD, &o.IsCorrect); err != nil {
			return assessment.Quiz{}, err
		}
		i, ok := indice[p.ID]
		if !ok {
			indice[p.ID] = len(q.Questions)
			q.Questions = append(q.Questions, p)
			i = len(q.Questions) - 1
		}
		q.Questions[i].Options = append(q.Questions[i].Options, o)
	}
	return q, rows.Err()
}

const colsQuiz = `id, resource_id, course_version_id, stable_id, max_attempts,
	time_limit_seconds, pass_score, feedback_policy, shuffle, partial_credit`

func scanQuiz(row pgx.Row) (assessment.Quiz, error) {
	var q assessment.Quiz
	err := row.Scan(&q.ID, &q.ResourceID, &q.CourseVersionID, &q.StableID, &q.MaxAttempts,
		&q.TimeLimitSeconds, &q.PassScore, &q.FeedbackPolicy, &q.Shuffle, &q.PartialCredit)
	if err != nil {
		return assessment.Quiz{}, nfa(err)
	}
	return q, nil
}

func (s *AssessmentStore) QuizByResource(ctx context.Context, db dbx.DB, resourceID uuid.UUID) (assessment.Quiz, error) {
	q, err := scanQuiz(db.QueryRow(ctx, `SELECT `+colsQuiz+` FROM assessment.quizzes WHERE resource_id=$1`, resourceID))
	if err != nil {
		return assessment.Quiz{}, err
	}
	return s.cargarQuiz(ctx, db, q.ID, q)
}

// QuizByStableID resuelve por identidad pedagógica dentro de la versión
// vigente del curso: el intento sobrevive a una versión nueva (ADR-0007).
func (s *AssessmentStore) QuizByStableID(ctx context.Context, db dbx.DB, courseID, stableID uuid.UUID) (assessment.Quiz, error) {
	q, err := scanQuiz(db.QueryRow(ctx, `
		SELECT z.id, z.resource_id, z.course_version_id, z.stable_id, z.max_attempts,
		       z.time_limit_seconds, z.pass_score, z.feedback_policy, z.shuffle, z.partial_credit
		  FROM assessment.quizzes z
		  JOIN authoring.resources r ON r.id = z.resource_id
		  JOIN authoring.courses c ON c.current_version_id = r.course_version_id
		 WHERE c.id = $1 AND r.stable_id = $2`, courseID, stableID))
	if err != nil {
		return assessment.Quiz{}, err
	}
	return s.cargarQuiz(ctx, db, q.ID, q)
}

func (s *AssessmentStore) InsertAttempt(ctx context.Context, db dbx.DB, a assessment.Attempt) error {
	snap, err := json.Marshal(a.Snapshot)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `
		INSERT INTO assessment.quiz_attempts
		    (id, quiz_stable_id, course_id, user_id, enrollment_id, attempt_number,
		     snapshot, status, started_at, expires_at, grading_version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		a.ID, a.QuizStableID, a.CourseID, a.UserID, a.EnrollmentID, a.AttemptNumber,
		snap, a.Status, a.StartedAt, a.ExpiresAt, a.GradingVersion)
	return err
}

func (s *AssessmentStore) AttemptByID(ctx context.Context, db dbx.DB, id uuid.UUID) (assessment.Attempt, error) {
	var a assessment.Attempt
	var snap, detalle []byte
	err := db.QueryRow(ctx, `
		SELECT id, quiz_stable_id, course_id, user_id, enrollment_id, attempt_number,
		       snapshot, status, started_at, expires_at, submitted_at, score, score_detail, grading_version
		  FROM assessment.quiz_attempts WHERE id=$1`, id).
		Scan(&a.ID, &a.QuizStableID, &a.CourseID, &a.UserID, &a.EnrollmentID, &a.AttemptNumber,
			&snap, &a.Status, &a.StartedAt, &a.ExpiresAt, &a.SubmittedAt, &a.Score, &detalle, &a.GradingVersion)
	if err != nil {
		return assessment.Attempt{}, nfa(err)
	}
	_ = json.Unmarshal(snap, &a.Snapshot)
	if len(detalle) > 0 {
		_ = json.Unmarshal(detalle, &a.ScoreDetail)
	}

	rows, err := db.Query(ctx,
		`SELECT question_stable_id, selected_option_stable_ids FROM assessment.attempt_answers WHERE attempt_id=$1`, id)
	if err != nil {
		return assessment.Attempt{}, err
	}
	defer rows.Close()
	a.Answers = map[uuid.UUID][]uuid.UUID{}
	for rows.Next() {
		var q uuid.UUID
		var sel []uuid.UUID
		if err := rows.Scan(&q, &sel); err != nil {
			return assessment.Attempt{}, err
		}
		a.Answers[q] = sel
	}
	return a, rows.Err()
}

func (s *AssessmentStore) AttemptsOf(ctx context.Context, db dbx.DB, enrollmentID, quizStableID uuid.UUID) ([]assessment.Attempt, error) {
	rows, err := db.Query(ctx, `
		SELECT id, attempt_number, status, started_at, expires_at, submitted_at, score
		  FROM assessment.quiz_attempts
		 WHERE enrollment_id=$1 AND quiz_stable_id=$2 ORDER BY attempt_number`, enrollmentID, quizStableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []assessment.Attempt
	for rows.Next() {
		var a assessment.Attempt
		if err := rows.Scan(&a.ID, &a.AttemptNumber, &a.Status, &a.StartedAt,
			&a.ExpiresAt, &a.SubmittedAt, &a.Score); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *AssessmentStore) SaveAnswers(ctx context.Context, db dbx.DB, attemptID uuid.UUID, respuestas map[uuid.UUID][]uuid.UUID) error {
	for pregunta, elegidas := range respuestas {
		if elegidas == nil {
			elegidas = []uuid.UUID{}
		}
		if _, err := db.Exec(ctx, `
			INSERT INTO assessment.attempt_answers (attempt_id, question_stable_id, selected_option_stable_ids)
			VALUES ($1,$2,$3)
			ON CONFLICT (attempt_id, question_stable_id)
			DO UPDATE SET selected_option_stable_ids = $3, saved_at = now()`,
			attemptID, pregunta, elegidas); err != nil {
			return err
		}
	}
	return nil
}

func (s *AssessmentStore) FinishAttempt(ctx context.Context, db dbx.DB, a assessment.Attempt) error {
	detalle, err := json.Marshal(a.ScoreDetail)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `
		UPDATE assessment.quiz_attempts
		   SET status=$2, submitted_at=$3, score=$4, score_detail=$5
		 WHERE id=$1 AND status='in_progress'`,
		a.ID, a.Status, a.SubmittedAt, a.Score, detalle)
	return err
}

func (s *AssessmentStore) EnrollmentOf(ctx context.Context, db dbx.DB, enrollmentID uuid.UUID) (uuid.UUID, uuid.UUID, string, error) {
	var userID, courseID uuid.UUID
	var estado string
	err := db.QueryRow(ctx,
		`SELECT user_id, course_id, status FROM progress.enrollments WHERE id=$1`, enrollmentID).
		Scan(&userID, &courseID, &estado)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", nfa(err)
	}
	return userID, courseID, estado, nil
}
