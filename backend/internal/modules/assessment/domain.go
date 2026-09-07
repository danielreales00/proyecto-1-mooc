// Package assessment cubre quizzes e intentos (RF-08).
//
// El invariante que gobierna este módulo: la clave correcta nunca sale del
// servidor por las rutas del estudiante (CA-04, ADR-0013). No se resuelve con
// `omitempty`: hay dos tipos distintos, y el del estudiante no tiene el campo.
package assessment

import (
	"math"
	"time"

	"github.com/google/uuid"
)

const (
	StatusInProgress       = "in_progress"
	StatusSubmitted        = "submitted"
	StatusExpired          = "expired"
	StatusExpiredSubmitted = "expired_submitted"
)

const (
	FeedbackNone        = "none"
	FeedbackScoreOnly   = "score_only"
	FeedbackCorrectness = "correctness"
	FeedbackFull        = "full"
)

const (
	KindSingle   = "single"
	KindMultiple = "multiple"
)

// GradingVersion permite recalificar de forma auditable si se corrige un error.
const GradingVersion = 1

type Quiz struct {
	ID               uuid.UUID
	ResourceID       uuid.UUID
	CourseVersionID  uuid.UUID
	StableID         uuid.UUID
	MaxAttempts      int
	TimeLimitSeconds *int
	PassScore        float64
	FeedbackPolicy   string
	Shuffle          bool
	PartialCredit    bool
	Questions        []Question
}

// Question es la pregunta con sus claves. SOLO circula del lado del servidor y
// por la ruta de autoría.
type Question struct {
	ID            uuid.UUID
	StableID      uuid.UUID
	Position      int
	StatementMD   string
	Kind          string
	Points        float64
	ExplanationMD *string
	Options       []Option
}

// Option lleva is_correct. Ver OptionSnapshot para lo que ve el estudiante.
type Option struct {
	ID        uuid.UUID
	StableID  uuid.UUID
	Position  int
	TextMD    string
	IsCorrect bool
}

// Correctas devuelve los stable_id de las opciones correctas.
func (q Question) Correctas() []uuid.UUID {
	var out []uuid.UUID
	for _, o := range q.Options {
		if o.IsCorrect {
			out = append(out, o.StableID)
		}
	}
	return out
}

// --------------------------------------------------------------- snapshot --

// QuestionSnapshot es lo que se congela al iniciar el intento y lo que se
// devuelve al estudiante. NO tiene is_correct: el campo no existe en el tipo.
type QuestionSnapshot struct {
	StableID    uuid.UUID        `json:"stable_id"`
	Position    int              `json:"position"`
	StatementMD string           `json:"statement_md"`
	Kind        string           `json:"kind"`
	Points      float64          `json:"points"`
	Options     []OptionSnapshot `json:"options"`
}

type OptionSnapshot struct {
	StableID uuid.UUID `json:"stable_id"`
	TextMD   string    `json:"text_md"`
}

// Snapshot congela el quiz para un intento. Si el profesor edita el quiz
// después, el estudiante sigue viendo el mismo examen y la nota es
// reproducible (ADR-0013).
func Snapshot(q Quiz, barajar func([]QuestionSnapshot)) []QuestionSnapshot {
	out := make([]QuestionSnapshot, 0, len(q.Questions))
	for _, p := range q.Questions {
		opts := make([]OptionSnapshot, 0, len(p.Options))
		for _, o := range p.Options {
			opts = append(opts, OptionSnapshot{StableID: o.StableID, TextMD: o.TextMD})
		}
		out = append(out, QuestionSnapshot{
			StableID: p.StableID, Position: p.Position, StatementMD: p.StatementMD,
			Kind: p.Kind, Points: p.Points, Options: opts,
		})
	}
	if q.Shuffle && barajar != nil {
		barajar(out)
	}
	return out
}

type Attempt struct {
	ID             uuid.UUID
	QuizStableID   uuid.UUID
	CourseID       uuid.UUID
	UserID         uuid.UUID
	EnrollmentID   uuid.UUID
	AttemptNumber  int
	Snapshot       []QuestionSnapshot
	Status         string
	StartedAt      time.Time
	ExpiresAt      *time.Time
	SubmittedAt    *time.Time
	Score          *float64
	ScoreDetail    []QuestionResult
	GradingVersion int
	Answers        map[uuid.UUID][]uuid.UUID
}

// Expirado usa el reloj del SERVIDOR, nunca el del cliente.
func (a Attempt) Expirado(ahora time.Time) bool {
	return a.ExpiresAt != nil && ahora.After(*a.ExpiresAt)
}

type QuestionResult struct {
	QuestionStableID uuid.UUID   `json:"question_stable_id"`
	AwardedPoints    float64     `json:"awarded_points"`
	MaxPoints        float64     `json:"max_points"`
	Correct          bool        `json:"correct"`
	CorrectOptions   []uuid.UUID `json:"correct_option_stable_ids,omitempty"`
	ExplanationMD    *string     `json:"explanation_md,omitempty"`
}

// ------------------------------------------------------------ calificación --

// Calificar calcula la nota en el servidor contra las claves de la base, no
// contra el snapshot (que no las tiene). Es reproducible: mismas respuestas,
// misma nota (CA-04).
func Calificar(q Quiz, respuestas map[uuid.UUID][]uuid.UUID) (float64, []QuestionResult) {
	var obtenidos, posibles float64
	resultados := make([]QuestionResult, 0, len(q.Questions))

	for _, pregunta := range q.Questions {
		correctas := pregunta.Correctas()
		elegidas := respuestas[pregunta.StableID]
		posibles += pregunta.Points

		conjuntoCorrectas := map[uuid.UUID]bool{}
		for _, id := range correctas {
			conjuntoCorrectas[id] = true
		}
		aciertos, errores := 0, 0
		vistas := map[uuid.UUID]bool{}
		for _, id := range elegidas {
			if vistas[id] {
				continue // una opción marcada dos veces cuenta una
			}
			vistas[id] = true
			if conjuntoCorrectas[id] {
				aciertos++
			} else {
				errores++
			}
		}

		var puntos float64
		exacto := aciertos == len(correctas) && errores == 0 && len(correctas) > 0
		switch {
		case pregunta.Kind == KindSingle || !q.PartialCredit:
			// Todo o nada.
			if exacto {
				puntos = pregunta.Points
			}
		default:
			// Puntuación parcial: aciertos menos errores, sin bajar de cero.
			if len(correctas) > 0 {
				bruto := float64(aciertos-errores) / float64(len(correctas))
				puntos = math.Max(0, bruto) * pregunta.Points
			}
		}

		obtenidos += puntos
		resultados = append(resultados, QuestionResult{
			QuestionStableID: pregunta.StableID,
			AwardedPoints:    math.Round(puntos*100) / 100,
			MaxPoints:        pregunta.Points,
			Correct:          exacto,
			CorrectOptions:   correctas,
			ExplanationMD:    pregunta.ExplanationMD,
		})
	}

	if posibles == 0 {
		return 0, resultados
	}
	nota := obtenidos / posibles * 100
	return math.Round(nota*100) / 100, resultados
}

// FiltrarRetroalimentacion decide qué se le devuelve al estudiante según la
// política. Solo `full` revela las claves, y solo si ya no quedan intentos
// (ADR-0013).
func FiltrarRetroalimentacion(politica string, resultados []QuestionResult,
	intentosRestantes int) []QuestionResult {

	switch politica {
	case FeedbackNone, FeedbackScoreOnly:
		return nil
	case FeedbackFull:
		if intentosRestantes > 0 {
			// Degrada a correctness: revelar la clave con intentos por delante
			// convierte el examen en un ejercicio de memoria.
			politica = FeedbackCorrectness
		}
	}

	out := make([]QuestionResult, 0, len(resultados))
	for _, r := range resultados {
		if politica == FeedbackCorrectness {
			r.CorrectOptions = nil
			r.ExplanationMD = nil
		}
		out = append(out, r)
	}
	return out
}

// IntentosRestantes devuelve cuántos quedan; -1 significa ilimitados.
func IntentosRestantes(max, usados int) int {
	if max <= 0 {
		return -1
	}
	r := max - usados
	if r < 0 {
		return 0
	}
	return r
}
