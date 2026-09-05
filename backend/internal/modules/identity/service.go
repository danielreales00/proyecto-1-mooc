package identity

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"mooc/backend/internal/modules/audit"
	"mooc/backend/internal/platform/dbx"
	"mooc/backend/internal/platform/ids"
	"mooc/backend/internal/platform/jobs"
	"mooc/backend/internal/platform/passwords"
)

type Config struct {
	SessionTTL     time.Duration
	VerifyTokenTTL time.Duration
	PublicBaseURL  string
}

type Service struct {
	store    Store
	sessions SessionStore
	queue    Publisher
	audit    *audit.Recorder
	cfg      Config
	log      *slog.Logger
}

func NewService(store Store, sessions SessionStore, queue Publisher, rec *audit.Recorder, cfg Config, log *slog.Logger) *Service {
	return &Service{store: store, sessions: sessions, queue: queue, audit: rec, cfg: cfg, log: log}
}

// RequestContext son los datos de la petición que la auditoría necesita.
type RequestContext struct {
	IP        string
	UserAgent string
	TraceID   string
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// Register crea un estudiante y encola el correo de verificación.
//
// El usuario, el token y el registro del trabajo entran en la MISMA
// transacción (patrón outbox, ADR-0004); la publicación en la cola ocurre
// después del commit. Si esa publicación falla, la fila queda en 'queued' y el
// reaper la recupera: por eso el registro no falla si Redis está caído.
func (s *Service) Register(ctx context.Context, in RegisterInput, rc RequestContext) (User, error) {
	if errs := in.Validate(); !errs.Empty() {
		return User{}, errs
	}

	email := NormalizeEmail(in.Email)
	if _, err := s.store.UserByEmail(ctx, s.store.DB(), email); err == nil {
		return User{}, ErrEmailTaken
	} else if !errors.Is(err, ErrNotFound) {
		return User{}, err
	}

	hash, err := passwords.Hash(in.Password)
	if err != nil {
		return User{}, err
	}

	now := time.Now().UTC()
	user := NewStudent(ids.New(), in, hash, now)

	rawToken, err := ids.Token()
	if err != nil {
		return User{}, err
	}
	token := OneTimeToken{
		ID:          ids.New(),
		UserID:      user.ID,
		Purpose:     PurposeEmailVerify,
		TokenSHA256: hashToken(rawToken),
		ExpiresAt:   now.Add(s.cfg.VerifyTokenTTL),
	}

	job := JobRef{
		Key:   fmt.Sprintf("%s:%s:%s", jobs.TypeEmailSend, PurposeEmailVerify, token.ID),
		Type:  jobs.TypeEmailSend,
		Queue: jobs.QueueCritical,
		Payload: map[string]any{
			"template": PurposeEmailVerify,
			"user_id":  user.ID.String(),
			"to":       user.Email,
			"name":     user.FullName,
			// El token en claro viaja aquí porque el correo debe contenerlo y la
			// tabla solo guarda su hash. Queda acotado: expira en 24 h y la fila
			// de job_runs se poda. Ver arquitectura/pendientes.md.
			"token": rawToken,
		},
	}

	err = s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.InsertUser(ctx, tx, user); err != nil {
			return err
		}
		if err := s.store.InsertOneTimeToken(ctx, tx, token); err != nil {
			return err
		}
		if err := jobs.Record(ctx, tx, jobs.Job{
			Key: job.Key, Type: job.Type, Queue: job.Queue, Payload: job.Payload,
		}); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: &user.ID, ActorRole: user.Role,
			Action: audit.ActionUserRegistered, EntityType: "user", EntityID: &user.ID,
			IP: rc.IP, UserAgent: rc.UserAgent, TraceID: rc.TraceID,
			Metadata: map[string]any{"email": user.Email},
		})
	})
	if err != nil {
		return User{}, err
	}

	if err := s.queue.Publish(ctx, job, rc.TraceID); err != nil {
		// No se falla el registro: el trabajo ya está en PostgreSQL.
		s.log.Warn("no se pudo publicar en la cola; el reaper lo recuperará",
			"job_key", job.Key, "error", err)
	}
	return user, nil
}

// VerifyEmail consume el token y marca el correo como verificado.
func (s *Service) VerifyEmail(ctx context.Context, rawToken string, rc RequestContext) error {
	if rawToken == "" {
		return ErrTokenInvalid
	}
	now := time.Now().UTC()

	return s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		tok, err := s.store.ConsumeOneTimeToken(ctx, tx, PurposeEmailVerify, hashToken(rawToken), now)
		if err != nil {
			return err
		}
		if err := s.store.MarkEmailVerified(ctx, tx, tok.UserID, now); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: &tok.UserID, ActorRole: RoleStudent,
			Action: audit.ActionEmailVerified, EntityType: "user", EntityID: &tok.UserID,
			IP: rc.IP, UserAgent: rc.UserAgent, TraceID: rc.TraceID,
		})
	})
}

// LoginResult es lo que se devuelve al iniciar sesión.
type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	User      User
}

// ErrInvalidCredentials es deliberadamente el mismo error para usuario
// inexistente y contraseña incorrecta: no se revela cuáles correos existen.
var ErrInvalidCredentials = errors.New("identity: credenciales inválidas")

func (s *Service) Login(ctx context.Context, email, password string, rc RequestContext) (LoginResult, error) {
	email = NormalizeEmail(email)
	now := time.Now().UTC()

	user, err := s.store.UserByEmail(ctx, s.store.DB(), email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			s.auditFailedLogin(ctx, email, "usuario inexistente", rc)
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}

	if err := passwords.Verify(password, user.PasswordHash); err != nil {
		s.auditFailedLogin(ctx, email, "contraseña incorrecta", rc)
		return LoginResult{}, ErrInvalidCredentials
	}

	if ok, reason := user.CanLogIn(); !ok {
		s.auditFailedLogin(ctx, email, reason, rc)
		return LoginResult{}, ErrInvalidCredentials
	}

	rawToken, err := ids.Token()
	if err != nil {
		return LoginResult{}, err
	}
	session := Session{
		ID:          ids.New(),
		UserID:      user.ID,
		Role:        user.Role,
		TokenSHA256: hashToken(rawToken),
		IP:          rc.IP,
		UserAgent:   rc.UserAgent,
		CreatedAt:   now,
		LastSeenAt:  now,
		ExpiresAt:   now.Add(s.cfg.SessionTTL),
	}

	// Redis manda sobre la validez; PostgreSQL guarda la historia (ADR-0006).
	if err := s.sessions.Create(ctx, rawToken, session, s.cfg.SessionTTL); err != nil {
		return LoginResult{}, err
	}
	if err := s.store.InsertSession(ctx, s.store.DB(), session); err != nil {
		_ = s.sessions.Delete(ctx, rawToken)
		return LoginResult{}, err
	}

	s.audit.RecordBestEffort(ctx, s.store.DB(), audit.Event{
		ActorID: &user.ID, ActorRole: user.Role,
		Action: audit.ActionLoginSucceeded, EntityType: "session", EntityID: &session.ID,
		IP: rc.IP, UserAgent: rc.UserAgent, TraceID: rc.TraceID,
	})

	return LoginResult{Token: rawToken, ExpiresAt: session.ExpiresAt, User: user}, nil
}

func (s *Service) auditFailedLogin(ctx context.Context, email, reason string, rc RequestContext) {
	s.audit.RecordBestEffort(ctx, s.store.DB(), audit.Event{
		ActorRole: "anonymous",
		Action:    audit.ActionLoginFailed,
		// No se registra el correo completo por sí solo como identidad; se deja
		// como metadato para poder investigar un ataque.
		EntityType: "user",
		IP:         rc.IP, UserAgent: rc.UserAgent, TraceID: rc.TraceID,
		Metadata: map[string]any{"email": email, "reason": reason},
	})
}

// Logout revoca la sesión actual. Borrar la clave de Redis surte efecto en la
// siguiente petición, sin ventana de gracia (SEG-1).
func (s *Service) Logout(ctx context.Context, rawToken string, sessionID uuid.UUID, actor uuid.UUID, role string, rc RequestContext) error {
	if err := s.sessions.Delete(ctx, rawToken); err != nil {
		return err
	}
	if err := s.store.RevokeSession(ctx, s.store.DB(), sessionID, time.Now().UTC()); err != nil {
		return err
	}
	s.audit.RecordBestEffort(ctx, s.store.DB(), audit.Event{
		ActorID: &actor, ActorRole: role,
		Action: audit.ActionLogout, EntityType: "session", EntityID: &sessionID,
		IP: rc.IP, UserAgent: rc.UserAgent, TraceID: rc.TraceID,
	})
	return nil
}

// Authenticate resuelve un token opaco a su sesión. Es lo que usa el middleware.
func (s *Service) Authenticate(ctx context.Context, rawToken string) (Session, error) {
	return s.sessions.Lookup(ctx, rawToken)
}

func (s *Service) UserByID(ctx context.Context, id uuid.UUID) (User, error) {
	return s.store.UserByID(ctx, s.store.DB(), id)
}
