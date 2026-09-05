// Package problem implementa el formato uniforme de errores del ADR-0009:
// RFC 9457, application/problem+json.
package problem

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

const baseType = "https://mooc.local/errors/"

// FieldError es un incumplimiento concreto. El arreglo de estos es lo que
// permite devolver la "lista exhaustiva de errores" que exige CA-01.
type FieldError struct {
	Code   string `json:"code"`
	Field  string `json:"field,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type Problem struct {
	Type     string       `json:"type"`
	Title    string       `json:"title"`
	Status   int          `json:"status"`
	Detail   string       `json:"detail,omitempty"`
	Instance string       `json:"instance,omitempty"`
	TraceID  string       `json:"trace_id,omitempty"`
	Errors   []FieldError `json:"errors,omitempty"`
}

func (p *Problem) Error() string { return p.Title + ": " + p.Detail }

func New(status int, code, title, detail string) *Problem {
	return &Problem{Type: baseType + code, Title: title, Status: status, Detail: detail}
}

func (p *Problem) With(errs ...FieldError) *Problem {
	p.Errors = append(p.Errors, errs...)
	return p
}

func Unauthorized(detail string) *Problem {
	return New(http.StatusUnauthorized, "unauthorized", "No autenticado", detail)
}

func Forbidden(detail string) *Problem {
	return New(http.StatusForbidden, "forbidden", "No autorizado", detail)
}

func NotFound(detail string) *Problem {
	return New(http.StatusNotFound, "not_found", "No encontrado", detail)
}

func Conflict(code, detail string) *Problem {
	return New(http.StatusConflict, code, "Conflicto de estado", detail)
}

func Validation(detail string, errs ...FieldError) *Problem {
	return New(http.StatusUnprocessableEntity, "validation_failed",
		"La solicitud no es válida", detail).With(errs...)
}

func RateLimited(detail string) *Problem {
	return New(http.StatusTooManyRequests, "rate_limited", "Demasiadas solicitudes", detail)
}

func Internal() *Problem {
	return New(http.StatusInternalServerError, "internal", "Error interno",
		"Ocurrió un error inesperado. Use el trace_id para correlacionar.")
}

// Write serializa el problema. El trace_id viaja en el cuerpo para poder pegar
// una respuesta de Postman con su traza durante la demostración (ADR-0009).
func Write(w http.ResponseWriter, r *http.Request, p *Problem) {
	if p == nil {
		p = Internal()
	}
	p.Instance = r.URL.Path
	if p.TraceID == "" {
		p.TraceID = TraceIDFrom(r)
	}
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(p.Status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		slog.Error("no se pudo escribir el problema", "error", err)
	}
}

// traceIDKey lo pone el middleware de httpx; se resuelve por cabecera para no
// crear una dependencia circular entre paquetes de plataforma.
func TraceIDFrom(r *http.Request) string {
	return r.Header.Get("X-Request-Id")
}
