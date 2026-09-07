// Command api es el monolito modular HTTP (ADR-0001).
//
// No mantiene estado local: sesiones en Redis, datos en PostgreSQL, binarios en
// el almacén de objetos. Por eso `--scale api=N` funciona sin sesiones pegajosas.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/adapters/objectstore"
	"mooc/backend/internal/adapters/postgres"
	"mooc/backend/internal/adapters/queue"
	"mooc/backend/internal/adapters/rediscli"
	"mooc/backend/internal/adapters/sessions"
	"mooc/backend/internal/modules/assessment"
	"mooc/backend/internal/modules/audit"
	"mooc/backend/internal/modules/authoring"
	"mooc/backend/internal/modules/badges"
	"mooc/backend/internal/modules/identity"
	"mooc/backend/internal/modules/learning"
	"mooc/backend/internal/platform/config"
	"mooc/backend/internal/platform/httpx"
	"mooc/backend/internal/platform/jobs"
	"mooc/backend/internal/platform/logging"
	"mooc/backend/internal/platform/metrics"
	"mooc/backend/internal/platform/ratelimit"
	"mooc/backend/openapi"
)

func main() {
	if err := run(); err != nil {
		slog.Error("la api no pudo arrancar", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.LogLevel)
	// Para que los errores registrados fuera de un handler con logger propio
	// (httpx.Fail) salgan en el mismo formato estructurado.
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- adaptadores ------------------------------------------------------
	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	sessionRedis, err := rediscli.Open(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisSessionDB)
	if err != nil {
		return err
	}
	defer sessionRedis.Close()

	// Base lógica propia: los contadores de tasa no comparten espacio de
	// claves con las sesiones ni con la cola (ADR-0004).
	limitRedis, err := rediscli.Open(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisLimitDB)
	if err != nil {
		return err
	}
	defer limitRedis.Close()
	limiter := ratelimit.New(limitRedis)

	store, err := objectstore.Open(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey,
		cfg.S3UseSSL, cfg.S3Buckets.Originals)
	if err != nil {
		return err
	}

	publisher := queue.NewPublisher(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisQueueDB)
	defer publisher.Close()

	// --- módulos ----------------------------------------------------------
	recorder := audit.NewRecorder(log)
	identityStore := postgres.NewIdentityStore(pool)
	identitySvc := identity.NewService(
		identityStore,
		sessions.New(sessionRedis),
		queueBridge{publisher},
		recorder,
		identity.Config{
			SessionTTL:     cfg.SessionTTL,
			VerifyTokenTTL: cfg.VerifyTokenTTL,
			PublicBaseURL:  cfg.PublicBaseURL,
		},
		log,
	)

	// --- ruteo ------------------------------------------------------------
	authoringSvc := authoring.NewService(postgres.NewAuthoringStore(pool), recorder, log)
	learningSvc := learning.NewService(postgres.NewLearningStore(pool), recorder,
		learningBridge{publisher}, learning.UmbralesPorDefecto(), log)
	badgeSvc := badges.NewService(postgres.NewBadgeStore(pool), log)
	assessmentSvc := assessment.NewService(postgres.NewAssessmentStore(pool), log)

	metrics.Inicializar(jobs.TypeEmailSend, jobs.TypeBadgeIssue)

	mux := http.NewServeMux()
	auth := identity.Authenticate(identitySvc)
	soloDocentes := identity.RequireRole(identity.RoleTeacher, identity.RoleAdmin)
	soloAdmin := identity.RequireRole(identity.RoleAdmin)

	identity.NewAPI(identitySvc).Routes(mux, auth, identity.Limitador{
		PorIP: func(r ratelimit.Regla) httpx.Middleware {
			return limiter.Middleware(r, ratelimit.PorIP, log)
		},
		PorCuenta: func(r ratelimit.Regla) httpx.Middleware {
			return limiter.Middleware(r, ratelimit.PorCampoDelCuerpo("email"), log)
		},
	})
	authoring.NewAPI(authoringSvc).Routes(mux, auth, soloDocentes)
	learning.NewAPI(learningSvc, tamperAuditor{pool: pool, rec: recorder, log: log}).
		Routes(mux, auth, func(r ratelimit.Regla) httpx.Middleware {
			// Por sesión, no por IP: varios estudiantes tras el mismo NAT no
			// deben estorbarse.
			return limiter.Middleware(r, ratelimit.PorSesion, log)
		})
	badges.NewAPI(badgeSvc).Routes(mux, auth, soloAdmin)
	assessment.NewAPI(assessmentSvc).Routes(mux, auth, soloDocentes)

	health := &health{pool: pool, redis: sessionRedis, objects: store}
	mux.HandleFunc("GET /healthz", health.live)
	mux.HandleFunc("GET /readyz", health.ready)
	// Solo red interna: Compose no publica este puerto fuera.
	mux.Handle("GET /metrics", metrics.Handler())

	// La API sirve su propio contrato (RT-05, entregable §8).
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(openapi.Spec)
	})

	handler := httpx.Chain(mux,
		httpx.RequestID(),
		httpx.Recover(log),
		metrics.Middleware(),
		httpx.AccessLog(log),
	)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("api escuchando", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("apagando la api")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// queueBridge adapta el publicador de asynq al puerto que declara identity.
// La composición vive en cmd/, que es el único sitio que conoce a ambos lados
// (ADR-0001).
type queueBridge struct {
	p *queue.Publisher
}

func (b queueBridge) Publish(ctx context.Context, j identity.JobRef, traceID string) error {
	return b.p.Publish(ctx, jobs.Job{
		Key: j.Key, Type: j.Type, Queue: j.Queue, Payload: j.Payload,
	}, traceID)
}

// learningBridge adapta el mismo publicador a la firma que declara learning.
// Son dos puertos distintos porque cada módulo define el suyo; el adaptador
// vive aquí, en la composición (ADR-0001).
type learningBridge struct{ p *queue.Publisher }

func (b learningBridge) Publish(ctx context.Context, key, tipo, cola string,
	payload map[string]any, traceID string) error {
	return b.p.Publish(ctx, jobs.Job{Key: key, Type: tipo, Queue: cola, Payload: payload}, traceID)
}

// tamperAuditor registra el intento de enviar un progreso calculado por el
// cliente. Es la evidencia que exige CA-05 y que el segmento 7 debe mostrar.
type tamperAuditor struct {
	pool *pgxpool.Pool
	rec  *audit.Recorder
	log  *slog.Logger
}

func (t tamperAuditor) TamperingAttempt(r *http.Request, enrollmentID uuid.UUID, cuerpo map[string]any) {
	p, _ := httpx.PrincipalFrom(r.Context())
	t.rec.RecordBestEffort(r.Context(), t.pool, audit.Event{
		ActorID: &p.UserID, ActorRole: p.Role,
		Action: "progress.tampering_attempt", EntityType: "enrollment", EntityID: &enrollmentID,
		IP: httpx.ClientIP(r), UserAgent: r.UserAgent(),
		TraceID:  httpx.RequestIDFrom(r.Context()),
		Metadata: map[string]any{"body": cuerpo},
	})
	t.log.Warn("intento de enviar progreso calculado por el cliente",
		"enrollment_id", enrollmentID, "user_id", p.UserID, "body", cuerpo)
}
