package assessment

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"math/big"
	"time"

	"github.com/google/uuid"

	"mooc/backend/internal/platform/dbx"
	"mooc/backend/internal/platform/ids"
)

var (
	ErrNotFound    = errors.New("assessment: no encontrado")
	ErrProhibido   = errors.New("assessment: sin permiso")
	ErrEnCurso     = errors.New("assessment: ya hay un intento en curso")
	ErrSinIntentos = errors.New("assessment: se agotaron los intentos")
	ErrExpirado    = errors.New("assessment: el intento expiró")
	ErrYaEnviado   = errors.New("assessment: el intento ya fue enviado")
	ErrInmutable   = errors.New("assessment: la versión está publicada y es inmutable")
)

type Store interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, tx dbx.DB) error) error
	DB() dbx.DB

	UpsertQuiz(ctx context.Context, db dbx.DB, q Quiz) (Quiz, error)
	QuizByResource(ctx context.Context, db dbx.DB, resourceID uuid.UUID) (Quiz, error)
	// QuizByStableID busca por la identidad pedagógica dentro de la versión
	// vigente del curso: así el intento sobrevive a una versión nueva.
	QuizByStableID(ctx context.Context, db dbx.DB, courseID, stableID uuid.UUID) (Quiz, error)
	ResourceOwner(ctx context.Context, db dbx.DB, resourceID uuid.UUID) (uuid.UUID, uuid.UUID, string, error)

	InsertAttempt(ctx context.Context, db dbx.DB, a Attempt) error
	AttemptByID(ctx context.Context, db dbx.DB, id uuid.UUID) (Attempt, error)
	AttemptsOf(ctx context.Context, db dbx.DB, enrollmentID, quizStableID uuid.UUID) ([]Attempt, error)
	SaveAnswers(ctx context.Context, db dbx.DB, attemptID uuid.UUID, respuestas map[uuid.UUID][]uuid.UUID) error
	FinishAttempt(ctx context.Context, db dbx.DB, a Attempt) error

	// EnrollmentOf comprueba la inscripción activa del estudiante.
	EnrollmentOf(ctx context.Context, db dbx.DB, enrollmentID uuid.UUID) (uuid.UUID, uuid.UUID, string, error)
}

// Progreso es el puerto hacia el módulo que lleva el avance del estudiante.
//
// Aprobar un quiz completa su recurso, pero esa decisión es de `learning`, no
// de aquí: escribir directamente en su tabla saltaría sus reglas de completado
// y su recálculo del porcentaje. Los módulos se hablan por interfaz de
// servicio, no por la base (ADR-0001).
type Progreso interface {
	MarcarRecursoAprobado(ctx context.Context, enrollmentID, resourceStableID uuid.UUID) error
}

type Service struct {
	store    Store
	progreso Progreso
	log      *slog.Logger
}

func NewService(store Store, p Progreso, log *slog.Logger) *Service {
	return &Service{store: store, progreso: p, log: log}
}

type Actor struct {
	UserID uuid.UUID
	Role   string
	Trace  string
}

// ---------------------------------------------------------------- autoría --

// PutQuiz configura el quiz completo. Solo el profesor dueño del curso.
func (s *Service) PutQuiz(ctx context.Context, resourceID uuid.UUID, q Quiz, a Actor) (Quiz, error) {
	owner, versionID, estado, err := s.store.ResourceOwner(ctx, s.store.DB(), resourceID)
	if err != nil {
		return Quiz{}, err
	}
	if owner != a.UserID && a.Role != "admin" {
		return Quiz{}, ErrProhibido
	}
	if estado == "published" {
		return Quiz{}, ErrInmutable
	}

	q.ResourceID = resourceID
	q.CourseVersionID = versionID
	if q.ID == uuid.Nil {
		q.ID = ids.New()
	}
	if q.StableID == uuid.Nil {
		q.StableID = ids.New()
	}
	for i := range q.Questions {
		if q.Questions[i].StableID == uuid.Nil {
			q.Questions[i].StableID = ids.New()
		}
		q.Questions[i].ID = ids.New()
		q.Questions[i].Position = i + 1
		for j := range q.Questions[i].Options {
			if q.Questions[i].Options[j].StableID == uuid.Nil {
				q.Questions[i].Options[j].StableID = ids.New()
			}
			q.Questions[i].Options[j].ID = ids.New()
			q.Questions[i].Options[j].Position = j + 1
		}
	}
	return s.store.UpsertQuiz(ctx, s.store.DB(), q)
}

// GetQuizForAuthoring es la ÚNICA ruta por la que salen las claves.
func (s *Service) GetQuizForAuthoring(ctx context.Context, resourceID uuid.UUID, a Actor) (Quiz, error) {
	owner, _, _, err := s.store.ResourceOwner(ctx, s.store.DB(), resourceID)
	if err != nil {
		return Quiz{}, err
	}
	if owner != a.UserID && a.Role != "admin" {
		return Quiz{}, ErrProhibido
	}
	return s.store.QuizByResource(ctx, s.store.DB(), resourceID)
}

// -------------------------------------------------------------- intentos ---

func (s *Service) guardInscripcion(ctx context.Context, enrollmentID uuid.UUID, a Actor) (uuid.UUID, uuid.UUID, error) {
	userID, courseID, estado, err := s.store.EnrollmentOf(ctx, s.store.DB(), enrollmentID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if userID != a.UserID && a.Role != "admin" {
		return uuid.Nil, uuid.Nil, ErrProhibido
	}
	if estado != "active" {
		return uuid.Nil, uuid.Nil, ErrProhibido
	}
	return userID, courseID, nil
}

// Start crea el intento y congela el snapshot.
func (s *Service) Start(ctx context.Context, enrollmentID, quizStableID uuid.UUID, a Actor) (Attempt, error) {
	userID, courseID, err := s.guardInscripcion(ctx, enrollmentID, a)
	if err != nil {
		return Attempt{}, err
	}
	q, err := s.store.QuizByStableID(ctx, s.store.DB(), courseID, quizStableID)
	if err != nil {
		return Attempt{}, err
	}

	previos, err := s.store.AttemptsOf(ctx, s.store.DB(), enrollmentID, quizStableID)
	if err != nil {
		return Attempt{}, err
	}
	for _, p := range previos {
		if p.Status == StatusInProgress && !p.Expirado(time.Now().UTC()) {
			return Attempt{}, ErrEnCurso
		}
	}
	if r := IntentosRestantes(q.MaxAttempts, len(previos)); r == 0 {
		return Attempt{}, ErrSinIntentos
	}

	ahora := time.Now().UTC()
	att := Attempt{
		ID: ids.New(), QuizStableID: quizStableID, CourseID: courseID,
		UserID: userID, EnrollmentID: enrollmentID,
		AttemptNumber: len(previos) + 1,
		Snapshot:      Snapshot(q, barajar),
		Status:        StatusInProgress, StartedAt: ahora, GradingVersion: GradingVersion,
	}
	if q.TimeLimitSeconds != nil {
		exp := ahora.Add(time.Duration(*q.TimeLimitSeconds) * time.Second)
		att.ExpiresAt = &exp
	}

	if err := s.store.InsertAttempt(ctx, s.store.DB(), att); err != nil {
		return Attempt{}, err
	}
	return att, nil
}

func (s *Service) Get(ctx context.Context, attemptID uuid.UUID, a Actor) (Attempt, error) {
	att, err := s.store.AttemptByID(ctx, s.store.DB(), attemptID)
	if err != nil {
		return Attempt{}, err
	}
	if att.UserID != a.UserID && a.Role != "admin" {
		return Attempt{}, ErrProhibido
	}
	return att, nil
}

// SaveAnswers guarda parcialmente. Es naturalmente idempotente: sustituye la
// respuesta de cada pregunta.
func (s *Service) SaveAnswers(ctx context.Context, attemptID uuid.UUID,
	respuestas map[uuid.UUID][]uuid.UUID, a Actor) (Attempt, error) {

	att, err := s.Get(ctx, attemptID, a)
	if err != nil {
		return Attempt{}, err
	}
	if att.Status != StatusInProgress {
		return Attempt{}, ErrYaEnviado
	}
	// El reloj del servidor decide, no el del cliente.
	if att.Expirado(time.Now().UTC()) {
		return Attempt{}, ErrExpirado
	}
	if err := s.store.SaveAnswers(ctx, s.store.DB(), attemptID, respuestas); err != nil {
		return Attempt{}, err
	}
	return s.store.AttemptByID(ctx, s.store.DB(), attemptID)
}

// Submit califica en el servidor. Si llega tarde, se acepta pero se califica
// solo lo guardado a tiempo: se prefiere puntuar el trabajo hecho.
func (s *Service) Submit(ctx context.Context, attemptID uuid.UUID, a Actor) (Attempt, Quiz, error) {
	att, err := s.Get(ctx, attemptID, a)
	if err != nil {
		return Attempt{}, Quiz{}, err
	}
	q, err := s.store.QuizByStableID(ctx, s.store.DB(), att.CourseID, att.QuizStableID)
	if err != nil {
		return Attempt{}, Quiz{}, err
	}

	// Reenviar es idempotente: devuelve el mismo resultado sin recalcular.
	if att.Status != StatusInProgress {
		return att, q, nil
	}

	ahora := time.Now().UTC()
	nota, detalle := Calificar(q, att.Answers)
	att.Score = &nota
	att.ScoreDetail = detalle
	att.SubmittedAt = &ahora
	if att.Expirado(ahora) {
		att.Status = StatusExpiredSubmitted
	} else {
		att.Status = StatusSubmitted
	}

	if err := s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		return s.store.FinishAttempt(ctx, tx, att)
	}); err != nil {
		return Attempt{}, Quiz{}, err
	}

	// Aprobar el quiz completa su recurso. La decisión de qué significa
	// "completado" es de learning, así que se le pide a él.
	if nota >= q.PassScore {
		if err := s.progreso.MarcarRecursoAprobado(ctx, att.EnrollmentID, att.QuizStableID); err != nil {
			// La nota ya está guardada; no se pierde. El recálculo del
			// porcentaje ocurrirá con la evidencia siguiente.
			s.log.Error("no se pudo marcar el recurso del quiz como completado",
				"enrollment_id", att.EnrollmentID, "quiz", att.QuizStableID, "error", err)
		}
	}
	return att, q, nil
}

// barajar mezcla con Fisher-Yates usando crypto/rand.
//
// Con math/rand la mezcla es predecible a partir de la semilla. Aquí el riesgo
// concreto es bajo —las opciones se identifican por stable_id, no por
// posición, así que adivinar el orden no revela nada— pero un generador
// criptográfico no cuesta nada y evita tener que razonar sobre ello cada vez.
func barajar(qs []QuestionSnapshot) {
	for i := len(qs) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			// Sin entropía es preferible no barajar que barajar mal.
			return
		}
		j := int(n.Int64())
		qs[i], qs[j] = qs[j], qs[i]
	}
}

func (s *Service) History(ctx context.Context, enrollmentID, quizStableID uuid.UUID, a Actor) ([]Attempt, int, error) {
	if _, _, err := s.guardInscripcion(ctx, enrollmentID, a); err != nil {
		return nil, 0, err
	}
	intentos, err := s.store.AttemptsOf(ctx, s.store.DB(), enrollmentID, quizStableID)
	if err != nil {
		return nil, 0, err
	}
	return intentos, len(intentos), nil
}
