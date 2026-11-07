package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/adapters/postgres"
	"mooc/backend/internal/adapters/queue"
	"mooc/backend/internal/platform/config"
	"mooc/backend/internal/platform/httpx"
	"mooc/backend/internal/platform/metrics"
)

// servirPush es el worker de Cloud Run (ADR-0020). No consume de una cola: la
// suscripción push de Pub/Sub le entrega cada mensaje como una petición, y
// Cloud Scheduler le pide el reaper y la poda, porque Cloud Run solo da CPU
// mientras atiende una petición y un bucle en segundo plano no correría.
//
// Cloud Run autentica antes de llegar aquí. Solo la cuenta de la suscripción y
// la de Cloud Scheduler tienen roles/run.invoker sobre este servicio.
func servirPush(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool,
	runner *queue.Runner, manejadores map[string]queue.Handler,
	reaper *queue.Reaper, podador *postgres.Podador) error {

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", httpx.Disponibilidad(log,
		httpx.Dependencia{Nombre: "postgres", Ping: pool.Ping}))
	mux.Handle("GET /metrics", metrics.Handler())
	mux.Handle("POST /pubsub/push", queue.NewPush(runner, manejadores, cfg.PubSubMaxEntregas, log))
	mux.HandleFunc("POST /tareas/reaper", func(w http.ResponseWriter, r *http.Request) {
		n, err := reaper.Run(r.Context())
		if err != nil {
			log.Error("el reaper falló", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]int{"recuperados": n})
	})
	mux.HandleFunc("POST /tareas/podar", func(w http.ResponseWriter, r *http.Request) {
		resumen, err := podador.Run(r.Context())
		if err != nil {
			log.Error("la poda falló", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		httpx.JSON(w, http.StatusOK, resumen)
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpx.Chain(mux, httpx.RequestID(), httpx.Recover(log), httpx.AccessLog(log)),
		ReadHeaderTimeout: 10 * time.Second,
		// Sin WriteTimeout: una transcodificación responde al terminar, hasta
		// el tiempo máximo de solicitud del servicio (600 s).
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("worker push escuchando", "addr", cfg.HTTPAddr, "topic", cfg.PubSubTopic)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	// Cloud Run da 10 s tras SIGTERM. Lo que no termine se corta: el runner lo
	// marca reclamable y Pub/Sub lo entrega de nuevo, porque no se confirmó.
	log.Info("apagando el worker push")
	apagado, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	defer cancel()
	return srv.Shutdown(apagado)
}
