package httpx

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"mooc/backend/internal/platform/problem"
)

// ETagDe calcula el ETag a partir del CONTENIDO, no de la marca de tiempo.
//
// Con `updated_at` el ETag cambiaría al guardar dos veces lo mismo, y el
// autosave generaría revisiones falsas. Derivándolo del contenido, guardar lo
// mismo produce el mismo ETag, que es lo que el ADR-0014 promete.
func ETagDe(partes ...string) string {
	h := sha256.New()
	for _, p := range partes {
		h.Write([]byte(p))
		h.Write([]byte{0}) // separador: evita que "ab"+"c" y "a"+"bc" colisionen
	}
	return `"` + hex.EncodeToString(h.Sum(nil))[:32] + `"`
}

// SetETag pone la cabecera.
func SetETag(w http.ResponseWriter, etag string) { w.Header().Set("ETag", etag) }

// ExigirIfMatch comprueba la precondición de una escritura.
//
// La cabecera es obligatoria en los recursos editables: sin ella, dos pestañas
// abiertas se pisan y gana la última en guardar, sin que nadie se entere.
func ExigirIfMatch(r *http.Request, etagActual string) error {
	enviado := strings.TrimSpace(r.Header.Get("If-Match"))
	if enviado == "" {
		p := problem.New(http.StatusPreconditionRequired, "if_match_required",
			"Falta la precondición",
			"Este recurso exige la cabecera `If-Match` con el ETag que recibiste al leerlo.")
		return p
	}
	// `*` significa «existe, sea cual sea su versión».
	if enviado == "*" || coincide(enviado, etagActual) {
		return nil
	}
	p := problem.New(http.StatusPreconditionFailed, "precondition_failed",
		"El recurso cambió mientras tanto",
		"El `If-Match` no coincide con la versión actual. Vuelve a leer el recurso y resuelve el conflicto.")
	p.Errors = append(p.Errors, problem.FieldError{
		Code: "etag.mismatch", Field: "If-Match",
		Detail: "enviado " + enviado + ", actual " + etagActual,
	})
	return p
}

// coincide admite listas separadas por comas, como permite la norma.
func coincide(enviado, actual string) bool {
	for _, e := range strings.Split(enviado, ",") {
		e = strings.TrimSpace(e)
		// Un ETag débil (W/"...") compara igual para nuestro uso.
		e = strings.TrimPrefix(e, "W/")
		if e == actual {
			return true
		}
	}
	return false
}
