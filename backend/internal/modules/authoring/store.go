package authoring

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"mooc/backend/internal/platform/dbx"
)

var (
	ErrNotFound  = errors.New("authoring: no encontrado")
	ErrInmutable = errors.New("authoring: la versión está publicada y es inmutable")
	ErrConflicto = errors.New("authoring: estado incompatible")
)

// Store es el puerto de persistencia. Lo implementa adapters/postgres.
type Store interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, tx dbx.DB) error) error
	DB() dbx.DB

	InsertCourse(ctx context.Context, db dbx.DB, c Course) error
	UpdateCourse(ctx context.Context, db dbx.DB, c Course) error
	CourseByID(ctx context.Context, db dbx.DB, id uuid.UUID) (Course, error)
	CourseBySlug(ctx context.Context, db dbx.DB, slug string) (Course, error)
	CoursesByOwner(ctx context.Context, db dbx.DB, owner *uuid.UUID, limit int) ([]Course, error)
	SetCurrentVersion(ctx context.Context, db dbx.DB, courseID, versionID uuid.UUID, status string) error

	InsertVersion(ctx context.Context, db dbx.DB, v CourseVersion) error
	VersionByNumber(ctx context.Context, db dbx.DB, courseID uuid.UUID, n int) (CourseVersion, error)
	VersionByID(ctx context.Context, db dbx.DB, id uuid.UUID) (CourseVersion, error)
	VersionsOf(ctx context.Context, db dbx.DB, courseID uuid.UUID) ([]CourseVersion, error)
	SetVersionStatus(ctx context.Context, db dbx.DB, versionID uuid.UUID, status string, by *uuid.UUID) error

	// Tree carga módulos, unidades y recursos, con el estado del binario y del
	// quiz ya resuelto para poder validar la publicación de una sola vez.
	Tree(ctx context.Context, db dbx.DB, versionID uuid.UUID) ([]Module, error)

	InsertModule(ctx context.Context, db dbx.DB, m Module) error
	InsertUnit(ctx context.Context, db dbx.DB, u Unit) error
	InsertResource(ctx context.Context, db dbx.DB, r Resource) error
	UpdateResource(ctx context.Context, db dbx.DB, r Resource) error
	ResourceByID(ctx context.Context, db dbx.DB, id uuid.UUID) (Resource, error)
	ModuleByID(ctx context.Context, db dbx.DB, id uuid.UUID) (Module, error)
	UnitByID(ctx context.Context, db dbx.DB, id uuid.UUID) (Unit, error)
	NextPosition(ctx context.Context, db dbx.DB, tabla, columnaPadre string, padre uuid.UUID) (int, error)

	InsertRevision(ctx context.Context, db dbx.DB, resourceID, authorID uuid.UUID, contenido string) error

	// CloneVersion copia el árbol completo conservando los stable_id: es lo que
	// hace que el progreso sobreviva a la versión nueva (ADR-0007).
	CloneVersion(ctx context.Context, db dbx.DB, origen, destino uuid.UUID) error
}
