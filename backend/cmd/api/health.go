package main

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"mooc/backend/internal/adapters/objectstore"
	"mooc/backend/internal/platform/httpx"
)

// health separa vivacidad de preparación: Compose y, más adelante, Cloud Run
// usan readyz para el orden de arranque (ADR-0009).
type health struct {
	pool    *pgxpool.Pool
	redis   *redis.Client
	objects *objectstore.Store
}

func (h *health) live(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *health) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := map[string]string{}
	ok := true

	if err := h.pool.Ping(ctx); err != nil {
		checks["postgres"] = err.Error()
		ok = false
	} else {
		checks["postgres"] = "ok"
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = err.Error()
		ok = false
	} else {
		checks["redis"] = "ok"
	}

	if err := h.objects.Ping(ctx); err != nil {
		checks["objectstore"] = err.Error()
		ok = false
	} else {
		checks["objectstore"] = "ok"
	}

	status := http.StatusOK
	if !ok {
		status = http.StatusServiceUnavailable
	}
	httpx.JSON(w, status, checks)
}
