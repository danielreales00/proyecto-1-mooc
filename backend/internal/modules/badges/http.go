package badges

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"mooc/backend/internal/platform/httpx"
	"mooc/backend/internal/platform/problem"
)

type API struct{ svc *Service }

func NewAPI(svc *Service) *API { return &API{svc: svc} }

func (a *API) Routes(mux *http.ServeMux, auth httpx.Middleware, soloAdmin httpx.Middleware) {
	// La verificación es PÚBLICA: es el punto de CA-07.
	mux.HandleFunc("GET /api/v1/verify/{code}", a.verify)

	mux.Handle("GET /api/v1/me/badges", auth(http.HandlerFunc(a.mine)))
	mux.Handle("POST /api/v1/badges/{id}/revoke", auth(soloAdmin(http.HandlerFunc(a.revoke))))
}

// vistaPrivada la ve el dueño de la insignia o un administrador.
func vistaPrivada(b Badge, base string) map[string]any {
	return map[string]any{
		"id": b.ID.String(), "enrollment_id": b.EnrollmentID.String(),
		"course_id": b.CourseID.String(), "course_title": b.CourseTitle,
		"public_code":      b.PublicCode,
		"verification_url": base + "/api/v1/verify/" + b.PublicCode,
		"issued_at":        b.IssuedAt, "revoked_at": b.RevokedAt,
		"revocation_reason": b.RevocationReason,
		"status":            b.Estado(),
	}
}

func (a *API) mine(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	bs, err := a.svc.MyBadges(r.Context(), p.UserID)
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	base := baseURL(r)
	items := make([]map[string]any, 0, len(bs))
	for _, b := range bs {
		items = append(items, vistaPrivada(b, base))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// verify es la verificación pública. La respuesta NO incluye el correo del
// estudiante, y el código no deriva de su identidad (CA-07).
func (a *API) verify(w http.ResponseWriter, r *http.Request) {
	b, err := a.svc.Verify(r.Context(), r.PathValue("code"))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"public_code":    b.PublicCode,
		"status":         b.Estado(),
		"student_name":   b.StudentName,
		"course_title":   b.CourseTitle,
		"course_version": b.VersionNumber,
		"issued_at":      b.IssuedAt,
		"revoked_at":     b.RevokedAt,
	})
}

func (a *API) revoke(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.Fail(w, r, problem.Validation("El identificador no es un UUID válido."))
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	if body.Reason == "" {
		httpx.Fail(w, r, problem.Validation("Se requiere un motivo para revocar."))
		return
	}
	p, _ := httpx.PrincipalFrom(r.Context())
	b, err := a.svc.Revoke(r.Context(), id, p.UserID, body.Reason)
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.JSON(w, http.StatusOK, vistaPrivada(b, baseURL(r)))
}

func baseURL(r *http.Request) string {
	esquema := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		esquema = "https"
	}
	return esquema + "://" + r.Host
}

func traducir(err error) error {
	if errors.Is(err, ErrNotFound) {
		return problem.NotFound("No existe una insignia con ese código.")
	}
	return err
}
