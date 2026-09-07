package assessment

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"mooc/backend/internal/platform/httpx"
	"mooc/backend/internal/platform/problem"
)

type API struct{ svc *Service }

func NewAPI(svc *Service) *API { return &API{svc: svc} }

func (a *API) Routes(mux *http.ServeMux, auth httpx.Middleware, soloDocentes httpx.Middleware) {
	mux.Handle("GET /api/v1/resources/{resourceId}/quiz", auth(soloDocentes(http.HandlerFunc(a.getAuthoring))))
	mux.Handle("PUT /api/v1/resources/{resourceId}/quiz", auth(soloDocentes(http.HandlerFunc(a.put))))

	mux.Handle("POST /api/v1/enrollments/{eid}/quizzes/{stableId}/attempts", auth(http.HandlerFunc(a.start)))
	mux.Handle("GET /api/v1/enrollments/{eid}/quizzes/{stableId}/attempts", auth(http.HandlerFunc(a.history)))
	mux.Handle("GET /api/v1/attempts/{id}", auth(http.HandlerFunc(a.get)))
	mux.Handle("PATCH /api/v1/attempts/{id}/answers", auth(http.HandlerFunc(a.save)))
	mux.Handle("POST /api/v1/attempts/{id}/submit", auth(http.HandlerFunc(a.submit)))
	mux.Handle("GET /api/v1/attempts/{id}/result", auth(http.HandlerFunc(a.result)))
}

func actor(r *http.Request) Actor {
	p, _ := httpx.PrincipalFrom(r.Context())
	return Actor{UserID: p.UserID, Role: p.Role, Trace: httpx.RequestIDFrom(r.Context())}
}

func pid(r *http.Request, n string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(n))
	if err != nil {
		return uuid.Nil, problem.Validation("El identificador de la ruta no es un UUID válido.")
	}
	return id, nil
}

// ---------------------------------------------------------------- autoría --

type optionBody struct {
	TextMD    string `json:"text_md"`
	IsCorrect bool   `json:"is_correct"`
}

type questionBody struct {
	StatementMD   string       `json:"statement_md"`
	Kind          string       `json:"kind"`
	Points        float64      `json:"points"`
	ExplanationMD *string      `json:"explanation_md"`
	Options       []optionBody `json:"options"`
}

type quizBody struct {
	MaxAttempts      int            `json:"max_attempts"`
	TimeLimitSeconds *int           `json:"time_limit_seconds"`
	PassScore        float64        `json:"pass_score"`
	FeedbackPolicy   string         `json:"feedback_policy"`
	Shuffle          bool           `json:"shuffle"`
	PartialCredit    bool           `json:"partial_credit"`
	Questions        []questionBody `json:"questions"`
}

func (a *API) put(w http.ResponseWriter, r *http.Request) {
	rid, err := pid(r, "resourceId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var body quizBody
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}

	if len(body.Questions) == 0 {
		httpx.Fail(w, r, problem.Validation("Un quiz necesita al menos una pregunta."))
		return
	}
	q := Quiz{
		MaxAttempts: body.MaxAttempts, TimeLimitSeconds: body.TimeLimitSeconds,
		PassScore: body.PassScore, FeedbackPolicy: body.FeedbackPolicy,
		Shuffle: body.Shuffle, PartialCredit: body.PartialCredit,
	}
	if q.FeedbackPolicy == "" {
		q.FeedbackPolicy = FeedbackCorrectness
	}
	if q.PassScore == 0 {
		q.PassScore = 70
	}
	for i, pb := range body.Questions {
		if len(pb.Options) < 2 {
			httpx.Fail(w, r, problem.Validation("Cada pregunta necesita al menos dos opciones."))
			return
		}
		correctas := 0
		opts := make([]Option, 0, len(pb.Options))
		for _, ob := range pb.Options {
			if ob.IsCorrect {
				correctas++
			}
			opts = append(opts, Option{TextMD: ob.TextMD, IsCorrect: ob.IsCorrect})
		}
		if correctas == 0 {
			p := problem.Validation("Toda pregunta necesita al menos una opción correcta.")
			p.Errors = append(p.Errors, problem.FieldError{
				Code: "quiz.invalid", Field: "questions[" + itoa(i) + "]",
				Detail: "Ninguna opción está marcada como correcta.",
			})
			httpx.Fail(w, r, p)
			return
		}
		kind := pb.Kind
		if kind == "" {
			kind = KindSingle
		}
		puntos := pb.Points
		if puntos == 0 {
			puntos = 1
		}
		q.Questions = append(q.Questions, Question{
			StatementMD: pb.StatementMD, Kind: kind, Points: puntos,
			ExplanationMD: pb.ExplanationMD, Options: opts,
		})
	}

	out, err := a.svc.PutQuiz(r.Context(), rid, q, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusOK, vistaAutoria(out))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// vistaAutoria SÍ incluye is_correct: es la ruta del profesor dueño.
func vistaAutoria(q Quiz) map[string]any {
	preguntas := make([]map[string]any, 0, len(q.Questions))
	for _, p := range q.Questions {
		opciones := make([]map[string]any, 0, len(p.Options))
		for _, o := range p.Options {
			opciones = append(opciones, map[string]any{
				"stable_id": o.StableID.String(), "position": o.Position,
				"text_md": o.TextMD, "is_correct": o.IsCorrect,
			})
		}
		preguntas = append(preguntas, map[string]any{
			"stable_id": p.StableID.String(), "position": p.Position,
			"statement_md": p.StatementMD, "kind": p.Kind, "points": p.Points,
			"explanation_md": p.ExplanationMD, "options": opciones,
		})
	}
	return map[string]any{
		"id": q.ID.String(), "stable_id": q.StableID.String(),
		"resource_id": q.ResourceID.String(), "max_attempts": q.MaxAttempts,
		"time_limit_seconds": q.TimeLimitSeconds, "pass_score": q.PassScore,
		"feedback_policy": q.FeedbackPolicy, "shuffle": q.Shuffle,
		"partial_credit": q.PartialCredit, "questions": preguntas,
	}
}

func (a *API) getAuthoring(w http.ResponseWriter, r *http.Request) {
	rid, err := pid(r, "resourceId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	q, err := a.svc.GetQuizForAuthoring(r.Context(), rid, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusOK, vistaAutoria(q))
}

// -------------------------------------------------------------- intentos ---

// vistaIntento es la del estudiante. El snapshot no tiene claves, y esta
// función tampoco las añade: no hay forma de que salgan por aquí.
func vistaIntento(a Attempt) map[string]any {
	respuestas := make([]map[string]any, 0, len(a.Answers))
	for pregunta, elegidas := range a.Answers {
		ids := make([]string, 0, len(elegidas))
		for _, id := range elegidas {
			ids = append(ids, id.String())
		}
		respuestas = append(respuestas, map[string]any{
			"question_stable_id": pregunta.String(), "selected_option_stable_ids": ids,
		})
	}
	return map[string]any{
		"id": a.ID.String(), "quiz_stable_id": a.QuizStableID.String(),
		"attempt_number": a.AttemptNumber, "status": a.Status,
		"started_at": a.StartedAt, "expires_at": a.ExpiresAt, "submitted_at": a.SubmittedAt,
		"questions": a.Snapshot, "answers": respuestas,
	}
}

func (a *API) start(w http.ResponseWriter, r *http.Request) {
	eid, err := pid(r, "eid")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	sid, err := pid(r, "stableId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	att, err := a.svc.Start(r.Context(), eid, sid, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, vistaIntento(att))
}

func (a *API) get(w http.ResponseWriter, r *http.Request) {
	id, err := pid(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	att, err := a.svc.Get(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusOK, vistaIntento(att))
}

func (a *API) save(w http.ResponseWriter, r *http.Request) {
	id, err := pid(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var body struct {
		Answers []struct {
			QuestionStableID string   `json:"question_stable_id"`
			Selected         []string `json:"selected_option_stable_ids"`
		} `json:"answers"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	respuestas := map[uuid.UUID][]uuid.UUID{}
	for _, ans := range body.Answers {
		q, err := uuid.Parse(ans.QuestionStableID)
		if err != nil {
			httpx.Fail(w, r, problem.Validation("`question_stable_id` no es un UUID válido."))
			return
		}
		var sel []uuid.UUID
		for _, s := range ans.Selected {
			o, err := uuid.Parse(s)
			if err != nil {
				httpx.Fail(w, r, problem.Validation("`selected_option_stable_ids` trae un UUID inválido."))
				return
			}
			sel = append(sel, o)
		}
		respuestas[q] = sel
	}

	att, err := a.svc.SaveAnswers(r.Context(), id, respuestas, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusOK, vistaIntento(att))
}

func (a *API) submit(w http.ResponseWriter, r *http.Request) {
	id, err := pid(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	att, q, err := a.svc.Submit(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusOK, vistaResultado(att, q, 0))
}

func (a *API) result(w http.ResponseWriter, r *http.Request) {
	id, err := pid(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	att, err := a.svc.Get(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	if att.Status == StatusInProgress {
		httpx.Fail(w, r, problem.Conflict("attempt.not_submitted", "El intento aún no se ha enviado."))
		return
	}
	_, q, err := a.svc.Submit(r.Context(), id, actor(r)) // idempotente: devuelve lo ya calificado
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusOK, vistaResultado(att, q, 0))
}

// vistaResultado aplica la política de retroalimentación. Es el único sitio
// donde las claves PUEDEN salir hacia el estudiante, y solo con `full` y sin
// intentos restantes.
func vistaResultado(a Attempt, q Quiz, intentosRestantes int) map[string]any {
	out := map[string]any{
		"attempt_id": a.ID.String(), "status": a.Status,
		"score": a.Score, "grading_version": a.GradingVersion,
		"feedback_policy": q.FeedbackPolicy,
	}
	if a.Score != nil {
		out["passed"] = *a.Score >= q.PassScore
	}
	filtrados := FiltrarRetroalimentacion(q.FeedbackPolicy, a.ScoreDetail, intentosRestantes)
	if filtrados != nil {
		out["questions"] = filtrados
	}
	return out
}

func (a *API) history(w http.ResponseWriter, r *http.Request) {
	eid, err := pid(r, "eid")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	sid, err := pid(r, "stableId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	intentos, _, err := a.svc.History(r.Context(), eid, sid, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	items := make([]map[string]any, 0, len(intentos))
	for _, i := range intentos {
		items = append(items, map[string]any{
			"id": i.ID.String(), "attempt_number": i.AttemptNumber,
			"status": i.Status, "score": i.Score, "submitted_at": i.SubmittedAt,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func traducir(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return problem.NotFound("El recurso solicitado no existe.")
	case errors.Is(err, ErrProhibido):
		return problem.Forbidden("No tienes acceso a este quiz.")
	case errors.Is(err, ErrInmutable):
		return problem.Conflict("version_immutable", "La versión está publicada y es inmutable.")
	case errors.Is(err, ErrEnCurso):
		return problem.Conflict("attempt.in_progress", "Ya tienes un intento en curso para este quiz.")
	case errors.Is(err, ErrSinIntentos):
		return problem.Conflict("attempt.no_attempts_left", "Se agotaron los intentos disponibles.")
	case errors.Is(err, ErrExpirado):
		return problem.Conflict("attempt.expired", "El intento expiró.")
	case errors.Is(err, ErrYaEnviado):
		return problem.Conflict("attempt.already_submitted", "El intento ya fue enviado.")
	default:
		return err
	}
}
