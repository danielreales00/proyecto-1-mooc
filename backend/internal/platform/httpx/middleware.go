package httpx

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
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

// proxiesDeConfianza es cuántas direcciones añade la infraestructura a la
// DERECHA de la IP real del cliente en X-Forwarded-For.
//
// Con Caddy delante vale 0: el proxy reescribe la cabecera entera con la
// dirección del que llama, así que solo hay un valor y es el bueno. Detrás de
// un balanceador de Google vale 1, porque añade la suya después de la del
// cliente. El número se configura, no se adivina.
var proxiesDeConfianza int

// ConfigurarProxiesDeConfianza lo fija al arrancar. No hay candado porque se
// llama una vez, antes de atender la primera petición.
func ConfigurarProxiesDeConfianza(n int) {
	if n < 0 {
		n = 0
	}
	proxiesDeConfianza = n
}

// ClientIP devuelve la dirección del cliente para auditar y para limitar por
// IP.
//
// Se cuenta desde la derecha a propósito. X-Forwarded-For es una lista que
// cualquiera puede empezar: si el cliente manda la suya, queda a la IZQUIERDA
// de la real, porque los proxies añaden al final. Tomar el primer valor —que
// es lo que hacía esta función— convierte el límite por IP en decorativo
// detrás de un balanceador que añade en vez de reescribir: basta inventar una
// cabecera distinta en cada petición.
func ClientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		partes := strings.Split(v, ",")
		i := len(partes) - 1 - proxiesDeConfianza
		if i < 0 {
			// Vienen menos saltos de los configurados: alguien llega por un
			// camino que no es el previsto. Se usa el más a la izquierda, que
			// es el más antiguo de los que hay.
			i = 0
		}
		if ip := strings.TrimSpace(partes[i]); ip != "" {
			return ip
		}
	}
	return sinPuerto(r.RemoteAddr)
}

// sinPuerto recorta el puerto de origen. Sin esto, dos peticiones del mismo
// cliente son dos claves distintas en el límite de tasa, porque el puerto
// efímero cambia en cada conexión.
func sinPuerto(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
