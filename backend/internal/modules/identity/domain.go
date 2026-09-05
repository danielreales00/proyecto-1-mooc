// Package identity cubre registro, verificación de correo, sesiones y
// contraseñas (RF-01).
//
// Este archivo es dominio puro: entidades y reglas. No importa net/http, ni
// pgx, ni ningún adaptador (ADR-0001, regla 2).
package identity

import (
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Roles globales del enunciado §2.
const (
	RoleAdmin   = "admin"
	RoleTeacher = "teacher"
	RoleStudent = "student"
)

const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
	StatusDeleted   = "deleted"
)

const (
	PurposeEmailVerify   = "email_verify"
	PurposePasswordReset = "password_reset"
)

const (
	minPasswordLen = 10
	maxPasswordLen = 128
	maxNameLen     = 120
)

type User struct {
	ID              uuid.UUID
	Email           string
	EmailVerifiedAt *time.Time
	PasswordHash    string
	FullName        string
	Role            string
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (u User) IsVerified() bool { return u.EmailVerifiedAt != nil }
func (u User) IsActive() bool   { return u.Status == StatusActive }

// CanLogIn concentra las condiciones para iniciar sesión. Devuelve un motivo
// interno; la capa HTTP decide qué revelar al cliente.
func (u User) CanLogIn() (bool, string) {
	switch {
	case !u.IsActive():
		return false, "cuenta " + u.Status
	case !u.IsVerified():
		return false, "correo sin verificar"
	default:
		return true, ""
	}
}

type OneTimeToken struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Purpose     string
	TokenSHA256 []byte
	ExpiresAt   time.Time
	ConsumedAt  *time.Time
}

func (t OneTimeToken) Usable(now time.Time) bool {
	return t.ConsumedAt == nil && now.Before(t.ExpiresAt)
}

// Session es el registro de una sesión. El token opaco nunca se guarda: solo
// su SHA-256 (ADR-0006).
type Session struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Role        string
	TokenSHA256 []byte
	IP          string
	UserAgent   string
	CreatedAt   time.Time
	LastSeenAt  time.Time
	ExpiresAt   time.Time
}

// ValidationError es un incumplimiento concreto. El dominio los acumula para
// que la API pueda devolver la lista completa en un solo 422.
type ValidationError struct {
	Code   string
	Field  string
	Detail string
}

type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	parts := make([]string, 0, len(v))
	for _, e := range v {
		parts = append(parts, e.Field+": "+e.Detail)
	}
	return strings.Join(parts, "; ")
}

func (v ValidationErrors) Empty() bool { return len(v) == 0 }

// NormalizeEmail deja el correo en la forma con la que se compara y se guarda.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// RegisterInput son los datos de un registro público de estudiante.
type RegisterInput struct {
	Email    string
	Password string
	FullName string
}

// Validate acumula todos los problemas en lugar de parar en el primero.
func (in RegisterInput) Validate() ValidationErrors {
	var errs ValidationErrors

	email := NormalizeEmail(in.Email)
	switch {
	case email == "":
		errs = append(errs, ValidationError{"email.required", "email", "El correo es obligatorio."})
	case len(email) > 254:
		errs = append(errs, ValidationError{"email.too_long", "email", "El correo supera 254 caracteres."})
	default:
		if _, err := mail.ParseAddress(email); err != nil {
			errs = append(errs, ValidationError{"email.invalid", "email", "El correo no tiene un formato válido."})
		}
	}

	switch n := utf8.RuneCountInString(in.Password); {
	case in.Password == "":
		errs = append(errs, ValidationError{"password.required", "password", "La contraseña es obligatoria."})
	case n < minPasswordLen:
		errs = append(errs, ValidationError{"password.too_short", "password",
			fmt.Sprintf("La contraseña debe tener al menos %d caracteres.", minPasswordLen)})
	case n > maxPasswordLen:
		errs = append(errs, ValidationError{"password.too_long", "password",
			fmt.Sprintf("La contraseña no puede superar %d caracteres.", maxPasswordLen)})
	}

	name := strings.TrimSpace(in.FullName)
	switch {
	case name == "":
		errs = append(errs, ValidationError{"full_name.required", "full_name", "El nombre es obligatorio."})
	case utf8.RuneCountInString(name) > maxNameLen:
		errs = append(errs, ValidationError{"full_name.too_long", "full_name",
			fmt.Sprintf("El nombre no puede superar %d caracteres.", maxNameLen)})
	}

	return errs
}

// NewStudent construye el usuario a registrar. El registro público solo crea
// estudiantes: los profesores se crean por administración (RF-01).
func NewStudent(id uuid.UUID, in RegisterInput, passwordHash string, now time.Time) User {
	return User{
		ID:           id,
		Email:        NormalizeEmail(in.Email),
		PasswordHash: passwordHash,
		FullName:     strings.TrimSpace(in.FullName),
		Role:         RoleStudent,
		Status:       StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
