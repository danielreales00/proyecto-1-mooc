package identity

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// La validación debe acumular TODOS los incumplimientos, no parar en el
// primero: es lo que hace posible la lista exhaustiva de errores que exige
// CA-01 y que el segmento 2 de la demostración tiene que mostrar.
func TestRegisterInputValidateAcumulaTodosLosErrores(t *testing.T) {
	in := RegisterInput{Email: "no-es-un-correo", Password: "corta", FullName: "  "}

	errs := in.Validate()

	if len(errs) != 3 {
		t.Fatalf("se esperaban 3 errores, llegaron %d: %v", len(errs), errs)
	}
	want := map[string]bool{
		"email.invalid":      false,
		"password.too_short": false,
		"full_name.required": false,
	}
	for _, e := range errs {
		if _, ok := want[e.Code]; !ok {
			t.Errorf("código inesperado: %q", e.Code)
			continue
		}
		want[e.Code] = true
		if e.Field == "" || e.Detail == "" {
			t.Errorf("el error %q no trae campo o detalle: %+v", e.Code, e)
		}
	}
	for code, seen := range want {
		if !seen {
			t.Errorf("falta el error %q", code)
		}
	}
}

func TestRegisterInputValidate(t *testing.T) {
	casos := []struct {
		nombre string
		in     RegisterInput
		code   string // "" = sin errores
	}{
		{
			nombre: "válido",
			in:     RegisterInput{Email: "ana@mooc.local", Password: "contrasena-larga", FullName: "Ana"},
		},
		{
			nombre: "el correo se normaliza antes de validar",
			in:     RegisterInput{Email: "  ANA@MOOC.LOCAL  ", Password: "contrasena-larga", FullName: "Ana"},
		},
		{
			nombre: "correo vacío",
			in:     RegisterInput{Password: "contrasena-larga", FullName: "Ana"},
			code:   "email.required",
		},
		{
			nombre: "correo demasiado largo",
			in:     RegisterInput{Email: strings.Repeat("a", 250) + "@mooc.local", Password: "contrasena-larga", FullName: "Ana"},
			code:   "email.too_long",
		},
		{
			nombre: "contraseña en el límite inferior",
			in:     RegisterInput{Email: "ana@mooc.local", Password: strings.Repeat("x", minPasswordLen), FullName: "Ana"},
		},
		{
			nombre: "contraseña un carácter por debajo del límite",
			in:     RegisterInput{Email: "ana@mooc.local", Password: strings.Repeat("x", minPasswordLen-1), FullName: "Ana"},
			code:   "password.too_short",
		},
		{
			nombre: "contraseña por encima del máximo",
			in:     RegisterInput{Email: "ana@mooc.local", Password: strings.Repeat("x", maxPasswordLen+1), FullName: "Ana"},
			code:   "password.too_long",
		},
		{
			// La longitud se mide en runas, no en bytes: una contraseña de 10
			// caracteres acentuados es válida aunque ocupe más de 10 bytes.
			nombre: "contraseña con acentos en el límite",
			in:     RegisterInput{Email: "ana@mooc.local", Password: strings.Repeat("á", minPasswordLen), FullName: "Ana"},
		},
		{
			nombre: "nombre solo con espacios",
			in:     RegisterInput{Email: "ana@mooc.local", Password: "contrasena-larga", FullName: "   "},
			code:   "full_name.required",
		},
		{
			nombre: "nombre demasiado largo",
			in:     RegisterInput{Email: "ana@mooc.local", Password: "contrasena-larga", FullName: strings.Repeat("á", maxNameLen+1)},
			code:   "full_name.too_long",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			errs := c.in.Validate()
			if c.code == "" {
				if !errs.Empty() {
					t.Fatalf("se esperaba válido, llegó: %v", errs)
				}
				return
			}
			for _, e := range errs {
				if e.Code == c.code {
					return
				}
			}
			t.Fatalf("se esperaba el error %q, llegaron: %v", c.code, errs)
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	casos := map[string]string{
		"  Ana@Mooc.Local ": "ana@mooc.local",
		"ANA@MOOC.LOCAL":    "ana@mooc.local",
		"ana@mooc.local":    "ana@mooc.local",
		"":                  "",
	}
	for entrada, esperado := range casos {
		if got := NormalizeEmail(entrada); got != esperado {
			t.Errorf("NormalizeEmail(%q) = %q, se esperaba %q", entrada, got, esperado)
		}
	}
}

// CanLogIn concentra las condiciones de acceso. Que una cuenta suspendida o sin
// verificar no pueda entrar es control de acceso (CA-06), no cortesía.
func TestUserCanLogIn(t *testing.T) {
	verificado := time.Now().UTC()

	casos := []struct {
		nombre string
		user   User
		puede  bool
	}{
		{"activa y verificada", User{Status: StatusActive, EmailVerifiedAt: &verificado}, true},
		{"activa sin verificar", User{Status: StatusActive}, false},
		{"suspendida aunque verificada", User{Status: StatusSuspended, EmailVerifiedAt: &verificado}, false},
		{"eliminada", User{Status: StatusDeleted, EmailVerifiedAt: &verificado}, false},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			puede, motivo := c.user.CanLogIn()
			if puede != c.puede {
				t.Fatalf("CanLogIn() = %v (%q), se esperaba %v", puede, motivo, c.puede)
			}
			if !puede && motivo == "" {
				t.Error("un rechazo debe traer motivo, para poder auditarlo")
			}
		})
	}
}

// El registro público solo crea estudiantes: los profesores se crean por
// administración (RF-01).
func TestNewStudentSiempreEsEstudiante(t *testing.T) {
	now := time.Now().UTC()
	u := NewStudent(uuid.New(), RegisterInput{
		Email:    "  ANA@MOOC.LOCAL ",
		FullName: "  Ana Pérez  ",
	}, "hash", now)

	if u.Role != RoleStudent {
		t.Errorf("Role = %q, se esperaba %q", u.Role, RoleStudent)
	}
	if u.Status != StatusActive {
		t.Errorf("Status = %q, se esperaba %q", u.Status, StatusActive)
	}
	if u.Email != "ana@mooc.local" {
		t.Errorf("el correo no se normalizó: %q", u.Email)
	}
	if u.FullName != "Ana Pérez" {
		t.Errorf("el nombre no se recortó: %q", u.FullName)
	}
	if u.IsVerified() {
		t.Error("un usuario recién registrado no puede estar verificado")
	}
}

func TestOneTimeTokenUsable(t *testing.T) {
	ahora := time.Now().UTC()
	consumido := ahora.Add(-time.Minute)

	casos := []struct {
		nombre string
		token  OneTimeToken
		usable bool
	}{
		{"vigente y sin consumir", OneTimeToken{ExpiresAt: ahora.Add(time.Hour)}, true},
		{"expirado", OneTimeToken{ExpiresAt: ahora.Add(-time.Hour)}, false},
		{"ya consumido", OneTimeToken{ExpiresAt: ahora.Add(time.Hour), ConsumedAt: &consumido}, false},
		{"expirado y consumido", OneTimeToken{ExpiresAt: ahora.Add(-time.Hour), ConsumedAt: &consumido}, false},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := c.token.Usable(ahora); got != c.usable {
				t.Fatalf("Usable() = %v, se esperaba %v", got, c.usable)
			}
		})
	}
}
