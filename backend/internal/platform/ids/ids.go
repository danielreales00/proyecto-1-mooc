// Package ids genera los identificadores del proyecto.
//
// UUIDv7 (ADR-0003): ordenado en el tiempo, buena localidad de índice y no
// revela cardinalidad como haría un serial.
package ids

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/google/uuid"
)

func New() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		// NewV7 solo falla si la fuente de entropía falla; en ese caso v4 sirve.
		return uuid.New()
	}
	return id
}

// Token devuelve un secreto opaco de 32 bytes en base64url, para sesiones y
// tokens de un solo uso (ADR-0006).
func Token() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// PublicCode devuelve un código corto para URLs públicas (insignias).
func PublicCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
