package authoring

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"mooc/backend/internal/modules/audit"
	"mooc/backend/internal/platform/dbx"
	"mooc/backend/internal/platform/ids"
	"mooc/backend/internal/platform/markdown"
)

type Service struct {
	store Store
	audit *audit.Recorder
	log   *slog.Logger
}

func NewService(store Store, rec *audit.Recorder, log *slog.Logger) *Service {
	return &Service{store: store, audit: rec, log: log}
}

type Actor struct {
	UserID uuid.UUID
	Role   string
	IP     string
	Agent  string
	Trace  string
}

func (a Actor) esAdmin() bool { return a.Role == "admin" }

// puedeEditar aplica el control por propiedad: un profesor solo toca sus
// cursos; un administrador, cualquiera (CA-06).
func (s *Service) puedeEditar(c Course, a Actor) bool {
	return a.esAdmin() || c.OwnerID == a.UserID
}

// CreateCourse crea el curso y su versión 1 en la misma transacción.
func (s *Service) CreateCourse(ctx context.Context, in CourseInput, a Actor) (Course, CourseVersion, error) {
	if errs := in.Validate(); !errs.Empty() {
		return Course{}, CourseVersion{}, errs
	}

	now := time.Now().UTC()
	curso := Course{
		ID:        ids.New(),
		Slug:      Slugify(in.Title),
		Title:     strings.TrimSpace(in.Title),
		Summary:   strings.TrimSpace(in.Summary),
		Category:  strings.TrimSpace(in.Category),
		Language:  strings.TrimSpace(in.Language),
		OwnerID:   a.UserID,
		Status:    StatusDraft,
		CreatedAt: now, UpdatedAt: now,
	}
	// El slug debe ser único; si choca, se desambigua con el prefijo del id.
	if _, err := s.store.CourseBySlug(ctx, s.store.DB(), curso.Slug); err == nil {
		curso.Slug = fmt.Sprintf("%s-%s", curso.Slug, curso.ID.String()[:8])
	} else if !errors.Is(err, ErrNotFound) {
		return Course{}, CourseVersion{}, err
	}

	version := CourseVersion{
		ID:               ids.New(),
		CourseID:         curso.ID,
		VersionNumber:    1,
		Status:           StatusDraft,
		ApprovalCriteria: ApprovalCriteria{RequiredCompletionPct: 100, MinQuizScore: 70},
		CreatedAt:        now,
	}

	err := s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.InsertCourse(ctx, tx, curso); err != nil {
			return err
		}
		if err := s.store.InsertVersion(ctx, tx, version); err != nil {
			return err
		}
		if err := s.store.SetCurrentVersion(ctx, tx, curso.ID, version.ID, StatusDraft); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: &a.UserID, ActorRole: a.Role, Action: "course.created",
			EntityType: "course", EntityID: &curso.ID,
			IP: a.IP, UserAgent: a.Agent, TraceID: a.Trace,
			Metadata: map[string]any{"title": curso.Title, "slug": curso.Slug},
		})
	})
	if err != nil {
		return Course{}, CourseVersion{}, err
	}
	curso.CurrentVersionID = &version.ID
	return curso, version, nil
}

func (s *Service) ListCourses(ctx context.Context, a Actor, limit int) ([]Course, error) {
	var owner *uuid.UUID
	if !a.esAdmin() {
		owner = &a.UserID
	}
	return s.store.CoursesByOwner(ctx, s.store.DB(), owner, limit)
}

func (s *Service) GetCourse(ctx context.Context, id uuid.UUID, a Actor) (Course, error) {
	c, err := s.store.CourseByID(ctx, s.store.DB(), id)
	if err != nil {
		return Course{}, err
	}
	if !s.puedeEditar(c, a) {
		return Course{}, ErrProhibido
	}
	return c, nil
}

var ErrProhibido = errors.New("authoring: sin permiso sobre este curso")

func (s *Service) UpdateCourse(ctx context.Context, id uuid.UUID, in CourseInput, a Actor) (Course, error) {
	if errs := in.Validate(); !errs.Empty() {
		return Course{}, errs
	}
	c, err := s.GetCourse(ctx, id, a)
	if err != nil {
		return Course{}, err
	}
	c.Title = strings.TrimSpace(in.Title)
	c.Summary = strings.TrimSpace(in.Summary)
	c.Category = strings.TrimSpace(in.Category)
	c.Language = strings.TrimSpace(in.Language)
	c.UpdatedAt = time.Now().UTC()

	if err := s.store.UpdateCourse(ctx, s.store.DB(), c); err != nil {
		return Course{}, err
	}
	return c, nil
}

// GetVersionTree carga la versión con su árbol completo.
func (s *Service) GetVersionTree(ctx context.Context, courseID uuid.UUID, n int, a Actor) (Course, CourseVersion, error) {
	c, err := s.GetCourse(ctx, courseID, a)
	if err != nil {
		return Course{}, CourseVersion{}, err
	}
	v, err := s.store.VersionByNumber(ctx, s.store.DB(), courseID, n)
	if err != nil {
		return Course{}, CourseVersion{}, err
	}
	v.Modules, err = s.store.Tree(ctx, s.store.DB(), v.ID)
	if err != nil {
		return Course{}, CourseVersion{}, err
	}
	return c, v, nil
}

func (s *Service) ListVersions(ctx context.Context, courseID uuid.UUID, a Actor) ([]CourseVersion, error) {
	if _, err := s.GetCourse(ctx, courseID, a); err != nil {
		return nil, err
	}
	return s.store.VersionsOf(ctx, s.store.DB(), courseID)
}

// Validate comprueba las reglas de publicación sin publicar.
func (s *Service) Validate(ctx context.Context, courseID uuid.UUID, n int, a Actor) (ValidationErrors, error) {
	c, v, err := s.GetVersionTree(ctx, courseID, n, a)
	if err != nil {
		return nil, err
	}
	return ValidarPublicacion(c, v), nil
}

// Publish publica la versión si supera TODAS las reglas.
func (s *Service) Publish(ctx context.Context, courseID uuid.UUID, n int, a Actor) (CourseVersion, error) {
	c, v, err := s.GetVersionTree(ctx, courseID, n, a)
	if err != nil {
		return CourseVersion{}, err
	}
	if v.Status == StatusPublished {
		return v, nil // idempotente: ya está publicada
	}
	if errs := ValidarPublicacion(c, v); !errs.Empty() {
		return CourseVersion{}, errs
	}

	err = s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.SetVersionStatus(ctx, tx, v.ID, StatusPublished, &a.UserID); err != nil {
			return err
		}
		if err := s.store.SetCurrentVersion(ctx, tx, c.ID, v.ID, StatusPublished); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: &a.UserID, ActorRole: a.Role, Action: "course.published",
			EntityType: "course_version", EntityID: &v.ID,
			IP: a.IP, UserAgent: a.Agent, TraceID: a.Trace,
			Metadata: map[string]any{"course_id": c.ID.String(), "version": v.VersionNumber},
		})
	})
	if err != nil {
		return CourseVersion{}, err
	}
	v.Status = StatusPublished
	ahora := time.Now().UTC()
	v.PublishedAt = &ahora
	return v, nil
}

// Unpublish devuelve la versión a estado editable. Conserva las inscripciones
// existentes y su progreso; solo bloquea las nuevas.
func (s *Service) Unpublish(ctx context.Context, courseID uuid.UUID, n int, a Actor) (CourseVersion, error) {
	c, err := s.GetCourse(ctx, courseID, a)
	if err != nil {
		return CourseVersion{}, err
	}
	v, err := s.store.VersionByNumber(ctx, s.store.DB(), courseID, n)
	if err != nil {
		return CourseVersion{}, err
	}
	if v.Status != StatusPublished {
		return CourseVersion{}, ErrConflicto
	}

	err = s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.SetVersionStatus(ctx, tx, v.ID, StatusUnpublished, &a.UserID); err != nil {
			return err
		}
		if err := s.store.SetCurrentVersion(ctx, tx, c.ID, v.ID, StatusUnpublished); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: &a.UserID, ActorRole: a.Role, Action: "course.unpublished",
			EntityType: "course_version", EntityID: &v.ID,
			IP: a.IP, UserAgent: a.Agent, TraceID: a.Trace,
		})
	})
	if err != nil {
		return CourseVersion{}, err
	}
	v.Status = StatusUnpublished
	return v, nil
}

// NewVersion clona la versión vigente a un borrador con número siguiente,
// conservando los stable_id (ADR-0007).
func (s *Service) NewVersion(ctx context.Context, courseID uuid.UUID, a Actor) (CourseVersion, error) {
	c, err := s.GetCourse(ctx, courseID, a)
	if err != nil {
		return CourseVersion{}, err
	}
	versiones, err := s.store.VersionsOf(ctx, s.store.DB(), courseID)
	if err != nil {
		return CourseVersion{}, err
	}
	if len(versiones) == 0 {
		return CourseVersion{}, ErrNotFound
	}
	ultima := versiones[0]
	for _, v := range versiones {
		if v.VersionNumber > ultima.VersionNumber {
			ultima = v
		}
		if v.Status == StatusDraft {
			return CourseVersion{}, ErrConflicto // ya hay un borrador abierto
		}
	}

	nueva := CourseVersion{
		ID:               ids.New(),
		CourseID:         courseID,
		VersionNumber:    ultima.VersionNumber + 1,
		Status:           StatusDraft,
		ApprovalCriteria: ultima.ApprovalCriteria,
		CreatedAt:        time.Now().UTC(),
	}

	err = s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.InsertVersion(ctx, tx, nueva); err != nil {
			return err
		}
		if err := s.store.CloneVersion(ctx, tx, ultima.ID, nueva.ID); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: &a.UserID, ActorRole: a.Role, Action: "course.version_created",
			EntityType: "course_version", EntityID: &nueva.ID,
			IP: a.IP, UserAgent: a.Agent, TraceID: a.Trace,
			Metadata: map[string]any{"course_id": c.ID.String(), "from_version": ultima.VersionNumber},
		})
	})
	if err != nil {
		return CourseVersion{}, err
	}
	return nueva, nil
}

// ------------------------------------------------------- jerarquía ---------

func (s *Service) guardVersionEditable(ctx context.Context, versionID uuid.UUID, a Actor) (Course, error) {
	v, err := s.store.VersionByID(ctx, s.store.DB(), versionID)
	if err != nil {
		return Course{}, err
	}
	c, err := s.store.CourseByID(ctx, s.store.DB(), v.CourseID)
	if err != nil {
		return Course{}, err
	}
	if !s.puedeEditar(c, a) {
		return Course{}, ErrProhibido
	}
	if !v.Editable() {
		return Course{}, ErrInmutable
	}
	return c, nil
}

func (s *Service) AddModule(ctx context.Context, versionID uuid.UUID, titulo string, a Actor) (Module, error) {
	if _, err := s.guardVersionEditable(ctx, versionID, a); err != nil {
		return Module{}, err
	}
	pos, err := s.store.NextPosition(ctx, s.store.DB(), "authoring.modules", "course_version_id", versionID)
	if err != nil {
		return Module{}, err
	}
	m := Module{ID: ids.New(), CourseVersionID: versionID, StableID: ids.New(), Position: pos, Title: titulo}
	if err := s.store.InsertModule(ctx, s.store.DB(), m); err != nil {
		return Module{}, err
	}
	return m, nil
}

func (s *Service) AddUnit(ctx context.Context, moduleID uuid.UUID, titulo string, a Actor) (Unit, error) {
	m, err := s.store.ModuleByID(ctx, s.store.DB(), moduleID)
	if err != nil {
		return Unit{}, err
	}
	if _, err := s.guardVersionEditable(ctx, m.CourseVersionID, a); err != nil {
		return Unit{}, err
	}
	pos, err := s.store.NextPosition(ctx, s.store.DB(), "authoring.units", "module_id", moduleID)
	if err != nil {
		return Unit{}, err
	}
	u := Unit{ID: ids.New(), ModuleID: moduleID, CourseVersionID: m.CourseVersionID,
		StableID: ids.New(), Position: pos, Title: titulo}
	if err := s.store.InsertUnit(ctx, s.store.DB(), u); err != nil {
		return Unit{}, err
	}
	return u, nil
}

func (s *Service) AddResource(ctx context.Context, unitID uuid.UUID, in ResourceInput, a Actor) (Resource, error) {
	if errs := in.Validate(); !errs.Empty() {
		return Resource{}, errs
	}
	u, err := s.store.UnitByID(ctx, s.store.DB(), unitID)
	if err != nil {
		return Resource{}, err
	}
	if _, err := s.guardVersionEditable(ctx, u.CourseVersionID, a); err != nil {
		return Resource{}, err
	}
	pos, err := s.store.NextPosition(ctx, s.store.DB(), "authoring.resources", "unit_id", unitID)
	if err != nil {
		return Resource{}, err
	}

	r := Resource{
		ID: ids.New(), UnitID: unitID, CourseVersionID: u.CourseVersionID,
		StableID: ids.New(), Position: pos,
		Title: strings.TrimSpace(in.Title), Type: in.Type,
		Visible: true, Required: true, Downloadable: false,
		AssetID: in.AssetID, ExternalURL: in.ExternalURL,
		UpdatedAt: time.Now().UTC(),
	}
	if in.Visible != nil {
		r.Visible = *in.Visible
	}
	if in.Required != nil {
		r.Required = *in.Required
	}
	if in.Downloadable != nil {
		r.Downloadable = *in.Downloadable
	}
	if in.ContentMD != nil {
		normalizado, refs, err := markdown.Canonicalizar(*in.ContentMD)
		if err != nil {
			return Resource{}, ValidationErrors{{Code: "content.invalid", Field: "content_md", Detail: err.Error()}}
		}
		r.ContentMD = &normalizado
		r.AssetRefs = refs
	}

	if err := s.store.InsertResource(ctx, s.store.DB(), r); err != nil {
		return Resource{}, err
	}
	return r, nil
}

// SaveContent normaliza y guarda el Markdown, y deja una revisión (RF-04).
func (s *Service) SaveContent(ctx context.Context, resourceID uuid.UUID, contenido string, a Actor) (Resource, error) {
	r, err := s.store.ResourceByID(ctx, s.store.DB(), resourceID)
	if err != nil {
		return Resource{}, err
	}
	if _, err := s.guardVersionEditable(ctx, r.CourseVersionID, a); err != nil {
		return Resource{}, err
	}

	normalizado, refs, err := markdown.Canonicalizar(contenido)
	if err != nil {
		return Resource{}, ValidationErrors{{Code: "content.invalid", Field: "content_md", Detail: err.Error()}}
	}
	r.ContentMD = &normalizado
	r.AssetRefs = refs
	r.UpdatedAt = time.Now().UTC()

	err = s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.UpdateResource(ctx, tx, r); err != nil {
			return err
		}
		return s.store.InsertRevision(ctx, tx, r.ID, a.UserID, normalizado)
	})
	if err != nil {
		return Resource{}, err
	}
	return r, nil
}

func (s *Service) UpdateResource(ctx context.Context, resourceID uuid.UUID, in ResourceInput, a Actor) (Resource, error) {
	if errs := in.Validate(); !errs.Empty() {
		return Resource{}, errs
	}
	r, err := s.store.ResourceByID(ctx, s.store.DB(), resourceID)
	if err != nil {
		return Resource{}, err
	}
	if _, err := s.guardVersionEditable(ctx, r.CourseVersionID, a); err != nil {
		return Resource{}, err
	}
	if in.Title != "" {
		r.Title = strings.TrimSpace(in.Title)
	}
	if in.Visible != nil {
		r.Visible = *in.Visible
	}
	if in.Required != nil {
		r.Required = *in.Required
	}
	if in.Downloadable != nil {
		r.Downloadable = *in.Downloadable
	}
	if in.AssetID != nil {
		r.AssetID = in.AssetID
	}
	if in.ExternalURL != nil {
		r.ExternalURL = in.ExternalURL
	}
	r.UpdatedAt = time.Now().UTC()

	if err := s.store.UpdateResource(ctx, s.store.DB(), r); err != nil {
		return Resource{}, err
	}
	return r, nil
}
