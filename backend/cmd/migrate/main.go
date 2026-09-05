// Command migrate aplica las migraciones SQL versionadas.
//
// Corre como job separado en el arranque (ADR-0003), que es también la forma en
// que Cloud Run espera ejecutarlas (ADR-0010).
package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/adapters/postgres"
	"mooc/backend/internal/platform/config"
	"mooc/backend/internal/platform/logging"
	"mooc/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migración fallida", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.LogLevel)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Protocolo simple: los archivos de migración traen varias sentencias por
	// archivo, y el protocolo extendido admite solo una.
	pool, err := postgres.Open(ctx, cfg.DatabaseURL, func(c *pgxpool.Config) {
		c.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS platform;
		CREATE TABLE IF NOT EXISTS platform.schema_migrations (
		    version    text        PRIMARY KEY,
		    applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("crear tabla de migraciones: %w", err)
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	files, err := upFiles()
	if err != nil {
		return err
	}

	pending := 0
	for _, name := range files {
		version := strings.TrimSuffix(name, ".up.sql")
		if applied[version] {
			continue
		}
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("leer %s: %w", name, err)
		}

		start := time.Now()
		if err := applyOne(ctx, pool, version, string(body)); err != nil {
			return fmt.Errorf("aplicar %s: %w", name, err)
		}
		pending++
		log.Info("migración aplicada", "version", version, "duration_ms", time.Since(start).Milliseconds())
	}

	if pending == 0 {
		log.Info("esquema al día", "aplicadas", len(applied))
	} else {
		log.Info("migraciones completadas", "nuevas", pending)
	}
	return nil
}

// applyOne ejecuta la migración y registra su versión en la misma transacción:
// o se aplica entera y queda anotada, o no ocurre nada.
func applyOne(ctx context.Context, pool *pgxpool.Pool, version, body string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, body); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO platform.schema_migrations (version) VALUES ($1)`, version); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM platform.schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("leer migraciones aplicadas: %w", err)
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func upFiles() ([]string, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			names = append(names, e.Name())
		}
	}
	// Orden léxico: los nombres van numerados con ceros a la izquierda.
	sort.Strings(names)
	return names, nil
}
