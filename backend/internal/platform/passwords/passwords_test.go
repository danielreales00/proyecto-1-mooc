package passwords

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestHashYVerify(t *testing.T) {
	const clave = "contrasena-de-prueba"

	encoded, err := Hash(clave)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if strings.Contains(encoded, clave) {
		t.Fatal("el hash contiene la contraseña en claro")
	}
	if !strings.HasPrefix(encoded, "$argon2id$") {
		t.Fatalf("el hash no está en formato PHC de argon2id: %q", encoded)
	}
	if err := Verify(clave, encoded); err != nil {
		t.Fatalf("Verify con la clave correcta: %v", err)
	}
}

func TestVerifyRechazaClaveIncorrecta(t *testing.T) {
	encoded, err := Hash("la-correcta")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if err := Verify("la-incorrecta", encoded); !errors.Is(err, ErrMismatch) {
		t.Fatalf("se esperaba ErrMismatch, llegó %v", err)
	}
}

// La sal es por usuario: dos cuentas con la misma contraseña no comparten hash.
// Sin esto, un volcado de la tabla revela qué usuarios repiten contraseña.
func TestHashUsaSalDistintaCadaVez(t *testing.T) {
	a, err := Hash("misma-contrasena")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	b, err := Hash("misma-contrasena")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if a == b {
		t.Fatal("dos hashes de la misma contraseña salieron idénticos: no hay sal por usuario")
	}
	if err := Verify("misma-contrasena", a); err != nil {
		t.Errorf("Verify sobre el primero: %v", err)
	}
	if err := Verify("misma-contrasena", b); err != nil {
		t.Errorf("Verify sobre el segundo: %v", err)
	}
}

func TestVerifyRechazaHashesMalformados(t *testing.T) {
	casos := map[string]string{
		"vacío":                 "",
		"sin formato PHC":       "no-es-un-hash",
		"algoritmo distinto":    "$argon2i$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
		"versión desconocida":   "$argon2id$v=99$m=65536,t=3,p=2$c2FsdA$aGFzaA",
		"parámetros ilegibles":  "$argon2id$v=19$m=xx,t=yy,p=zz$c2FsdA$aGFzaA",
		"sal no es base64":      "$argon2id$v=19$m=65536,t=3,p=2$!!!$aGFzaA",
		"hash no es base64":     "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$!!!",
		"le faltan componentes": "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA",
	}
	for nombre, encoded := range casos {
		t.Run(nombre, func(t *testing.T) {
			if err := Verify("cualquiera", encoded); err == nil {
				t.Fatal("se esperaba error y no lo hubo")
			}
		})
	}
}

// Los parámetros van dentro del hash, así que Verify debe usar los del hash y
// no los constantes del paquete. Es lo que permite subir el coste en el futuro
// sin invalidar las contraseñas ya guardadas.
func TestVerifyUsaLosParametrosDelHashYNoLosDelPaquete(t *testing.T) {
	// Hash forjado con t=1 y m=32 MiB, distintos de los valores actuales.
	const (
		otroT uint32 = 1
		otroM uint32 = 32 * 1024
		otroP uint8  = 1
	)
	if otroT == iterations && otroM == memory && otroP == parallelism {
		t.Fatal("los parámetros alternativos coinciden con los del paquete; la prueba no probaría nada")
	}

	salt := []byte("dieciseis-bytes!")
	clave := "contrasena-antigua"
	key := argon2.IDKey([]byte(clave), salt, otroT, otroM, otroP, keyLength)
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, otroM, otroT, otroP,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key))

	if err := Verify(clave, encoded); err != nil {
		t.Fatalf("Verify sobre un hash con parámetros antiguos: %v", err)
	}
	if err := Verify("otra-cosa", encoded); !errors.Is(err, ErrMismatch) {
		t.Fatalf("se esperaba ErrMismatch, llegó %v", err)
	}
}

// Un hash con la parte de clave vacía haría que ConstantTimeCompare devolviera 1
// para cualquier entrada. Hash() nunca lo produce, pero un registro corrupto o
// importado sí podría: sería una omisión de autenticación completa.
func TestVerifyNoAceptaCualquierContrasenaConHashDegenerado(t *testing.T) {
	degenerados := []string{
		"$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$", // clave vacía
		"$argon2id$v=19$m=65536,t=3,p=2$$",       // sal y clave vacías
		"$argon2id$v=19$m=65536,t=3,p=2$$aGFzaA", // sin sal
	}
	contrasenas := []string{"", "cualquiera", "otra-distinta", strings.Repeat("x", 100)}

	for _, encoded := range degenerados {
		for _, clave := range contrasenas {
			if err := Verify(clave, encoded); err == nil {
				t.Fatalf("Verify(%q, %q) aceptó la contraseña: omisión de autenticación", clave, encoded)
			}
		}
	}
}
