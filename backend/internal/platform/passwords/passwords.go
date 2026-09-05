// Package passwords implementa el hash de contraseñas con Argon2id (ADR-0006).
package passwords

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parámetros del ADR-0006: m=64 MiB, t=3, p=2.
const (
	memory      uint32 = 64 * 1024
	iterations  uint32 = 3
	parallelism uint8  = 2
	saltLength         = 16
	keyLength   uint32 = 32
)

var ErrMismatch = errors.New("la contraseña no coincide")

// Hash devuelve la codificación estándar de PHC, que lleva los parámetros
// dentro: subirlos en el futuro no invalida los hashes existentes.
func Hash(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generar sal: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memory, iterations, parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify compara en tiempo constante.
func Verify(password, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return fmt.Errorf("hash con formato desconocido")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return fmt.Errorf("versión de argon2 no soportada")
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return fmt.Errorf("parámetros de argon2 ilegibles: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return fmt.Errorf("sal ilegible: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return fmt.Errorf("hash ilegible: %w", err)
	}

	// Sin estas dos comprobaciones, un hash almacenado con la parte de clave
	// vacía verificaría CUALQUIER contraseña: subtle.ConstantTimeCompare
	// devuelve 1 al comparar dos slices vacíos. Hash() nunca produce eso, pero
	// Verify no debe confiar en que lo almacenado venga de Hash().
	if len(salt) == 0 {
		return fmt.Errorf("el hash no tiene sal")
	}
	if len(want) != int(keyLength) {
		return fmt.Errorf("longitud de clave inesperada: %d, se esperaba %d", len(want), keyLength)
	}

	got := argon2.IDKey([]byte(password), salt, t, m, p, keyLength)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrMismatch
	}
	return nil
}
