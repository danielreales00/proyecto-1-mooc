// Package badges emite y verifica insignias (RF-09, CA-07).
package badges

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"mooc/backend/internal/platform/dbx"
	"mooc/backend/internal/platform/ids"
)

var ErrNotFound = errors.New("badges: no encontrado")

type Badge struct {
	ID               uuid.UUID
	EnrollmentID     uuid.UUID
	UserID           uuid.UUID
	CourseID         uuid.UUID
	CourseVersionID  uuid.UUID
	PublicCode       string
	ImageKey         string
	IssuedAt         time.Time
	RevokedAt        *time.Time
	RevocationReason *string

	// Datos para la verificación pública. NUNCA incluyen el correo (CA-07).
	StudentName   string
	CourseTitle   string
	VersionNumber int
}

func (b Badge) Estado() string {
	if b.RevokedAt != nil {
		return "revoked"
	}
	return "valid"
}

type Store interface {
	DB() dbx.DB
	// IssueForEnrollment crea la insignia si no existe. La unicidad la
	// garantiza la restricción UNIQUE(enrollment_id): es la segunda barrera
	// tras el reclamo idempotente del trabajo (ADR-0008).
	IssueForEnrollment(ctx context.Context, db dbx.DB, b Badge) (Badge, bool, error)
	ByPublicCode(ctx context.Context, db dbx.DB, codigo string) (Badge, error)
	ByID(ctx context.Context, db dbx.DB, id uuid.UUID) (Badge, error)
	ByUser(ctx context.Context, db dbx.DB, userID uuid.UUID) ([]Badge, error)
	Revoke(ctx context.Context, db dbx.DB, id uuid.UUID, by uuid.UUID, motivo string) (Badge, error)
	// EnrollmentSnapshot trae lo necesario para emitir: usuario, curso, versión.
	EnrollmentSnapshot(ctx context.Context, db dbx.DB, enrollmentID uuid.UUID) (uuid.UUID, uuid.UUID, uuid.UUID, error)
}

// Almacen es lo único que badges necesita del almacenamiento de objetos: la
// imagen de la insignia no puede vivir en la base relacional (§7 del enunciado).
type Almacen interface {
	Subir(ctx context.Context, bucket, key, contentType string, datos []byte) error
}

type Service struct {
	store Store
	almacen Almacen
	bucket  string
	// baseImagenes es el origen público del almacén. El bucket de insignias
	// tiene lectura anónima (ADR-0005), así que la URL no se firma: una
	// insignia que caduca a los quince minutos no sirve para acreditar nada.
	baseImagenes string
	log          *slog.Logger
}

func NewService(store Store, almacen Almacen, bucket, baseImagenes string, log *slog.Logger) *Service {
	return &Service{store: store, almacen: almacen, bucket: bucket,
		baseImagenes: strings.TrimSuffix(baseImagenes, "/"), log: log}
}

// URLImagen devuelve la dirección pública de la imagen, o cadena vacía si no
// hay almacén configurado.
func (s *Service) URLImagen(b Badge) string {
	if s.baseImagenes == "" || b.ImageKey == "" {
		return ""
	}
	return s.baseImagenes + "/" + s.bucket + "/" + b.ImageKey
}

// publicarImagen sube la imagen al bucket público. Se llama también cuando la
// insignia ya existía: así un reintento repara una imagen que faltara, y como
// la clave y el contenido son deterministas, escribir dos veces no cambia nada.
func (s *Service) publicarImagen(ctx context.Context, b Badge) error {
	if s.almacen == nil {
		return nil
	}
	return s.almacen.Subir(ctx, s.bucket, b.ImageKey, TipoImagen, Imagen(b))
}

// Handle atiende el trabajo badge.issue. Es idempotente por partida doble: el
// runner reclama la ejecución antes de entrar aquí, y la restricción única de
// la base impide una segunda fila aunque el reclamo fallara.
func (s *Service) Handle(ctx context.Context, data map[string]any) error {
	raw, _ := data["enrollment_id"].(string)
	enrollmentID, err := uuid.Parse(raw)
	if err != nil {
		return fmt.Errorf("badge.issue con enrollment_id inválido: %q", raw)
	}

	userID, courseID, versionID, err := s.store.EnrollmentSnapshot(ctx, s.store.DB(), enrollmentID)
	if err != nil {
		return fmt.Errorf("leer la inscripción %s: %w", enrollmentID, err)
	}

	codigo, err := ids.PublicCode()
	if err != nil {
		return err
	}
	b := Badge{
		ID: ids.New(), EnrollmentID: enrollmentID, UserID: userID,
		CourseID: courseID, CourseVersionID: versionID,
		PublicCode: codigo,
		// Determinista: reintentar escribe en el mismo sitio con el mismo
		// contenido, así que no hay forma de dejar dos imágenes.
		ImageKey: ClaveImagen(codigo),
		IssuedAt: time.Now().UTC(),
	}

	emitida, nueva, err := s.store.IssueForEnrollment(ctx, s.store.DB(), b)
	if err != nil {
		return err
	}

	// La inserción no devuelve el nombre del estudiante ni el título del curso;
	// hay que releer para poder dibujarlos.
	completa, err := s.store.ByID(ctx, s.store.DB(), emitida.ID)
	if err != nil {
		return fmt.Errorf("releer la insignia %s: %w", emitida.ID, err)
	}
	if err := s.publicarImagen(ctx, completa); err != nil {
		return fmt.Errorf("publicar la imagen de la insignia %s: %w", emitida.ID, err)
	}

	if !nueva {
		s.log.Info("la insignia ya existía; no se emite otra",
			"enrollment_id", enrollmentID, "badge_id", emitida.ID)
		return nil
	}
	s.log.Info("insignia emitida",
		"enrollment_id", enrollmentID, "badge_id", emitida.ID,
		"public_code", emitida.PublicCode, "image_key", completa.ImageKey)
	return nil
}

func (s *Service) Verify(ctx context.Context, codigo string) (Badge, error) {
	return s.store.ByPublicCode(ctx, s.store.DB(), codigo)
}

func (s *Service) MyBadges(ctx context.Context, userID uuid.UUID) ([]Badge, error) {
	return s.store.ByUser(ctx, s.store.DB(), userID)
}

func (s *Service) Revoke(ctx context.Context, id, by uuid.UUID, motivo string) (Badge, error) {
	return s.store.Revoke(ctx, s.store.DB(), id, by, motivo)
}
