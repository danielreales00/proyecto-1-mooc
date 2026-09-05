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

	"mooc/backend/internal/adapters/objectstore"
	"mooc/backend/internal/adapters/postgres"
	"mooc/backend/internal/adapters/queue"
	"mooc/backend/internal/adapters/rediscli"
	"mooc/backend/internal/adapters/sessions"
	"mooc/backend/internal/modules/audit"
	"mooc/backend/internal/modules/identity"
	"mooc/backend/internal/platform/config"
	"mooc/backend/internal/platform/httpx"
	"mooc/backend/internal/platform/jobs"
	"mooc/backend/internal/platform/logging"
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
	mux := http.NewServeMux()
	auth := identity.Authenticate(identitySvc)
	identity.NewAPI(identitySvc).Routes(mux, auth)

	health := &health{pool: pool, redis: sessionRedis, objects: store}
	mux.HandleFunc("GET /healthz", health.live)
	mux.HandleFunc("GET /readyz", health.ready)

	handler := httpx.Chain(mux,
		httpx.RequestID(),
		httpx.Recover(log),
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
