// Package learning cubre catálogo, inscripción y progreso (RF-09, RF-10).
//
// Van juntos porque comparten el agregado inscripción: separar el cálculo del
// progreso de la inscripción obligaría a que cada uno leyera las tablas del
// otro, que es justo lo que el ADR-0001 prohíbe.
package learning

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	StatusActive    = "active"
	StatusWithdrawn = "withdrawn"
)

const (
	StateInProgress = "in_progress"
	StateCompleted  = "completed"
	StateApproved   = "approved"
)

const (
	ProgressNotStarted = "not_started"
	ProgressInProgress = "in_progress"
	ProgressCompleted  = "completed"
)

const (
	EvidenceOpen      = "open"
	EvidenceHeartbeat = "heartbeat"
	EvidenceClose     = "close"
)

// Umbrales de validación de evidencias (ADR-0012). Son configurables porque
// van a ajustarse al grabar la demostración.
type Umbrales struct {
	CadenciaMinima     time.Duration // entre heartbeats del mismo recurso
	FactorAvanceMaximo float64       // avance de posición vs. tiempo real
	PermanenciaTexto   time.Duration
	PermanenciaDoc     time.Duration
	FraccionMedia      float64 // del total del video/audio
}

func UmbralesPorDefecto() Umbrales {
	return Umbrales{
		CadenciaMinima:     10 * time.Second,
		FactorAvanceMaximo: 2.0,
		PermanenciaTexto:   15 * time.Second,
		PermanenciaDoc:     30 * time.Second,
		FraccionMedia:      0.9,
	}
}

type Enrollment struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	CourseID          uuid.UUID
	Status            string
	State             string
	ProgressPct       float64
	ApprovedVersionID *uuid.UUID
	EnrolledAt        time.Time
	WithdrawnAt       *time.Time
	CompletedAt       *time.Time
	ApprovedAt        *time.Time
}

func (e Enrollment) Activa() bool { return e.Status == StatusActive }

type ResourceProgress struct {
	EnrollmentID        uuid.UUID
	ResourceStableID    uuid.UUID
	State               string
	DwellMsTotal        int64
	LastPositionSeconds float64
	MaxPositionSeconds  float64
	OpenedAt            *time.Time
	CompletedAt         *time.Time
	UpdatedAt           time.Time
}

// Evidence es lo que el cliente reporta: hechos, no conclusiones.
type Evidence struct {
	ResourceStableID uuid.UUID
	Kind             string
	PositionSeconds  *float64
	ClientTS         *time.Time
}

// Motivos de rechazo de una evidencia. Se registran para poder auditarlos.
const (
	RejectCadence         = "cadence"
	RejectPositionJump    = "position_jump"
	RejectPositionRange   = "position_out_of_range"
	RejectMissingOpen     = "missing_open"
	RejectNotEnrolled     = "not_enrolled"
	RejectUnknownResource = "unknown_resource"
)

// Veredicto es el resultado de validar una evidencia contra el estado previo.
type Veredicto struct {
	Aceptada bool
	Motivo   string
	// DwellMs es cuánto tiempo de permanencia añade esta evidencia.
	DwellMs int64
}

// ValidarEvidencia decide si una evidencia cuenta. Es función pura: recibe el
// estado previo y el reloj del servidor, y no toca la base. Por eso se puede
// probar exhaustivamente sin contenedores (ADR-0012).
//
// El reloj SIEMPRE lo pone el servidor: client_ts se registra para diagnóstico
// pero no interviene en ningún cálculo.
func ValidarEvidencia(ev Evidence, previo ResourceProgress, duracion *float64,
	ahora time.Time, u Umbrales) Veredicto {

	if ev.Kind == EvidenceOpen {
		return Veredicto{Aceptada: true}
	}

	// Un heartbeat sin apertura previa no cuenta: ordena la secuencia y evita
	// que alguien "estudie" un recurso que nunca abrió.
	if previo.OpenedAt == nil {
		return Veredicto{Motivo: RejectMissingOpen}
	}

	transcurrido := ahora.Sub(previo.UpdatedAt)
	if ev.Kind == EvidenceHeartbeat && transcurrido < u.CadenciaMinima {
		// Inundar de heartbeats no simula permanencia.
		return Veredicto{Motivo: RejectCadence}
	}

	if ev.PositionSeconds != nil {
		pos := *ev.PositionSeconds
		if pos < 0 {
			return Veredicto{Motivo: RejectPositionRange}
		}
		if duracion != nil && pos > *duracion+2 {
			return Veredicto{Motivo: RejectPositionRange}
		}
		// Saltar al final del video no es haberlo visto.
		avance := pos - previo.LastPositionSeconds
		maximo := transcurrido.Seconds()*u.FactorAvanceMaximo + 2
		if avance > maximo {
			return Veredicto{Motivo: RejectPositionJump}
		}
	}

	// La permanencia se acumula: un video visto en tres tandas cuenta entero.
	dwell := transcurrido
	if dwell > 2*u.CadenciaMinima {
		dwell = 2 * u.CadenciaMinima // una pausa larga no se regala
	}
	return Veredicto{Aceptada: true, DwellMs: dwell.Milliseconds()}
}

// CriterioCompletado dice si un recurso se puede dar por completado con el
// estado actual (ADR-0012).
func CriterioCompletado(tipo string, p ResourceProgress, duracion *float64, u Umbrales) bool {
	switch tipo {
	case "video", "audio":
		if duracion == nil || *duracion <= 0 {
			return false
		}
		permanencia := float64(p.DwellMsTotal) / 1000
		return permanencia >= *duracion*u.FraccionMedia &&
			p.MaxPositionSeconds >= *duracion*u.FraccionMedia
	case "pdf", "slides":
		return p.OpenedAt != nil && p.DwellMsTotal >= u.PermanenciaDoc.Milliseconds()
	case "rich_text":
		return p.OpenedAt != nil && p.DwellMsTotal >= u.PermanenciaTexto.Milliseconds()
	case "quiz":
		return false // lo marca el módulo de evaluación al aprobar el intento
	default:
		// imagen, enlace, descargable, iframe: basta con abrirlo
		return p.OpenedAt != nil
	}
}

// CalcularPorcentaje devuelve el avance sobre los recursos obligatorios y
// visibles. Los opcionales no cuentan (ADR-0012).
func CalcularPorcentaje(obligatorios int, completados int) float64 {
	if obligatorios == 0 {
		return 0
	}
	pct := float64(completados) / float64(obligatorios) * 100
	if pct > 100 {
		pct = 100
	}
	return pct
}

// EvaluarEstado decide el estado de la inscripción. Aprobar es un hecho
// histórico: una versión nueva del curso no revoca una aprobación previa
// (ADR-0007).
func EvaluarEstado(actual string, pct float64, criterioPct float64,
	notaMinimaCumplida bool) string {

	if actual == StateApproved {
		return StateApproved
	}
	if pct >= criterioPct && notaMinimaCumplida {
		return StateApproved
	}
	if pct >= 100 {
		return StateCompleted
	}
	return StateInProgress
}

// NormalizarBusqueda limpia el término del catálogo.
func NormalizarBusqueda(q string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(q)), " ")
}
