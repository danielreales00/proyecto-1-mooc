package authoring

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"mooc/backend/internal/platform/httpx"
	"mooc/backend/internal/platform/markdown"
	"mooc/backend/internal/platform/problem"
)

type API struct{ svc *Service }

func NewAPI(svc *Service) *API { return &API{svc: svc} }

// Routes registra la autoría. Todo exige sesión de profesor o administrador.
func (a *API) Routes(mux *http.ServeMux, auth httpx.Middleware,
	soloDocentes httpx.Middleware, idem httpx.Middleware) {

	p := func(h http.HandlerFunc) http.Handler { return auth(soloDocentes(h)) }
	// Operaciones no repetibles: crear, versionar y publicar.
	pi := func(h http.HandlerFunc) http.Handler {
		if idem == nil {
			return p(h)
		}
		return auth(soloDocentes(idem(h)))
	}

	mux.Handle("GET /api/v1/courses", p(a.listCourses))
	mux.Handle("POST /api/v1/courses", pi(a.createCourse))
	mux.Handle("GET /api/v1/courses/{courseId}", p(a.getCourse))
	mux.Handle("PATCH /api/v1/courses/{courseId}", p(a.updateCourse))
	mux.Handle("GET /api/v1/courses/{courseId}/versions", p(a.listVersions))
	mux.Handle("POST /api/v1/courses/{courseId}/versions", pi(a.newVersion))
	mux.Handle("GET /api/v1/courses/{courseId}/versions/{n}", p(a.getVersion))
	mux.Handle("GET /api/v1/courses/{courseId}/versions/{n}/preview", p(a.preview))
	mux.Handle("POST /api/v1/courses/{courseId}/versions/{n}/validate", p(a.validate))
	mux.Handle("POST /api/v1/courses/{courseId}/versions/{n}/publish", pi(a.publish))
	mux.Handle("POST /api/v1/courses/{courseId}/versions/{n}/unpublish", p(a.unpublish))

	mux.Handle("POST /api/v1/versions/{versionId}/modules", p(a.addModule))
	mux.Handle("POST /api/v1/modules/{moduleId}/units", p(a.addUnit))
	mux.Handle("POST /api/v1/units/{unitId}/resources", p(a.addResource))
	mux.Handle("PATCH /api/v1/resources/{resourceId}", p(a.updateResource))
	mux.Handle("PATCH /api/v1/resources/{resourceId}/content", p(a.saveContent))
}

func actor(r *http.Request) Actor {
	pr, _ := httpx.PrincipalFrom(r.Context())
	return Actor{
		UserID: pr.UserID, Role: pr.Role,
		IP: httpx.ClientIP(r), Agent: r.UserAgent(),
		Trace: httpx.RequestIDFrom(r.Context()),
	}
}

func idDe(r *http.Request, nombre string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(nombre))
	if err != nil {
		return uuid.Nil, problem.Validation("El identificador de la ruta no es un UUID válido.")
	}
	return id, nil
}

func numeroDe(r *http.Request, nombre string) (int, error) {
	var n int
	s := r.PathValue(nombre)
	if s == "" {
		return 0, problem.Validation("Falta el número de versión.")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, problem.Validation("El número de versión debe ser un entero.")
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return 0, problem.Validation("El número de versión empieza en 1.")
	}
	return n, nil
}

// ------------------------------------------------------------- respuestas --

type courseView struct {
	ID               string    `json:"id"`
	Slug             string    `json:"slug"`
	Title            string    `json:"title"`
	Summary          string    `json:"summary"`
	Category         string    `json:"category"`
	Language         string    `json:"language"`
	OwnerID          string    `json:"owner_id"`
	Status           string    `json:"status"`
	CurrentVersionID *string   `json:"current_version_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func toCourseView(c Course) courseView {
	v := courseView{
		ID: c.ID.String(), Slug: c.Slug, Title: c.Title, Summary: c.Summary,
		Category: c.Category, Language: c.Language, OwnerID: c.OwnerID.String(),
		Status: c.Status, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
	if c.CurrentVersionID != nil {
		s := c.CurrentVersionID.String()
		v.CurrentVersionID = &s
	}
	return v
}

type versionView struct {
	ID               string           `json:"id"`
	CourseID         string           `json:"course_id"`
	VersionNumber    int              `json:"version_number"`
	Status           string           `json:"status"`
	ApprovalCriteria ApprovalCriteria `json:"approval_criteria"`
	PublishedAt      *time.Time       `json:"published_at"`
	CreatedAt        time.Time        `json:"created_at"`
	Modules          []moduleView     `json:"modules,omitempty"`
}

type moduleView struct {
	ID       string     `json:"id"`
	StableID string     `json:"stable_id"`
	Position int        `json:"position"`
	Title    string     `json:"title"`
	Units    []unitView `json:"units"`
}

type unitView struct {
	ID        string         `json:"id"`
	StableID  string         `json:"stable_id"`
	Position  int            `json:"position"`
	Title     string         `json:"title"`
	Resources []resourceView `json:"resources"`
}

// etagDelRecurso viaja en el cuerpo además de la cabecera: el cliente que lee
// el árbol completo necesita el ETag de cada recurso para poder editarlo.
type resourceView struct {
	ID               string  `json:"id"`
	StableID         string  `json:"stable_id"`
	Position         int     `json:"position"`
	Title            string  `json:"title"`
	Type             string  `json:"type"`
	Visible          bool    `json:"visible"`
	Required         bool    `json:"required"`
	Downloadable     bool    `json:"downloadable"`
	ContentMD        *string `json:"content_md,omitempty"`
	ContentHTML      *string `json:"content_html,omitempty"`
	AssetID          *string `json:"asset_id"`
	ExternalURL      *string `json:"external_url"`
	ProcessingStatus string  `json:"processing_status"`
	ETag             string  `json:"etag"`
}

func toResourceView(r Resource, conHTML bool) resourceView {
	v := resourceView{
		ID: r.ID.String(), StableID: r.StableID.String(), Position: r.Position,
		Title: r.Title, Type: r.Type, Visible: r.Visible, Required: r.Required,
		Downloadable: r.Downloadable, ContentMD: r.ContentMD, ExternalURL: r.ExternalURL,
		ProcessingStatus: r.ProcessingStatus(),
		ETag:             r.ContenidoETag(),
	}
	if r.AssetID != nil {
		s := r.AssetID.String()
		v.AssetID = &s
	}
	if conHTML && r.ContentMD != nil {
		if html, err := markdown.RenderHTML(*r.ContentMD); err == nil {
			v.ContentHTML = &html
			v.ContentMD = nil // en previsualización manda el HTML saneado
		}
	}
	return v
}

func toVersionView(v CourseVersion, conArbol, conHTML bool) versionView {
	out := versionView{
		ID: v.ID.String(), CourseID: v.CourseID.String(), VersionNumber: v.VersionNumber,
		Status: v.Status, ApprovalCriteria: v.ApprovalCriteria,
		PublishedAt: v.PublishedAt, CreatedAt: v.CreatedAt,
	}
	if !conArbol {
		return out
	}
	out.Modules = make([]moduleView, 0, len(v.Modules))
	for _, m := range v.Modules {
		mv := moduleView{ID: m.ID.String(), StableID: m.StableID.String(),
			Position: m.Position, Title: m.Title, Units: make([]unitView, 0, len(m.Units))}
		for _, u := range m.Units {
			uv := unitView{ID: u.ID.String(), StableID: u.StableID.String(),
				Position: u.Position, Title: u.Title, Resources: make([]resourceView, 0, len(u.Resources))}
			for _, r := range u.Resources {
				uv.Resources = append(uv.Resources, toResourceView(r, conHTML))
			}
			mv.Units = append(mv.Units, uv)
		}
		out.Modules = append(out.Modules, mv)
	}
	return out
}

// -------------------------------------------------------------- handlers ---

func (a *API) listCourses(w http.ResponseWriter, r *http.Request) {
	cursos, err := a.svc.ListCourses(r.Context(), actor(r), 100)
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	items := make([]courseView, 0, len(cursos))
	for _, c := range cursos {
		items = append(items, toCourseView(c))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nil})
}

type courseBody struct {
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Category string `json:"category"`
	Language string `json:"language"`
}

func (a *API) createCourse(w http.ResponseWriter, r *http.Request) {
	var body courseBody
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	c, v, err := a.svc.CreateCourse(r.Context(),
		CourseInput{Title: body.Title, Summary: body.Summary, Category: body.Category, Language: body.Language},
		actor(r))
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"course": toCourseView(c), "version": toVersionView(v, false, false),
	})
}

func (a *API) getCourse(w http.ResponseWriter, r *http.Request) {
	id, err := idDe(r, "courseId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	c, err := a.svc.GetCourse(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, toCourseView(c))
}

func (a *API) updateCourse(w http.ResponseWriter, r *http.Request) {
	id, err := idDe(r, "courseId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var body courseBody
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	c, err := a.svc.UpdateCourse(r.Context(), id,
		CourseInput{Title: body.Title, Summary: body.Summary, Category: body.Category, Language: body.Language},
		actor(r))
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, toCourseView(c))
}

func (a *API) listVersions(w http.ResponseWriter, r *http.Request) {
	id, err := idDe(r, "courseId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	vs, err := a.svc.ListVersions(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	items := make([]versionView, 0, len(vs))
	for _, v := range vs {
		items = append(items, toVersionView(v, false, false))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) newVersion(w http.ResponseWriter, r *http.Request) {
	id, err := idDe(r, "courseId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	v, err := a.svc.NewVersion(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, toVersionView(v, false, false))
}

func (a *API) versionTree(w http.ResponseWriter, r *http.Request, conHTML bool) {
	id, err := idDe(r, "courseId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	n, err := numeroDe(r, "n")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	c, v, err := a.svc.GetVersionTree(r.Context(), id, n, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"course": toCourseView(c), "version": toVersionView(v, true, conHTML),
	})
}

func (a *API) getVersion(w http.ResponseWriter, r *http.Request) { a.versionTree(w, r, false) }
func (a *API) preview(w http.ResponseWriter, r *http.Request)    { a.versionTree(w, r, true) }

func (a *API) validate(w http.ResponseWriter, r *http.Request) {
	id, err := idDe(r, "courseId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	n, err := numeroDe(r, "n")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	errs, err := a.svc.Validate(r.Context(), id, n, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	lista := make([]problem.FieldError, 0, len(errs))
	for _, e := range errs {
		lista = append(lista, problem.FieldError{Code: e.Code, Field: e.Field, Detail: e.Detail})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"publishable": len(errs) == 0, "errors": lista})
}

func (a *API) publish(w http.ResponseWriter, r *http.Request) {
	id, err := idDe(r, "courseId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	n, err := numeroDe(r, "n")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	v, err := a.svc.Publish(r.Context(), id, n, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, toVersionView(v, false, false))
}

func (a *API) unpublish(w http.ResponseWriter, r *http.Request) {
	id, err := idDe(r, "courseId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	n, err := numeroDe(r, "n")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	v, err := a.svc.Unpublish(r.Context(), id, n, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, toVersionView(v, false, false))
}

type tituloBody struct {
	Title string `json:"title"`
}

func (a *API) addModule(w http.ResponseWriter, r *http.Request) {
	id, err := idDe(r, "versionId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var body tituloBody
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	m, err := a.svc.AddModule(r.Context(), id, body.Title, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, moduleView{
		ID: m.ID.String(), StableID: m.StableID.String(), Position: m.Position,
		Title: m.Title, Units: []unitView{},
	})
}

func (a *API) addUnit(w http.ResponseWriter, r *http.Request) {
	id, err := idDe(r, "moduleId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var body tituloBody
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	u, err := a.svc.AddUnit(r.Context(), id, body.Title, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, unitView{
		ID: u.ID.String(), StableID: u.StableID.String(), Position: u.Position,
		Title: u.Title, Resources: []resourceView{},
	})
}

type resourceBody struct {
	Title        string  `json:"title"`
	Type         string  `json:"type"`
	Visible      *bool   `json:"visible"`
	Required     *bool   `json:"required"`
	Downloadable *bool   `json:"downloadable"`
	AssetID      *string `json:"asset_id"`
	ExternalURL  *string `json:"external_url"`
	ContentMD    *string `json:"content_md"`
}

func (b resourceBody) toInput() (ResourceInput, error) {
	in := ResourceInput{
		Title: b.Title, Type: b.Type, Visible: b.Visible, Required: b.Required,
		Downloadable: b.Downloadable, ExternalURL: b.ExternalURL, ContentMD: b.ContentMD,
	}
	if b.AssetID != nil && *b.AssetID != "" {
		id, err := uuid.Parse(*b.AssetID)
		if err != nil {
			return ResourceInput{}, problem.Validation("`asset_id` no es un UUID válido.")
		}
		in.AssetID = &id
	}
	return in, nil
}

func (a *API) addResource(w http.ResponseWriter, r *http.Request) {
	id, err := idDe(r, "unitId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var body resourceBody
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	in, err := body.toInput()
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	res, err := a.svc.AddResource(r.Context(), id, in, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	httpx.SetETag(w, res.ContenidoETag())
	httpx.JSON(w, http.StatusCreated, toResourceView(res, false))
}

func (a *API) updateResource(w http.ResponseWriter, r *http.Request) {
	id, err := idDe(r, "resourceId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var body resourceBody
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	in, err := body.toInput()
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	res, err := a.svc.UpdateResource(r.Context(), id, in, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, toResourceView(res, false))
}

func (a *API) saveContent(w http.ResponseWriter, r *http.Request) {
	id, err := idDe(r, "resourceId")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var body struct {
		ContentMD string `json:"content_md"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	res, err := a.svc.SaveContent(r.Context(), id, body.ContentMD,
		func(etagActual string) error { return httpx.ExigirIfMatch(r, etagActual) }, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducirError(err))
		return
	}
	// El ETag del contenido guardado: guardar lo mismo devuelve el mismo.
	httpx.SetETag(w, res.ContenidoETag())
	httpx.JSON(w, http.StatusOK, toResourceView(res, false))
}

// traducirError decide qué se le cuenta al cliente.
func traducirError(err error) error {
	var verrs ValidationErrors
	if errors.As(err, &verrs) {
		p := problem.Validation("La solicitud no cumple las reglas del curso.")
		for _, e := range verrs {
			p.Errors = append(p.Errors, problem.FieldError{Code: e.Code, Field: e.Field, Detail: e.Detail})
		}
		return p
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return problem.NotFound("El recurso solicitado no existe.")
	case errors.Is(err, ErrProhibido):
		return problem.Forbidden("No eres autor de este curso.")
	case errors.Is(err, ErrInmutable):
		return problem.Conflict("version_immutable",
			"La versión está publicada y es inmutable. Despublícala para editarla.")
	case errors.Is(err, ErrConflicto):
		return problem.Conflict("conflict", "La operación no es válida en el estado actual.")
	default:
		return err
	}
}
