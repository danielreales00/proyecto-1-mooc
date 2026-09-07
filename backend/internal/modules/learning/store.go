package learning

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"mooc/backend/internal/platform/dbx"
)

var (
	ErrNotFound    = errors.New("learning: no encontrado")
	ErrProhibido   = errors.New("learning: sin permiso")
	ErrNoPublicado = errors.New("learning: el curso no está publicado")
)

// CatalogCourse es la ficha pública. Sin contenido privado ni binarios.
type CatalogCourse struct {
	ID            uuid.UUID
	Slug          string
	Title         string
	Summary       string
	Category      string
	Language      string
	VersionNumber int
	EnrolledCount int
	CreatedAt     time.Time
	Outline       []OutlineModule
}

// Cursor es el punto desde el que sigue una página. Lleva fecha e
// identificador porque ordenar solo por fecha no es determinista: dos cursos
// publicados en el mismo instante se perderían al paginar.
type Cursor struct {
	Fecha time.Time
	ID    string
}

type OutlineModule struct {
	Title string
	Units []string
}

// ResourceRef es lo mínimo del recurso que el progreso necesita: tipo,
// obligatoriedad y duración del binario si lo tiene.
type ResourceRef struct {
	StableID  uuid.UUID
	Title     string
	Type      string
	Visible   bool
	Required  bool
	Duration  *float64
	ContentMD *string
	AssetID   *uuid.UUID
}

type Store interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, tx dbx.DB) error) error
	DB() dbx.DB

	SearchCatalog(ctx context.Context, db dbx.DB, q, categoria, idioma string,
		limit int, desde *Cursor) ([]CatalogCourse, error)
	CatalogBySlug(ctx context.Context, db dbx.DB, slug string) (CatalogCourse, error)

	// CurrentPublishedVersion devuelve la versión vigente publicada del curso.
	CurrentPublishedVersion(ctx context.Context, db dbx.DB, courseID uuid.UUID) (uuid.UUID, int, float64, float64, error)

	UpsertEnrollment(ctx context.Context, db dbx.DB, e Enrollment) (Enrollment, error)
	EnrollmentByID(ctx context.Context, db dbx.DB, id uuid.UUID) (Enrollment, error)
	EnrollmentsByUser(ctx context.Context, db dbx.DB, userID uuid.UUID) ([]Enrollment, error)
	UpdateEnrollment(ctx context.Context, db dbx.DB, e Enrollment) error

	// ResourcesOfVersion devuelve los recursos visibles de la versión vigente.
	ResourcesOfVersion(ctx context.Context, db dbx.DB, versionID uuid.UUID) ([]ResourceRef, error)

	ProgressOf(ctx context.Context, db dbx.DB, enrollmentID uuid.UUID) (map[uuid.UUID]ResourceProgress, error)
	ProgressOne(ctx context.Context, db dbx.DB, enrollmentID, stableID uuid.UUID) (ResourceProgress, error)
	UpsertProgress(ctx context.Context, db dbx.DB, p ResourceProgress) error
	InsertEvent(ctx context.Context, db dbx.DB, enrollmentID, stableID uuid.UUID,
		kind string, pos *float64, clientTS any, aceptada bool, motivo string) error

	// MinQuizScoreCumplida dice si el estudiante alcanzó la nota mínima en
	// todos los quizzes obligatorios de la versión.
	MinQuizScoreCumplida(ctx context.Context, db dbx.DB, enrollmentID, versionID uuid.UUID, minima float64) (bool, error)
}

// Publisher es el puerto de la cola: al aprobar se encola la insignia.
type Publisher interface {
	Publish(ctx context.Context, key, tipo, cola string, payload map[string]any, traceID string) error
}
