package admin

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"mooc/backend/internal/modules/audit"
	"mooc/backend/internal/platform/dbx"
	"mooc/backend/internal/platform/ids"
	"mooc/backend/internal/platform/jobs"
	"mooc/backend/internal/platform/passwords"
)

// TTLInvitacionHoras es lo que dura el enlace para que un profesor invitado
// fije su contraseña. Más largo que un restablecimiento normal porque una
// invitación puede quedarse sin leer un par de días.
const TTLInvitacionHoras = 72

type Service struct {
	store    Store
	sesiones RevocadorDeSesiones
	queue    Publicador
	audit    *audit.Recorder
	log      *slog.Logger
}

func NewService(s Store, r RevocadorDeSesiones, q Publicador, rec *audit.Recorder, log *slog.Logger) *Service {
	return &Service{store: s, sesiones: r, queue: q, audit: rec, log: log}
}

type Actor struct {
	UserID uuid.UUID
	Role   string
	IP     string
	Agent  string
	Trace  string
}

func (s *Service) ListarUsuarios(ctx context.Context, f FiltroUsuarios) ([]Usuario, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 50
	}
	return s.store.ListarUsuarios(ctx, s.store.DB(), f)
}

func (s *Service) Usuario(ctx context.Context, id uuid.UUID) (Usuario, error) {
	return s.store.UsuarioPorID(ctx, s.store.DB(), id)
}

// CrearUsuario da de alta un profesor o administrador y le encola la
// invitación. La cuenta nace sin contraseña utilizable y con el correo dado por
// verificado: el administrador responde por él.
func (s *Service) CrearUsuario(ctx context.Context, in NuevoUsuario, a Actor) (Usuario, error) {
	if errs := in.Validate(); !errs.Empty() {
		return Usuario{}, errs
	}

	now := time.Now().UTC()
	u := Usuario{
		ID:       ids.New(),
		Email:    strings.ToLower(strings.TrimSpace(in.Email)),
		FullName: strings.TrimSpace(in.FullName),
		Role:     in.Role,
		Status:   EstadoActivo,
		// Verificado de oficio: la invitación llega a ese buzón, así que
		// abrirla ya demuestra el control del correo.
		EmailVerifiedAt: &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// Una contraseña aleatoria que nadie conoce. La cuenta solo se activa por
	// el enlace de la invitación; dejarla sin hash obligaría a tratar el caso
	// «sin contraseña» en el login, que es justo donde no conviene añadir ramas.
	aleatoria, err := ids.Token()
	if err != nil {
		return Usuario{}, err
	}
	hash, err := passwords.Hash(aleatoria)
	if err != nil {
		return Usuario{}, err
	}

	rawToken, err := ids.Token()
	if err != nil {
		return Usuario{}, err
	}
	sum := sha256.Sum256([]byte(rawToken))
	tokenID := ids.New()

	clave := fmt.Sprintf("%s:invitation:%s", jobs.TypeEmailSend, tokenID)
	payload := map[string]any{
		"template": "password_reset",
		"user_id":  u.ID.String(),
		"to":       u.Email,
		"name":     u.FullName,
		"token":    rawToken,
	}

	err = s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.InsertarUsuario(ctx, tx, u, hash); err != nil {
			return err
		}
		if err := s.store.InsertarTokenInvitacion(ctx, tx, tokenID, u.ID, sum[:], TTLInvitacionHoras); err != nil {
			return err
		}
		if err := jobs.Record(ctx, tx, jobs.Job{
			Key: clave, Type: jobs.TypeEmailSend, Queue: jobs.QueueCritical, Payload: payload,
		}); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: &a.UserID, ActorRole: a.Role, Action: "admin.user_created",
			EntityType: "user", EntityID: &u.ID,
			IP: a.IP, UserAgent: a.Agent, TraceID: a.Trace,
			Metadata: map[string]any{"email": u.Email, "role": u.Role},
		})
	})
	if err != nil {
		return Usuario{}, err
	}

	if err := s.queue.Publish(ctx, clave, jobs.TypeEmailSend, jobs.QueueCritical, payload, a.Trace); err != nil {
		s.log.Warn("no se pudo publicar la invitación; el reaper la recuperará",
			"job_key", clave, "error", err)
	}
	return u, nil
}

// ActualizarUsuario cambia rol o estado. La comprobación del último
// administrador ocurre DENTRO de la transacción: hacerla antes dejaría una
// ventana en la que dos peticiones simultáneas degradan a los dos últimos.
func (s *Service) ActualizarUsuario(ctx context.Context, id uuid.UUID, c CambioDeUsuario, a Actor) (Usuario, error) {
	if errs := c.Validate(); !errs.Empty() {
		return Usuario{}, errs
	}

	var out Usuario
	err := s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		actual, err := s.store.UsuarioPorID(ctx, tx, id)
		if err != nil {
			return err
		}
		otros, err := s.store.OtrosAdminsActivos(ctx, tx, id)
		if err != nil {
			return err
		}
		if DejaSinAdministrador(actual, c, otros) {
			return ErrUltimoAdministrador
		}

		out, err = s.store.ActualizarUsuario(ctx, tx, id, c)
		if err != nil {
			return err
		}

		// Suspender o eliminar debe cortar el acceso ya, no cuando expire la
		// sesión. El borrado en Redis va tras el commit.
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: &a.UserID, ActorRole: a.Role, Action: "admin.user_updated",
			EntityType: "user", EntityID: &id,
			IP: a.IP, UserAgent: a.Agent, TraceID: a.Trace,
			Metadata: map[string]any{
				"role_anterior": actual.Role, "estado_anterior": actual.Status,
				"role_nuevo": out.Role, "estado_nuevo": out.Status,
			},
		})
	})
	if err != nil {
		return Usuario{}, err
	}

	if out.Status != EstadoActivo {
		if err := s.sesiones.DeleteAllForUser(ctx, id); err != nil {
			s.log.Error("no se pudieron revocar las sesiones tras desactivar",
				"user_id", id, "error", err)
		}
	}
	return out, nil
}

func (s *Service) Sesiones(ctx context.Context, userID uuid.UUID) ([]Sesion, error) {
	return s.store.SesionesDe(ctx, s.store.DB(), userID)
}

// RevocarSesiones corta el acceso de un usuario con efecto inmediato. Redis
// manda sobre la validez, así que se borra allí primero.
func (s *Service) RevocarSesiones(ctx context.Context, userID uuid.UUID, a Actor) (int, error) {
	if _, err := s.store.UsuarioPorID(ctx, s.store.DB(), userID); err != nil {
		return 0, err
	}
	if err := s.sesiones.DeleteAllForUser(ctx, userID); err != nil {
		return 0, err
	}
	n, err := s.store.RevocarSesionesDe(ctx, s.store.DB(), userID)
	if err != nil {
		return 0, err
	}
	s.audit.RecordBestEffort(ctx, s.store.DB(), audit.Event{
		ActorID: &a.UserID, ActorRole: a.Role, Action: audit.ActionSessionRevoked,
		EntityType: "user", EntityID: &userID,
		IP: a.IP, UserAgent: a.Agent, TraceID: a.Trace,
		Metadata: map[string]any{"sesiones": n},
	})
	return n, nil
}

func (s *Service) Auditoria(ctx context.Context, f FiltroAuditoria) ([]EventoAuditoria, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	return s.store.Auditoria(ctx, s.store.DB(), f)
}

func (s *Service) Trabajos(ctx context.Context, f FiltroTrabajos) ([]Trabajo, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	return s.store.Trabajos(ctx, s.store.DB(), f)
}

// Reencolar devuelve un trabajo de la DLQ a la cola CON LA MISMA job_key, que
// es lo que el segmento 4 de la demostración debe evidenciar. El reclamo
// atómico impide que reejecutarlo produzca una segunda salida (ADR-0008).
func (s *Service) Reencolar(ctx context.Context, id int64, a Actor) (Trabajo, error) {
	t, err := s.store.TrabajoPorID(ctx, s.store.DB(), id)
	if err != nil {
		return Trabajo{}, err
	}
	if t.Status != "dead" && t.Status != "failed" {
		return Trabajo{}, fmt.Errorf("%w: el trabajo está en estado %s", ErrNoReencolable, t.Status)
	}

	t, err = s.store.ReencolarTrabajo(ctx, s.store.DB(), id)
	if err != nil {
		return Trabajo{}, err
	}
	if err := s.queue.Publish(ctx, t.JobKey, t.Type, t.Queue, nil, a.Trace); err != nil {
		s.log.Warn("no se pudo publicar el reencolado; el reaper lo recuperará",
			"job_key", t.JobKey, "error", err)
	}

	s.audit.RecordBestEffort(ctx, s.store.DB(), audit.Event{
		ActorID: &a.UserID, ActorRole: a.Role, Action: "admin.job_requeued",
		EntityType: "job", IP: a.IP, UserAgent: a.Agent, TraceID: a.Trace,
		Metadata: map[string]any{"job_key": t.JobKey, "type": t.Type},
	})
	return t, nil
}

var ErrNoReencolable = errors.New("admin: el trabajo no está en la DLQ")
