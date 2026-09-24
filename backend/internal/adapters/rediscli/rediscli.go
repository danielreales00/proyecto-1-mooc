// Package rediscli abre los clientes de Redis. Cada uso tiene su base lógica:
// no se comparte espacio de claves entre cola, sesiones, caché y límites
// (ADR-0004).
package rediscli

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Open abre un cliente contra una base lógica.
//
// `useTLS` lo exige Memorystore cuando se activa el cifrado en tránsito; en
// Compose va apagado, porque Redis no sale del entorno de Docker. Es
// configuración, no una rama de código por proveedor (ADR-0010).
func Open(ctx context.Context, addr, password string, db int, useTLS bool) (*redis.Client, error) {
	opciones := &redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
	if useTLS {
		// Sin InsecureSkipVerify: si el certificado del servidor no valida, la
		// conexión debe fallar. Un TLS que no verifica no es TLS.
		opciones.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: hostDe(addr)}
	}
	c := redis.NewClient(opciones)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("ping a redis db %d: %w", db, err)
	}
	return c, nil
}

// hostDe recorta el puerto: el nombre del certificado es el host, no host:puerto.
func hostDe(addr string) string {
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return addr[:i]
	}
	return addr
}
