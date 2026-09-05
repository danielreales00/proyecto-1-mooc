package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"

	"mooc/backend/internal/platform/ids"
	"mooc/backend/internal/platform/problem"
)

type ctxKey int

const (
	ctxRequestID ctxKey = iota
	ctxPrincipal
)

// Principal es el sujeto autenticado. Vive en plataforma, no en identity, para
// que el middleware no dependa de un módulo de dominio (ADR-0001).
type Principal struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	Role      string
	// SessionToken es el token opaco de la petición. Se necesita para revocar
	// la sesión en Redis. Nunca se registra en logs.
	SessionToken string
}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxPrincipal, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxPrincipal).(Principal)
	return p, ok
}

func RequestIDFrom(ctx context.Context) string {
	s, _ := ctx.Value(ctxRequestID).(string)
	return s
}

type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, mw ...Middleware) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// RequestID asigna un identificador a cada petición y lo devuelve en la
// cabecera. Cuando entre OpenTelemetry, este será el trace_id (ADR-0009).
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-Id")
			if id == "" {
				id = ids.New().String()
				r.Header.Set("X-Request-Id", id)
			}
			w.Header().Set("X-Request-Id", id)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxRequestID, id)))
		})
	}
}

// Recover convierte un panic en un 500 uniforme en lugar de cerrar la conexión.
func Recover(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic en el handler",
						"panic", rec,
						"path", r.URL.Path,
						"trace_id", RequestIDFrom(r.Context()),
						"stack", string(debug.Stack()))
					problem.Write(w, r, problem.Internal())
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// AccessLog emite una línea JSON por petición con los campos obligatorios del
// ADR-0009. Nunca registra cabeceras de autorización.
func AccessLog(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)
			if sw.status == 0 {
				sw.status = http.StatusOK
			}

			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"bytes", sw.bytes,
				"duration_ms", time.Since(start).Milliseconds(),
				"trace_id", RequestIDFrom(r.Context()),
				"remote_ip", ClientIP(r),
			}
			if p, ok := PrincipalFrom(r.Context()); ok {
				attrs = append(attrs, "user_id", p.UserID.String(), "role", p.Role)
			}

			switch {
			case sw.status >= 500:
				log.Error("petición", attrs...)
			case sw.status >= 400:
				log.Warn("petición", attrs...)
			default:
				log.Info("petición", attrs...)
			}
		})
	}
}

func ClientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		for i := 0; i < len(v); i++ {
			if v[i] == ',' {
				return v[:i]
			}
		}
		return v
	}
	return r.RemoteAddr
}
