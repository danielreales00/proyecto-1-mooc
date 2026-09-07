// Package httpx contiene el andamiaje HTTP compartido: middleware, decodificación
// y escritura de respuestas. No conoce ningún módulo de dominio (ADR-0001).
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"mooc/backend/internal/platform/problem"
)

const maxBodyBytes = 1 << 20 // 1 MiB; los binarios no pasan por la API (ADR-0005)

// JSON escribe una respuesta correcta. Los errores van siempre por problem.Write.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// Decode lee el cuerpo JSON con límite de tamaño y rechaza campos desconocidos.
//
// Rechazar campos desconocidos no es cosmético: es lo que hace que
// POST /progress devuelva 422 cuando el cliente manda progress_percent
// (CA-05, ADR-0012).
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			return problem.New(http.StatusRequestEntityTooLarge, "payload_too_large",
				"Cuerpo demasiado grande", "El cuerpo supera 1 MiB.")
		case errors.Is(err, io.EOF):
			return problem.Validation("El cuerpo de la solicitud está vacío.")
		default:
			return problem.Validation(err.Error())
		}
	}
	if dec.More() {
		return problem.Validation("El cuerpo contiene más de un documento JSON.")
	}
	return nil
}

// Fail traduce cualquier error a una respuesta uniforme. Un error que no sea
// *problem.Problem se considera interno: al cliente se le da un mensaje
// genérico, pero la causa **sí se registra** con el trace_id de la petición.
// Sin eso, un 500 no deja rastro de por qué ocurrió.
func Fail(w http.ResponseWriter, r *http.Request, err error) {
	var p *problem.Problem
	if errors.As(err, &p) {
		problem.Write(w, r, p)
		return
	}
	slog.Error("error interno no controlado",
		"error", err,
		"method", r.Method,
		"path", r.URL.Path,
		"trace_id", RequestIDFrom(r.Context()))
	problem.Write(w, r, problem.Internal())
}
