package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"mooc/backend/internal/platform/httpx"
	"mooc/backend/internal/platform/problem"
)

type API struct{ svc *Service }

func NewAPI(svc *Service) *API { return &API{svc: svc} }

// Routes registra la administración. Todo exige rol de administrador.
func (a *API) Routes(mux *http.ServeMux, auth httpx.Middleware, soloAdmin httpx.Middleware) {
	p := func(h http.HandlerFunc) http.Handler { return auth(soloAdmin(h)) }

	mux.Handle("GET /api/v1/admin/users", p(a.listarUsuarios))
	mux.Handle("POST /api/v1/admin/users", p(a.crearUsuario))
	mux.Handle("GET /api/v1/admin/users/{id}", p(a.verUsuario))
	mux.Handle("PATCH /api/v1/admin/users/{id}", p(a.actualizarUsuario))
	mux.Handle("GET /api/v1/admin/users/{id}/sessions", p(a.sesiones))
	mux.Handle("DELETE /api/v1/admin/users/{id}/sessions", p(a.revocarSesiones))
	mux.Handle("GET /api/v1/admin/audit", p(a.auditoria))
	mux.Handle("GET /api/v1/admin/jobs", p(a.trabajos))
	mux.Handle("POST /api/v1/admin/jobs/{id}/requeue", p(a.reencolar))
}

func actor(r *http.Request) Actor {
	pr, _ := httpx.PrincipalFrom(r.Context())
	return Actor{UserID: pr.UserID, Role: pr.Role, IP: httpx.ClientIP(r),
		Agent: r.UserAgent(), Trace: httpx.RequestIDFrom(r.Context())}
}

func pathUUID(r *http.Request, n string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(n))
	if err != nil {
		return uuid.Nil, problem.Validation("El identificador de la ruta no es un UUID válido.")
	}
	return id, nil
}

func entero(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func vistaUsuario(u Usuario) map[string]any {
	return map[string]any{
		"id": u.ID.String(), "email": u.Email, "full_name": u.FullName,
		"role": u.Role, "status": u.Status,
		"email_verified_at": u.EmailVerifiedAt,
		"created_at":        u.CreatedAt,
	}
}

func (a *API) listarUsuarios(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	us, err := a.svc.ListarUsuarios(r.Context(), FiltroUsuarios{
		Role: q.Get("role"), Status: q.Get("status"),
		Buscar: q.Get("q"), Limit: entero(q.Get("limit"), 50),
	})
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	items := make([]map[string]any, 0, len(us))
	for _, u := range us {
		items = append(items, vistaUsuario(u))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nil})
}

func (a *API) crearUsuario(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		FullName string `json:"full_name"`
		Role     string `json:"role"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	u, err := a.svc.CrearUsuario(r.Context(),
		NuevoUsuario{Email: body.Email, FullName: body.FullName, Role: body.Role}, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, vistaUsuario(u))
}

func (a *API) verUsuario(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	u, err := a.svc.Usuario(r.Context(), id)
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusOK, vistaUsuario(u))
}

func (a *API) actualizarUsuario(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var body struct {
		Role   *string `json:"role"`
		Status *string `json:"status"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	u, err := a.svc.ActualizarUsuario(r.Context(), id,
		CambioDeUsuario{Role: body.Role, Status: body.Status}, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusOK, vistaUsuario(u))
}

func (a *API) sesiones(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	ss, err := a.svc.Sesiones(r.Context(), id)
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	items := make([]map[string]any, 0, len(ss))
	for _, s := range ss {
		items = append(items, map[string]any{
			"id": s.ID.String(), "ip": s.IP, "user_agent": s.UserAgent,
			"created_at": s.CreatedAt, "last_seen_at": s.LastSeenAt,
			"expires_at": s.ExpiresAt, "revoked_at": s.RevokedAt,
			"active": s.Activa(),
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) revocarSesiones(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	n, err := a.svc.RevocarSesiones(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"revoked": n,
		"detail":  "Las sesiones dejan de valer en la petición siguiente, sin ventana de gracia.",
	})
}

func (a *API) auditoria(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := FiltroAuditoria{
		Action: q.Get("action"), EntityType: q.Get("entity_type"),
		Limit: entero(q.Get("limit"), 50),
	}
	if v := q.Get("actor_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.ActorID = &id
		}
	}
	if v := q.Get("entity_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.EntityID = &id
		}
	}

	es, err := a.svc.Auditoria(r.Context(), f)
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	items := make([]map[string]any, 0, len(es))
	for _, e := range es {
		item := map[string]any{
			"id": e.ID, "occurred_at": e.OccurredAt, "actor_role": e.ActorRole,
			"action": e.Action, "entity_type": e.EntityType,
			"ip": e.IP, "trace_id": e.TraceID, "metadata": e.Metadata,
		}
		if e.ActorID != nil {
			item["actor_id"] = e.ActorID.String()
		}
		if e.EntityID != nil {
			item["entity_id"] = e.EntityID.String()
		}
		items = append(items, item)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nil})
}

func vistaTrabajo(t Trabajo) map[string]any {
	return map[string]any{
		"id": t.ID, "job_key": t.JobKey, "type": t.Type, "queue": t.Queue,
		"status": t.Status, "attempt": t.Attempt, "last_error": t.LastError,
		"heartbeat_at": t.HeartbeatAt, "created_at": t.CreatedAt,
		"finished_at": t.FinishedAt,
	}
}

func (a *API) trabajos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ts, err := a.svc.Trabajos(r.Context(), FiltroTrabajos{
		Status: q.Get("status"), Type: q.Get("type"), Limit: entero(q.Get("limit"), 50),
	})
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	items := make([]map[string]any, 0, len(ts))
	for _, t := range ts {
		items = append(items, vistaTrabajo(t))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nil})
}

func (a *API) reencolar(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Fail(w, r, problem.Validation("El identificador del trabajo debe ser un entero."))
		return
	}
	t, err := a.svc.Reencolar(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusAccepted, vistaTrabajo(t))
}

func traducir(err error) error {
	var verrs ValidationErrors
	if errors.As(err, &verrs) {
		p := problem.Validation("La solicitud no es válida.")
		for _, e := range verrs {
			p.Errors = append(p.Errors, problem.FieldError{Code: e.Code, Field: e.Field, Detail: e.Detail})
		}
		return p
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return problem.NotFound("El recurso solicitado no existe.")
	case errors.Is(err, ErrUltimoAdministrador):
		return problem.Conflict("last_admin",
			"No se puede dejar el sistema sin ningún administrador activo.")
	case errors.Is(err, ErrCorreoEnUso):
		return problem.Conflict("email_taken", "Ya existe una cuenta con ese correo.")
	case errors.Is(err, ErrNoReencolable):
		return problem.Conflict("job.not_requeueable", err.Error())
	default:
		return err
	}
}
