package main

import (
	"context"
	"log/slog"
	"net/http"

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
	objects objectstore.Almacen
}

func (h *health) live(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *health) ready(log *slog.Logger) http.HandlerFunc {
	return httpx.Disponibilidad(log,
		httpx.Dependencia{Nombre: "postgres", Ping: h.pool.Ping},
		httpx.Dependencia{Nombre: "redis", Ping: func(ctx context.Context) error { return h.redis.Ping(ctx).Err() }},
		httpx.Dependencia{Nombre: "objectstore", Ping: h.objects.Ping},
	)
}
