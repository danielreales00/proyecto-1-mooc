// Package admin cubre la gestión de usuarios, sesiones, auditoría y cola
// (RF-02).
//
// Dominio puro: las reglas que deciden si una operación administrativa es
// legítima, sin tocar la base ni el HTTP.
package admin

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("admin: no encontrado")
	// ErrUltimoAdministrador protege al último administrador activo (RF-02).
	// Sin esta regla, un descuido deja el sistema sin nadie que lo administre
	// y sin forma de recuperarlo por la API.
	ErrUltimoAdministrador = errors.New("admin: no se puede dejar el sistema sin administrador activo")
	ErrCorreoEnUso         = errors.New("admin: el correo ya está registrado")
)

const (
	RolAdmin      = "admin"
	RolProfesor   = "teacher"
	RolEstudiante = "student"
)

const (
	EstadoActivo     = "active"
	EstadoSuspendido = "suspended"
	EstadoEliminado  = "deleted"
)

type Usuario struct {
	ID              uuid.UUID
	Email           string
	FullName        string
	Role            string
	Status          string
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Sesion struct {
	ID         uuid.UUID
	IP         *string
	UserAgent  *string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
}

func (s Sesion) Activa() bool {
	return s.RevokedAt == nil && time.Now().Before(s.ExpiresAt)
}

type EventoAuditoria struct {
	ID         int64
	OccurredAt time.Time
	ActorID    *uuid.UUID
	ActorRole  string
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	IP         *string
	TraceID    *string
	Metadata   map[string]any
}

type Trabajo struct {
	ID          int64
	JobKey      string
	Type        string
	Queue       string
	Status      string
	Attempt     int
	LastError   *string
	HeartbeatAt *time.Time
	CreatedAt   time.Time
	FinishedAt  *time.Time
}

// ---------------------------------------------------------------- reglas ---

type ValidationError struct {
	Code   string
	Field  string
	Detail string
}

type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	p := make([]string, 0, len(v))
	for _, e := range v {
		p = append(p, e.Field+": "+e.Detail)
	}
	return strings.Join(p, "; ")
}

func (v ValidationErrors) Empty() bool { return len(v) == 0 }

// NuevoUsuario son los datos para crear un profesor o administrador. El
// registro público solo crea estudiantes; esta es la única vía para los demás
// roles (RF-01).
type NuevoUsuario struct {
	Email    string
	FullName string
	Role     string
}

func (in NuevoUsuario) Validate() ValidationErrors {
	var errs ValidationErrors

	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" {
		errs = append(errs, ValidationError{"email.required", "email", "El correo es obligatorio."})
	} else if _, err := mail.ParseAddress(email); err != nil {
		errs = append(errs, ValidationError{"email.invalid", "email", "El correo no tiene un formato válido."})
	}

	if strings.TrimSpace(in.FullName) == "" {
		errs = append(errs, ValidationError{"full_name.required", "full_name", "El nombre es obligatorio."})
	}

	// Un estudiante se crea por registro público, no por administración: dejar
	// crearlo aquí abriría una vía para saltarse la verificación de correo.
	if in.Role != RolAdmin && in.Role != RolProfesor {
		errs = append(errs, ValidationError{"role.invalid", "role",
			"El rol debe ser admin o teacher. Los estudiantes se crean por registro público."})
	}

	return errs
}

// CambioDeUsuario es una modificación administrativa. Los punteros permiten
// distinguir «no lo toques» de «ponlo a este valor».
type CambioDeUsuario struct {
	Role   *string
	Status *string
}

func (c CambioDeUsuario) Validate() ValidationErrors {
	var errs ValidationErrors
	if c.Role != nil && *c.Role != RolAdmin && *c.Role != RolProfesor && *c.Role != RolEstudiante {
		errs = append(errs, ValidationError{"role.invalid", "role", "Rol desconocido."})
	}
	if c.Status != nil && *c.Status != EstadoActivo && *c.Status != EstadoSuspendido && *c.Status != EstadoEliminado {
		errs = append(errs, ValidationError{"status.invalid", "status", "Estado desconocido."})
	}
	if c.Role == nil && c.Status == nil {
		errs = append(errs, ValidationError{"change.empty", "", "No se indicó ningún cambio."})
	}
	return errs
}

// DejaSinAdministrador dice si aplicar el cambio a este usuario dejaría el
// sistema sin ningún administrador activo.
//
// Es función pura para poder probar la regla exhaustivamente sin base de
// datos: recibe el estado actual del usuario y cuántos OTROS administradores
// activos hay.
func DejaSinAdministrador(actual Usuario, c CambioDeUsuario, otrosAdminsActivos int) bool {
	eraAdminActivo := actual.Role == RolAdmin && actual.Status == EstadoActivo
	if !eraAdminActivo || otrosAdminsActivos > 0 {
		return false
	}

	// Es el último administrador activo: solo se puede rechazar si el cambio
	// le quita el rol o lo desactiva.
	seguiraSiendoAdmin := actual.Role
	if c.Role != nil {
		seguiraSiendoAdmin = *c.Role
	}
	seguiraActivo := actual.Status
	if c.Status != nil {
		seguiraActivo = *c.Status
	}
	return seguiraSiendoAdmin != RolAdmin || seguiraActivo != EstadoActivo
}
