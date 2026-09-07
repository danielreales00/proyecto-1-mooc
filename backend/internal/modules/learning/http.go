package learning

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"mooc/backend/internal/platform/cursor"
	"mooc/backend/internal/platform/httpx"
	"mooc/backend/internal/platform/markdown"
	"mooc/backend/internal/platform/metrics"
	"mooc/backend/internal/platform/problem"
	"mooc/backend/internal/platform/ratelimit"
)

type API struct {
	svc   *Service
	audit auditor
}

// auditor permite registrar el intento de manipulación sin acoplar la capa
// HTTP al módulo de auditoría completo.
type auditor interface {
	TamperingAttempt(r *http.Request, enrollmentID uuid.UUID, cuerpo map[string]any)
}

func NewAPI(svc *Service, a auditor) *API { return &API{svc: svc, audit: a} }

// LimiteProgreso acota la ingesta de evidencias. El servidor ya rechaza los
// heartbeats demasiado seguidos por cadencia (ADR-0012); esto es la segunda
// barrera, contra quien inunde el endpoint sin esperar respuesta.
var LimiteProgreso = ratelimit.Regla{Nombre: "progress", Limite: 120, Ventana: time.Minute}

// idem aplica la idempotencia por cabecera a las operaciones no repetibles.
// Si es nil, se registran sin ella (útil en pruebas).
func (a *API) Routes(mux *http.ServeMux, auth httpx.Middleware,
	limitar func(ratelimit.Regla) httpx.Middleware, idem httpx.Middleware) {

	conIdem := func(h http.HandlerFunc) http.Handler {
		if idem == nil {
			return h
		}
		return idem(h)
	}
	// Catálogo: público.
	mux.HandleFunc("GET /api/v1/catalog/courses", a.search)
	mux.HandleFunc("GET /api/v1/catalog/courses/{slug}", a.bySlug)

	mux.Handle("POST /api/v1/enrollments", auth(conIdem(a.enroll)))
	mux.Handle("GET /api/v1/enrollments", auth(http.HandlerFunc(a.mine)))
	mux.Handle("GET /api/v1/enrollments/{id}", auth(http.HandlerFunc(a.get)))
	mux.Handle("POST /api/v1/enrollments/{id}/withdraw", auth(http.HandlerFunc(a.withdraw)))
	mux.Handle("GET /api/v1/enrollments/{id}/content", auth(http.HandlerFunc(a.content)))
	reportar := http.Handler(http.HandlerFunc(a.report))
	if limitar != nil {
		reportar = limitar(LimiteProgreso)(reportar)
	}
	mux.Handle("POST /api/v1/enrollments/{id}/progress", auth(reportar))
	mux.Handle("GET /api/v1/enrollments/{id}/progress", auth(http.HandlerFunc(a.summary)))
}

func actor(r *http.Request) Actor {
	p, _ := httpx.PrincipalFrom(r.Context())
	return Actor{UserID: p.UserID, Role: p.Role, IP: httpx.ClientIP(r),
		Agent: r.UserAgent(), Trace: httpx.RequestIDFrom(r.Context())}
}

func pathID(r *http.Request, nombre string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(nombre))
	if err != nil {
		return uuid.Nil, problem.Validation("El identificador de la ruta no es un UUID válido.")
	}
	return id, nil
}

// --------------------------------------------------------------- catálogo --

func (a *API) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limite := cursor.Limite(entero(q.Get("limit")), 20, 100)

	var desde *Cursor
	if c, ok := cursor.Decodificar(q.Get("cursor")); ok {
		desde = &Cursor{Fecha: c.Fecha, ID: c.ID}
	}

	cursos, err := a.svc.Search(r.Context(), q.Get("q"), q.Get("category"), q.Get("language"), limite, desde)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}

	cursos, siguiente := cursor.Pagina(cursos, limite,
		func(c CatalogCourse) (time.Time, string) { return c.CreatedAt, c.ID.String() })

	items := make([]map[string]any, 0, len(cursos))
	for _, c := range cursos {
		items = append(items, map[string]any{
			"id": c.ID.String(), "slug": c.Slug, "title": c.Title, "summary": c.Summary,
			"category": c.Category, "language": c.Language,
			"version_number": c.VersionNumber, "enrolled_count": c.EnrolledCount,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": siguiente})
}

func entero(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func (a *API) bySlug(w http.ResponseWriter, r *http.Request) {
	c, err := a.svc.CourseBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	esquema := make([]map[string]any, 0, len(c.Outline))
	for _, m := range c.Outline {
		unidades := make([]map[string]any, 0, len(m.Units))
		for _, u := range m.Units {
			unidades = append(unidades, map[string]any{"title": u})
		}
		esquema = append(esquema, map[string]any{"title": m.Title, "units": unidades})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id": c.ID.String(), "slug": c.Slug, "title": c.Title, "summary": c.Summary,
		"category": c.Category, "language": c.Language,
		"version_number": c.VersionNumber, "enrolled_count": c.EnrolledCount,
		"outline": esquema,
	})
}

// ------------------------------------------------------------ inscripción --

func enrollmentView(e Enrollment) map[string]any {
	v := map[string]any{
		"id": e.ID.String(), "course_id": e.CourseID.String(),
		"status": e.Status, "state": e.State, "progress_pct": e.ProgressPct,
		"enrolled_at": e.EnrolledAt, "completed_at": e.CompletedAt, "approved_at": e.ApprovedAt,
	}
	if e.ApprovedVersionID != nil {
		v["approved_version_id"] = e.ApprovedVersionID.String()
	} else {
		v["approved_version_id"] = nil
	}
	return v
}

func (a *API) enroll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CourseID string `json:"course_id"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	id, err := uuid.Parse(body.CourseID)
	if err != nil {
		httpx.Fail(w, r, problem.Validation("`course_id` no es un UUID válido."))
		return
	}
	e, err := a.svc.Enroll(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, enrollmentView(e))
}

func (a *API) mine(w http.ResponseWriter, r *http.Request) {
	es, err := a.svc.MyEnrollments(r.Context(), actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	items := make([]map[string]any, 0, len(es))
	for _, e := range es {
		items = append(items, enrollmentView(e))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nil})
}

func (a *API) get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	e, err := a.svc.GetEnrollment(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusOK, enrollmentView(e))
}

func (a *API) withdraw(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	e, err := a.svc.Withdraw(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusOK, enrollmentView(e))
}

func (a *API) content(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	e, recursos, err := a.svc.Content(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	items := make([]map[string]any, 0, len(recursos))
	for _, c := range recursos {
		item := map[string]any{
			"stable_id": c.StableID.String(), "title": c.Title, "type": c.Type,
			"required": c.Required,
			"progress": map[string]any{
				"state":                 c.Progress.State,
				"dwell_ms_total":        c.Progress.DwellMsTotal,
				"last_position_seconds": c.Progress.LastPositionSeconds,
				"completed_at":          c.Progress.CompletedAt,
			},
		}
		// El contenido textual se entrega ya renderizado y saneado.
		if c.ContentMD != nil {
			if html, err := markdown.RenderHTML(*c.ContentMD); err == nil {
				item["content_html"] = html
			}
		}
		items = append(items, item)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"enrollment": enrollmentView(e), "resources": items,
	})
}

// --------------------------------------------------------------- progreso --

// camposCalculados son los que el cliente NO puede enviar: el progreso lo
// calcula el servidor a partir de evidencias (CA-05, ADR-0012).
var camposCalculados = []string{"progress_percent", "progress_pct", "completed", "state", "score"}

func (a *API) report(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}

	// Se lee el cuerpo entero para poder detectar y AUDITAR el intento de
	// enviar un valor calculado, en vez de limitarse a rechazarlo.
	crudo, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64*1024))
	if err != nil {
		httpx.Fail(w, r, problem.Validation("No se pudo leer el cuerpo de la solicitud."))
		return
	}
	var libre map[string]any
	if err := json.Unmarshal(crudo, &libre); err != nil {
		httpx.Fail(w, r, problem.Validation("El cuerpo no es JSON válido."))
		return
	}

	for _, campo := range camposCalculados {
		if _, presente := libre[campo]; presente {
			metrics.ProgressRejected.WithLabelValues("client_computed_value").Inc()
			if a.audit != nil {
				a.audit.TamperingAttempt(r, id, libre)
			}
			p := problem.New(http.StatusUnprocessableEntity, "progress.client_computed_value",
				"El cliente no calcula el progreso",
				"El progreso lo calcula el servidor a partir de evidencias. El intento quedó auditado.")
			p.Errors = append(p.Errors, problem.FieldError{
				Code: "progress.client_computed_value", Field: campo,
				Detail: "Este campo no se acepta: reporta evidencias (open, heartbeat, close), no conclusiones.",
			})
			problem.Write(w, r, p)
			return
		}
	}

	var body struct {
		ResourceStableID string   `json:"resource_stable_id"`
		Kind             string   `json:"kind"`
		PositionSeconds  *float64 `json:"position_seconds"`
		ClientTS         *string  `json:"client_ts"`
	}
	dec := json.NewDecoder(newReader(crudo))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		httpx.Fail(w, r, problem.Validation(err.Error()))
		return
	}

	stableID, err := uuid.Parse(body.ResourceStableID)
	if err != nil {
		httpx.Fail(w, r, problem.Validation("`resource_stable_id` no es un UUID válido."))
		return
	}
	if body.Kind != EvidenceOpen && body.Kind != EvidenceHeartbeat && body.Kind != EvidenceClose {
		httpx.Fail(w, r, problem.Validation("`kind` debe ser open, heartbeat o close."))
		return
	}

	ev := Evidence{ResourceStableID: stableID, Kind: body.Kind, PositionSeconds: body.PositionSeconds}
	if body.ClientTS != nil {
		if t, err := time.Parse(time.RFC3339, *body.ClientTS); err == nil {
			ev.ClientTS = &t
		}
	}

	res, err := a.svc.ReportEvidence(r.Context(), id, ev, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	cuerpo := map[string]any{
		"accepted": res.Aceptada, "resource_state": res.EstadoRecurso,
		"progress_pct": res.ProgresoPct, "state": res.Estado,
	}
	if res.Motivo != "" {
		cuerpo["reject_reason"] = res.Motivo
	} else {
		cuerpo["reject_reason"] = nil
	}
	httpx.JSON(w, http.StatusAccepted, cuerpo)
}

func (a *API) summary(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	e, recursos, obligatorios, completados, err := a.svc.Summary(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	items := make([]map[string]any, 0, len(recursos))
	for _, c := range recursos {
		items = append(items, map[string]any{
			"resource_stable_id": c.StableID.String(), "title": c.Title,
			"required": c.Required, "state": c.Progress.State,
			"dwell_ms_total":        c.Progress.DwellMsTotal,
			"last_position_seconds": c.Progress.LastPositionSeconds,
			"completed_at":          c.Progress.CompletedAt,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"enrollment_id": e.ID.String(), "state": e.State,
		"progress_pct":   e.ProgressPct,
		"required_total": obligatorios, "required_completed": completados,
		"resources": items,
	})
}

func traducir(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return problem.NotFound("El recurso solicitado no existe.")
	case errors.Is(err, ErrProhibido):
		return problem.Forbidden("Esta inscripción no es tuya.")
	case errors.Is(err, ErrNoPublicado):
		return problem.Conflict("course_not_published", "El curso no tiene una versión publicada.")
	default:
		return err
	}
}
