package objectstore_test

import (
	"context"
	"os"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"mooc/backend/internal/adapters/objectstore"
	"mooc/backend/internal/adapters/objectstore/contrato"
)

// La misma suite que pasa el adaptador de GCS, contra MinIO: el contrato es
// uno y lo cumplen los dos. Necesita el MinIO de Compose; sin él se salta.
func TestContrato(t *testing.T) {
	endpoint := os.Getenv("S3_PRUEBA_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_PRUEBA_ENDPOINT vacío: la suite necesita un MinIO real")
	}
	acceso, secreto := os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY")
	bucket, cuarentena := os.Getenv("S3_BUCKET_ORIGINALS"), os.Getenv("S3_BUCKET_QUARANTINE")

	s, err := objectstore.Open(endpoint, "", acceso, secreto, false, bucket)
	if err != nil {
		t.Fatal(err)
	}
	cliente, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(acceso, secreto, "")})
	if err != nil {
		t.Fatal(err)
	}

	contrato.Probar(t, s, contrato.Config{
		Bucket:     bucket,
		Cuarentena: cuarentena,
		// S3 exige 5 MiB en toda parte salvo la última.
		TamParte: 5 << 20,
		Partes:   3,
		Borrar: func(ctx context.Context, b, k string) error {
			return cliente.RemoveObject(ctx, b, k, minio.RemoveObjectOptions{})
		},
	})
}
