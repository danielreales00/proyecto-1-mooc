package ratelimit

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"mooc/backend/internal/platform/httpx"
	"mooc/backend/internal/platform/metrics"
	"mooc/backend/internal/platform/problem"
)

// Sujeto decide a quién se le cuenta la petición: la IP, la sesión, o un campo
// del cuerpo. Devolver cadena vacía desactiva el límite para esa petición.
type Sujeto func(r *http.Request) string

// PorIP es el sujeto habitual en los endpoints sin sesión.
func PorIP(r *http.Request) string { return httpx.ClientIP(r) }

// PorSesion cuenta por sesión, para endpoints autenticados de alto volumen.
// Si no hay sesión, cae a la IP.
func PorSesion(r *http.Request) string {
	if p, ok := httpx.PrincipalFrom(r.Context()); ok {
		return p.UserID.String()
	}
	return httpx.ClientIP(r)
}

// PorCampoDelCuerpo cuenta por un campo del JSON de la petición, típicamente
// el correo en el login.
//
// Limitar el login SOLO por IP castiga a quien comparte salida a internet: una
// universidad entera detrás de un NAT, o el propio CI ejecutando varias suites
// seguidas. Y limitar solo por cuenta deja rociar contraseñas contra muchas
// cuentas desde una sola máquina. Por eso se aplican los dos: uno estrecho por
// cuenta y otro ancho por IP.
//
// El cuerpo se restaura para que el handler pueda volver a leerlo.
func PorCampoDelCuerpo(campo string) Sujeto {
	return func(r *http.Request) string {
		if r.Body == nil {
			return ""
		}
		crudo, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
		r.Body = io.NopCloser(bytes.NewReader(crudo))
		if err != nil {
			return ""
		}
		var m map[string]any
		if err := json.Unmarshal(crudo, &m); err != nil {
			return ""
		}
		v, _ := m[campo].(string)
		return strings.ToLower(strings.TrimSpace(v))
	}
}

// Middleware aplica una regla. Las cabeceras siguen la convención habitual
// para que un cliente pueda auto-regularse sin llegar al 429.
func (l *Limiter) Middleware(regla Regla, sujeto Sujeto, log *slog.Logger) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s := sujeto(r)
			if s == "" {
				next.ServeHTTP(w, r)
				return
			}

			v, err := l.Permitir(r.Context(), regla, s)
			if err != nil {
				// Se permitió la petición; queda constancia de que el
				// limitador no está funcionando.
				log.Error("el limitador de tasa falló", "regla", regla.Nombre, "error", err)
			}

			w.Header().Set("RateLimit-Limit", strconv.Itoa(regla.Limite))
			w.Header().Set("RateLimit-Remaining", strconv.Itoa(v.Restantes))
			w.Header().Set("RateLimit-Reset", strconv.Itoa(int(v.ReintentarEn.Seconds())))

			if !v.Permitido {
				segundos := int(v.ReintentarEn.Seconds())
				if segundos < 1 {
					segundos = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(segundos))
				metrics.RateLimited.WithLabelValues(regla.Nombre).Inc()
				log.Warn("límite de tasa superado",
					"regla", regla.Nombre, "sujeto", s,
					"path", r.URL.Path, "trace_id", httpx.RequestIDFrom(r.Context()))
				problem.Write(w, r, problem.RateLimited(
					"Demasiadas solicitudes. Inténtalo de nuevo en "+strconv.Itoa(segundos)+" s."))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
