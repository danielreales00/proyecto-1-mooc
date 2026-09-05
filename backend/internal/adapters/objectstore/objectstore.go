// Package objectstore implementa el puerto de almacenamiento de objetos sobre
// el protocolo S3, que sirve tanto para MinIO en local como para Cloud Storage
// en GCP (ADR-0005, ADR-0010).
package objectstore

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Store struct {
	client *minio.Client
	// Bucket de referencia para la comprobación de readiness.
	probeBucket string
}

func Open(endpoint, accessKey, secretKey string, useSSL bool, probeBucket string) (*Store, error) {
	c, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("cliente de objetos: %w", err)
	}
	return &Store{client: c, probeBucket: probeBucket}, nil
}

func (s *Store) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ok, err := s.client.BucketExists(ctx, s.probeBucket)
	if err != nil {
		return fmt.Errorf("almacén de objetos: %w", err)
	}
	if !ok {
		return fmt.Errorf("el bucket %q no existe", s.probeBucket)
	}
	return nil
}

// PresignGet emite una URL firmada de lectura. Se llama SIEMPRE después de
// verificar rol, propiedad e inscripción (CA-06).
func (s *Store) PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, bucket, key, ttl, nil)
	if err != nil {
		return "", fmt.Errorf("firmar lectura de %s/%s: %w", bucket, key, err)
	}
	return u.String(), nil
}
