package gcs

import (
	"context"
	"os"
	"testing"

	"mooc/backend/internal/adapters/objectstore/contrato"
)

// La suite corre contra Cloud Storage real: la parte delicada —firmar sin clave
// descargada— no se puede comprobar en Compose. Sin las variables se salta,
// así que `make ci` no necesita GCP. Se lanza con `make prueba-gcs`.
func TestContrato(t *testing.T) {
	bucket := os.Getenv("GCS_PRUEBA_BUCKET")
	if bucket == "" {
		t.Skip("GCS_PRUEBA_BUCKET vacío: la suite de GCS necesita un bucket real (make prueba-gcs)")
	}
	ctx := context.Background()
	s, err := Open(ctx, os.Getenv("GCS_SIGNER"), bucket)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	contrato.Probar(t, s, contrato.Config{
		Bucket:     bucket,
		Cuarentena: os.Getenv("GCS_PRUEBA_CUARENTENA"),
		// 40 partes pequeñas: más de 32 obliga a componer por niveles.
		TamParte: 64 << 10,
		Partes:   40,
		Borrar: func(ctx context.Context, b, k string) error {
			return s.client.Bucket(b).Object(k).Delete(ctx)
		},
	})
}
