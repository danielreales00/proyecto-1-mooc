// Package almacenes elige la implementación del almacén de objetos. La usan la
// API, el worker y la función de insignias, y antes estaba copiada en cada
// arranque.
package almacenes

import (
	"context"
	"fmt"
	"strings"

	"mooc.local/backend/internal/adapters/gcs"
	"mooc.local/backend/internal/adapters/objectstore"
	"mooc.local/backend/internal/platform/config"
)

// Abrir es el único sitio donde el proveedor se nombra. `s3` cubre MinIO en
// Compose y cualquier almacén compatible. `gcs` es el adaptador nativo de
// Cloud Storage (ADR-0015, D1), que se autentica con la identidad del servicio
// y firma sin clave descargada.
//
// Falla al arrancar y no a mitad de una subida: un almacén mal configurado
// tiene que notarse en el despliegue.
func Abrir(cfg config.Config) (objectstore.Almacen, error) {
	switch strings.ToLower(cfg.ObjectStore) {
	case "", "s3":
		return objectstore.Open(cfg.S3Endpoint, cfg.S3PublicEndpoint, cfg.S3AccessKey,
			cfg.S3SecretKey, cfg.S3UseSSL, cfg.S3Buckets.Originals)
	case "gcs":
		return gcs.Open(context.Background(), cfg.GCSSigner, cfg.S3Buckets.Originals)
	default:
		return nil, fmt.Errorf("OBJECT_STORE=%q no es una opción válida (s3, gcs)", cfg.ObjectStore)
	}
}
