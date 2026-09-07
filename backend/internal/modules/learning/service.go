package learning

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"mooc/backend/internal/modules/audit"
	"mooc/backend/internal/platform/dbx"
	"mooc/backend/internal/platform/ids"
	"mooc/backend/internal/platform/jobs"
)

type Service struct {
	store    Store
	audit    *audit.Recorder
	queue    Publisher
	umbrales Umbrales
	log      *slog.Logger
}

func NewService(store Store, rec *audit.Recorder, queue Publisher, u Umbrales, log *slog.Logger) *Service {
	return &Service{store: store, audit: rec, queue: queue, umbrales: u, log: log}
}

type Actor struct {
	UserID uuid.UUID
	Role   string
	IP     string
	Agent  string
	Trace  string
}

// ------------------------------------------------------------- catálogo ----

func (s *Service) Search(ctx context.Context, q, categoria, idioma string, limit int) ([]CatalogCourse, error) {
	return s.store.SearchCatalog(ctx, s.store.DB(), NormalizarBusqueda(q), categoria, idioma, limit)
}

func (s *Service) CourseBySlug(ctx context.Context, slug string) (CatalogCourse, error) {
	return s.store.CatalogBySlug(ctx, s.store.DB(), slug)
}

// ----------------------------------------------------------- inscripción ---

// Enroll inscribe o reactiva. Reinscribirse conserva el progreso y los
// resultados: al retirarse no se borró nada (RF-10).
func (s *Service) Enroll(ctx context.Context, courseID uuid.UUID, a Actor) (Enrollment, error) {
	if _, _, _, _, err := s.store.CurrentPublishedVersion(ctx, s.store.DB(), courseID); err != nil {
		return Enrollment{}, err
	}

	e := Enrollment{
		ID: ids.New(), UserID: a.UserID, CourseID: courseID,
		Status: StatusActive, State: StateInProgress, EnrolledAt: time.Now().UTC(),
	}

	var out Enrollment
	err := s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		var err error
		out, err = s.store.UpsertEnrollment(ctx, tx, e)
		if err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: &a.UserID, ActorRole: a.Role, Action: "enrollment.created",
			EntityType: "enrollment", EntityID: &out.ID,
			IP: a.IP, UserAgent: a.Agent, TraceID: a.Trace,
			Metadata: map[string]any{"course_id": courseID.String()},
		})
	})
	if err != nil {
		return Enrollment{}, err
	}
	return out, nil
}

func (s *Service) MyEnrollments(ctx context.Context, a Actor) ([]Enrollment, error) {
	return s.store.EnrollmentsByUser(ctx, s.store.DB(), a.UserID)
}

// guard comprueba que la inscripción sea del solicitante (CA-06).
func (s *Service) guard(ctx context.Context, id uuid.UUID, a Actor) (Enrollment, error) {
	e, err := s.store.EnrollmentByID(ctx, s.store.DB(), id)
	if err != nil {
		return Enrollment{}, err
	}
	if e.UserID != a.UserID && a.Role != "admin" {
		return Enrollment{}, ErrProhibido
	}
	return e, nil
}

func (s *Service) GetEnrollment(ctx context.Context, id uuid.UUID, a Actor) (Enrollment, error) {
	return s.guard(ctx, id, a)
}

// Withdraw marca la inscripción como retirada sin borrar progreso.
func (s *Service) Withdraw(ctx context.Context, id uuid.UUID, a Actor) (Enrollment, error) {
	e, err := s.guard(ctx, id, a)
	if err != nil {
		return Enrollment{}, err
	}
	if e.Status == StatusWithdrawn {
		return e, nil
	}
	ahora := time.Now().UTC()
	e.Status = StatusWithdrawn
	e.WithdrawnAt = &ahora

	err = s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.UpdateEnrollment(ctx, tx, e); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: &a.UserID, ActorRole: a.Role, Action: "enrollment.withdrawn",
			EntityType: "enrollment", EntityID: &e.ID,
			IP: a.IP, UserAgent: a.Agent, TraceID: a.Trace,
		})
	})
	if err != nil {
		return Enrollment{}, err
	}
	return e, nil
}

// ------------------------------------------------------------ contenido ----

type ContentResource struct {
	ResourceRef
	Progress ResourceProgress
}

func (s *Service) Content(ctx context.Context, id uuid.UUID, a Actor) (Enrollment, []ContentResource, error) {
	e, err := s.guard(ctx, id, a)
	if err != nil {
		return Enrollment{}, nil, err
	}
	versionID, _, _, _, err := s.store.CurrentPublishedVersion(ctx, s.store.DB(), e.CourseID)
	if err != nil {
		return Enrollment{}, nil, err
	}
	recursos, err := s.store.ResourcesOfVersion(ctx, s.store.DB(), versionID)
	if err != nil {
		return Enrollment{}, nil, err
	}
	avance, err := s.store.ProgressOf(ctx, s.store.DB(), e.ID)
	if err != nil {
		return Enrollment{}, nil, err
	}

	out := make([]ContentResource, 0, len(recursos))
	for _, r := range recursos {
		p, ok := avance[r.StableID]
		if !ok {
			p = ResourceProgress{EnrollmentID: e.ID, ResourceStableID: r.StableID, State: ProgressNotStarted}
		}
		out = append(out, ContentResource{ResourceRef: r, Progress: p})
	}
	return e, out, nil
}

// ------------------------------------------------------------- progreso ----

// ErrValorCalculadoPorCliente es CA-05: el cliente reporta evidencias, nunca
// conclusiones.
var ErrValorCalculadoPorCliente = errors.New("learning: el cliente envió un valor calculado")

type Resultado struct {
	Aceptada      bool
	Motivo        string
	EstadoRecurso string
	ProgresoPct   float64
	Estado        string

	// badgeJobKey se rellena cuando esta evidencia disparó la aprobación. El
	// trabajo ya está registrado en job_runs; solo falta publicarlo en la cola,
	// que ocurre tras el commit (ADR-0004).
	badgeJobKey     string
	badgeEnrollment uuid.UUID
}

// ReportEvidence ingiere una evidencia, la valida en el servidor y recalcula.
func (s *Service) ReportEvidence(ctx context.Context, id uuid.UUID, ev Evidence, a Actor) (Resultado, error) {
	e, err := s.guard(ctx, id, a)
	if err != nil {
		return Resultado{}, err
	}
	if !e.Activa() {
		s.registrarEvidencia(ctx, e.ID, ev, false, RejectNotEnrolled)
		return Resultado{Aceptada: false, Motivo: RejectNotEnrolled}, nil
	}

	versionID, _, criterioPct, notaMin, err := s.store.CurrentPublishedVersion(ctx, s.store.DB(), e.CourseID)
	if err != nil {
		return Resultado{}, err
	}
	recursos, err := s.store.ResourcesOfVersion(ctx, s.store.DB(), versionID)
	if err != nil {
		return Resultado{}, err
	}

	var ref *ResourceRef
	for i := range recursos {
		if recursos[i].StableID == ev.ResourceStableID {
			ref = &recursos[i]
			break
		}
	}
	if ref == nil {
		s.registrarEvidencia(ctx, e.ID, ev, false, RejectUnknownResource)
		return Resultado{Aceptada: false, Motivo: RejectUnknownResource}, nil
	}

	previo, err := s.store.ProgressOne(ctx, s.store.DB(), e.ID, ev.ResourceStableID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Resultado{}, err
	}
	if errors.Is(err, ErrNotFound) {
		previo = ResourceProgress{EnrollmentID: e.ID, ResourceStableID: ev.ResourceStableID,
			State: ProgressNotStarted, UpdatedAt: time.Now().UTC()}
	}

	ahora := time.Now().UTC()
	v := ValidarEvidencia(ev, previo, ref.Duration, ahora, s.umbrales)

	if !v.Aceptada {
		s.registrarEvidencia(ctx, e.ID, ev, false, v.Motivo)
		return Resultado{Aceptada: false, Motivo: v.Motivo,
			EstadoRecurso: previo.State, ProgresoPct: e.ProgressPct, Estado: e.State}, nil
	}

	// Aplicar la evidencia.
	nuevo := previo
	nuevo.UpdatedAt = ahora
	if ev.Kind == EvidenceOpen && nuevo.OpenedAt == nil {
		nuevo.OpenedAt = &ahora
	}
	if nuevo.State == ProgressNotStarted {
		nuevo.State = ProgressInProgress
	}
	nuevo.DwellMsTotal += v.DwellMs
	if ev.PositionSeconds != nil {
		nuevo.LastPositionSeconds = *ev.PositionSeconds
		if *ev.PositionSeconds > nuevo.MaxPositionSeconds {
			nuevo.MaxPositionSeconds = *ev.PositionSeconds
		}
	}
	if nuevo.State != ProgressCompleted && CriterioCompletado(ref.Type, nuevo, ref.Duration, s.umbrales) {
		nuevo.State = ProgressCompleted
		nuevo.CompletedAt = &ahora
	}

	var res Resultado
	err = s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.UpsertProgress(ctx, tx, nuevo); err != nil {
			return err
		}
		if err := s.store.InsertEvent(ctx, tx, e.ID, ev.ResourceStableID, ev.Kind,
			ev.PositionSeconds, ev.ClientTS, true, ""); err != nil {
			return err
		}
		var err error
		res, err = s.recalcular(ctx, tx, e, versionID, criterioPct, notaMin, a)
		return err
	})
	if err != nil {
		return Resultado{}, err
	}
	res.Aceptada = true
	res.EstadoRecurso = nuevo.State

	if res.badgeJobKey != "" {
		if err := s.queue.Publish(ctx, res.badgeJobKey, jobs.TypeBadgeIssue, jobs.QueueCritical,
			map[string]any{"enrollment_id": res.badgeEnrollment.String()}, a.Trace); err != nil {
			// No se falla la petición: el trabajo ya está en PostgreSQL y el
			// reaper lo recuperará.
			s.log.Warn("no se pudo publicar la emisión de insignia",
				"job_key", res.badgeJobKey, "error", err)
		}
	}
	return res, nil
}

// recalcular recompone el porcentaje y el estado, y encola la insignia al
// aprobar. Corre dentro de la transacción del llamante.
func (s *Service) recalcular(ctx context.Context, tx dbx.DB, e Enrollment,
	versionID uuid.UUID, criterioPct, notaMin float64, a Actor) (Resultado, error) {

	recursos, err := s.store.ResourcesOfVersion(ctx, tx, versionID)
	if err != nil {
		return Resultado{}, err
	}
	avance, err := s.store.ProgressOf(ctx, tx, e.ID)
	if err != nil {
		return Resultado{}, err
	}

	obligatorios, completados := 0, 0
	for _, r := range recursos {
		if !r.Required || !r.Visible {
			continue
		}
		obligatorios++
		if p, ok := avance[r.StableID]; ok && p.State == ProgressCompleted {
			completados++
		}
	}
	pct := CalcularPorcentaje(obligatorios, completados)

	notaOK := true
	if notaMin > 0 {
		notaOK, err = s.store.MinQuizScoreCumplida(ctx, tx, e.ID, versionID, notaMin)
		if err != nil {
			return Resultado{}, err
		}
	}

	anterior := e.State
	e.State = EvaluarEstado(e.State, pct, criterioPct, notaOK)
	e.ProgressPct = pct
	ahora := time.Now().UTC()
	if e.State == StateCompleted && e.CompletedAt == nil {
		e.CompletedAt = &ahora
	}
	if e.State == StateApproved && e.ApprovedAt == nil {
		e.ApprovedAt = &ahora
		e.CompletedAt = &ahora
		e.ApprovedVersionID = &versionID
	}

	if err := s.store.UpdateEnrollment(ctx, tx, e); err != nil {
		return Resultado{}, err
	}

	// Al aprobar se encola la insignia con clave determinista: aunque la
	// transición se dispare dos veces, la insignia es una sola (ADR-0008).
	if anterior != StateApproved && e.State == StateApproved {
		clave := fmt.Sprintf("%s:%s", jobs.TypeBadgeIssue, e.ID)
		if err := jobs.Record(ctx, tx, jobs.Job{
			Key: clave, Type: jobs.TypeBadgeIssue, Queue: jobs.QueueCritical,
			Payload: map[string]any{"enrollment_id": e.ID.String()},
		}); err != nil {
			return Resultado{}, err
		}
		if err := s.audit.Record(ctx, tx, audit.Event{
			ActorID: &e.UserID, ActorRole: "student", Action: "enrollment.approved",
			EntityType: "enrollment", EntityID: &e.ID,
			IP: a.IP, UserAgent: a.Agent, TraceID: a.Trace,
			Metadata: map[string]any{"progress_pct": pct},
		}); err != nil {
			return Resultado{}, err
		}
		// La publicación en la cola va DESPUÉS del commit; la clave viaja de
		// vuelta al llamante para eso.
		return Resultado{ProgresoPct: pct, Estado: e.State, badgeJobKey: clave,
			badgeEnrollment: e.ID}, nil
	}

	return Resultado{ProgresoPct: pct, Estado: e.State}, nil
}

func (s *Service) registrarEvidencia(ctx context.Context, enrollmentID uuid.UUID, ev Evidence, aceptada bool, motivo string) {
	if err := s.store.InsertEvent(ctx, s.store.DB(), enrollmentID, ev.ResourceStableID,
		ev.Kind, ev.PositionSeconds, ev.ClientTS, aceptada, motivo); err != nil {
		s.log.Error("no se pudo registrar la evidencia", "error", err)
	}
}

// Summary devuelve el progreso calculado por el servidor.
func (s *Service) Summary(ctx context.Context, id uuid.UUID, a Actor) (Enrollment, []ContentResource, int, int, error) {
	e, recursos, err := s.Content(ctx, id, a)
	if err != nil {
		return Enrollment{}, nil, 0, 0, err
	}
	obligatorios, completados := 0, 0
	for _, r := range recursos {
		if r.Required && r.Visible {
			obligatorios++
			if r.Progress.State == ProgressCompleted {
				completados++
			}
		}
	}
	return e, recursos, obligatorios, completados, nil
}
