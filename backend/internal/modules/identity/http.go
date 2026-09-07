package identity

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"mooc/backend/internal/platform/httpx"
	"mooc/backend/internal/platform/problem"
)

type API struct {
	svc *Service
}

func NewAPI(svc *Service) *API { return &API{svc: svc} }

// Routes registra los endpoints públicos y los autenticados. El ruteo por
// método y comodín es el ServeMux de la stdlib (ADR-0002).
func (a *API) Routes(mux *http.ServeMux, auth httpx.Middleware) {
	mux.HandleFunc("POST /api/v1/auth/register", a.register)
	mux.HandleFunc("POST /api/v1/auth/verify-email", a.verifyEmail)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)

	mux.Handle("POST /api/v1/auth/logout", auth(http.HandlerFunc(a.logout)))
	mux.Handle("GET /api/v1/me", auth(http.HandlerFunc(a.me)))
}

func requestContext(r *http.Request) RequestContext {
	return RequestContext{
		IP:        httpx.ClientIP(r),
		UserAgent: r.UserAgent(),
		TraceID:   httpx.RequestIDFrom(r.Context()),
	}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

type registerResponse struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}

	user, err := a.svc.Register(r.Context(), RegisterInput{
		Email: req.Email, Password: req.Password, FullName: req.FullName,
	}, requestContext(r))
	if err != nil {
		httpx.Fail(w, r, translate(err))
		return
	}

	// 202: la cuenta existe pero el correo de verificación se envía de forma
	// asíncrona. La API no espera al worker (CA-02).
	httpx.JSON(w, http.StatusAccepted, registerResponse{
		ID:     user.ID.String(),
		Email:  user.Email,
		Status: "pending_verification",
		Detail: "Revisa tu correo para verificar la cuenta.",
	})
}

type verifyRequest struct {
	Token string `json:"token"`
}

func (a *API) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	if err := a.svc.VerifyEmail(r.Context(), strings.TrimSpace(req.Token), requestContext(r)); err != nil {
		httpx.Fail(w, r, translate(err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"verified": true,
		"detail":   "Correo verificado. Ya puedes iniciar sesión.",
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userView struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	FullName        string     `json:"full_name"`
	Role            string     `json:"role"`
	Status          string     `json:"status"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

func toUserView(u User) userView {
	return userView{
		ID: u.ID.String(), Email: u.Email, FullName: u.FullName,
		Role: u.Role, Status: u.Status,
		EmailVerifiedAt: u.EmailVerifiedAt, CreatedAt: u.CreatedAt,
	}
}

type loginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      userView  `json:"user"`
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	res, err := a.svc.Login(r.Context(), req.Email, req.Password, requestContext(r))
	if err != nil {
		httpx.Fail(w, r, translate(err))
		return
	}
	httpx.JSON(w, http.StatusOK, loginResponse{
		Token: res.Token, ExpiresAt: res.ExpiresAt, User: toUserView(res.User),
	})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	p, ok := httpx.PrincipalFrom(r.Context())
	if !ok {
		httpx.Fail(w, r, problem.Unauthorized("Se requiere una sesión activa."))
		return
	}
	err := a.svc.Logout(r.Context(), p.SessionToken, p.SessionID, p.UserID, p.Role, requestContext(r))
	if err != nil {
		httpx.Fail(w, r, translate(err))
		return
	}
	httpx.NoContent(w)
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	p, ok := httpx.PrincipalFrom(r.Context())
	if !ok {
		httpx.Fail(w, r, problem.Unauthorized("Se requiere una sesión activa."))
		return
	}
	user, err := a.svc.UserByID(r.Context(), p.UserID)
	if err != nil {
		httpx.Fail(w, r, translate(err))
		return
	}
	httpx.JSON(w, http.StatusOK, toUserView(user))
}

// Authenticate es el middleware de sesión. Resuelve el token opaco contra
// Redis en cada petición: revocar borra la clave y el efecto es inmediato
// (ADR-0006).
func Authenticate(svc *Service) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				problem.Write(w, r, problem.Unauthorized("Falta la cabecera Authorization: Bearer."))
				return
			}
			session, err := svc.Authenticate(r.Context(), token)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					problem.Write(w, r, problem.Unauthorized("La sesión no existe, expiró o fue revocada."))
					return
				}
				httpx.Fail(w, r, err)
				return
			}
			ctx := httpx.WithPrincipal(r.Context(), httpx.Principal{
				UserID:       session.UserID,
				SessionID:    session.ID,
				Role:         session.Role,
				SessionToken: token,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(h[len(prefix):])
	return token, token != ""
}

// translate convierte errores del dominio en respuestas uniformes. Es el único
// punto donde el módulo decide qué se le cuenta al cliente.
func translate(err error) error {
	var verrs ValidationErrors
	if errors.As(err, &verrs) {
		p := problem.Validation("La solicitud tiene campos inválidos.")
		for _, e := range verrs {
			p.Errors = append(p.Errors, problem.FieldError{Code: e.Code, Field: e.Field, Detail: e.Detail})
		}
		return p
	}
	switch {
	case errors.Is(err, ErrEmailTaken):
		return problem.Conflict("email_taken", "Ya existe una cuenta con ese correo.")
	case errors.Is(err, ErrInvalidCredentials):
		// Mismo mensaje para usuario inexistente, contraseña incorrecta y cuenta
		// no verificada: no se revela qué correos existen.
		return problem.Unauthorized("Correo o contraseña incorrectos, o la cuenta no está verificada.")
	case errors.Is(err, ErrTokenInvalid):
		return problem.Validation("El token es inválido, ya se usó o expiró.")
	case errors.Is(err, ErrNotFound):
		return problem.NotFound("El recurso solicitado no existe.")
	default:
		return err
	}
}

// RequireRole restringe un endpoint a los roles indicados. Es el control de
// acceso por rol que exige CA-06; la propiedad la comprueba cada módulo.
func RequireRole(roles ...string) httpx.Middleware {
	permitidos := make(map[string]bool, len(roles))
	for _, r := range roles {
		permitidos[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := httpx.PrincipalFrom(r.Context())
			if !ok {
				problem.Write(w, r, problem.Unauthorized("Se requiere una sesión activa."))
				return
			}
			if !permitidos[p.Role] {
				problem.Write(w, r, problem.Forbidden("Tu rol no permite esta operación."))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
