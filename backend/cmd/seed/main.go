// Command seed carga datos sintéticos para la demostración (§10.1 del
// enunciado exige que se ejecute sobre datos sintéticos).
//
// Es idempotente: correrlo dos veces no duplica cuentas ni falla. Eso importa
// porque se ejecuta antes de cada ensayo de la demo.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"mooc/backend/internal/adapters/postgres"
	"mooc/backend/internal/modules/identity"
	"mooc/backend/internal/platform/config"
	"mooc/backend/internal/platform/ids"
	"mooc/backend/internal/platform/logging"
	"mooc/backend/internal/platform/passwords"
)

// La contraseña es la misma para todas las cuentas sintéticas, a propósito:
// esto solo corre en local y hace la demo reproducible. Nunca en producción.
const claveDemo = "Contrasena-Demo-2026"

type cuenta struct {
	email    string
	nombre   string
	rol      string
	descripc string
}

var cuentas = []cuenta{
	{"admin@mooc.local", "Administradora del Sistema", identity.RoleAdmin, "administra usuarios, roles y auditoría"},
	{"profesor@mooc.local", "Profesor Titular", identity.RoleTeacher, "crea y publica cursos"},
	{"profesora2@mooc.local", "Profesora Invitada", identity.RoleTeacher, "sirve para probar el control por propiedad"},
	{"estudiante1@mooc.local", "Estudiante Uno", identity.RoleStudent, "recorrido feliz"},
	{"estudiante2@mooc.local", "Estudiante Dos", identity.RoleStudent, "segundo inscrito"},
	{"estudiante3@mooc.local", "Estudiante Tres", identity.RoleStudent, "señales de progreso fraudulentas"},
	{"sinverificar@mooc.local", "Sin Verificar", identity.RoleStudent, "correo sin verificar: no debe poder entrar"},
	{"suspendido@mooc.local", "Suspendido", identity.RoleStudent, "cuenta suspendida: no debe poder entrar"},
}

func main() {
	if err := run(); err != nil {
		slog.Error("la semilla falló", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.LogLevel)

	if cfg.Env != "development" && os.Getenv("SEED_FORCE") != "1" {
		return fmt.Errorf("la semilla solo corre en APP_ENV=development (use SEED_FORCE=1 para forzar)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	hash, err := passwords.Hash(claveDemo)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	creadas, existentes := 0, 0

	for _, c := range cuentas {
		verificado := &now
		estado := identity.StatusActive
		switch c.email {
		case "sinverificar@mooc.local":
			verificado = nil
		case "suspendido@mooc.local":
			estado = identity.StatusSuspended
		}

		// ON CONFLICT DO NOTHING: la semilla es idempotente, igual que los
		// trabajos asíncronos (ADR-0008).
		tag, err := pool.Exec(ctx, `
			INSERT INTO identity.users
			    (id, email, email_verified_at, password_hash, full_name, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
			ON CONFLICT (email) DO NOTHING`,
			ids.New(), c.email, verificado, hash, c.nombre, c.rol, estado, now)
		if err != nil {
			return fmt.Errorf("insertar %s: %w", c.email, err)
		}
		if tag.RowsAffected() == 1 {
			creadas++
			log.Info("cuenta creada", "email", c.email, "rol", c.rol, "para", c.descripc)
		} else {
			existentes++
		}
	}

	log.Info("semilla lista", "creadas", creadas, "ya_existían", existentes)

	fmt.Println()
	fmt.Println("Cuentas sintéticas — contraseña única:", claveDemo)
	fmt.Println()
	fmt.Printf("  %-26s %-9s %s\n", "CORREO", "ROL", "PARA QUÉ")
	for _, c := range cuentas {
		fmt.Printf("  %-26s %-9s %s\n", c.email, c.rol, c.descripc)
	}
	fmt.Println()
	return nil
}
