// Comando buckets: deja el almacén de objetos con los cuatro buckets del
// ADR-0005 y la política pública del de insignias.
//
// Antes lo hacía un contenedor de `mc`, el cliente de MinIO. Se trajo aquí
// cuando MinIO cerró el acceso anónimo a sus imágenes —primero en Docker Hub y
// después en quay.io— y el CI dejó de poder descargarlas. Con el cliente de S3
// que el backend ya usa, esto deja de depender de que alguien siga publicando
// una imagen: no añade dependencias y funciona contra cualquier almacén
// compatible, que es también lo que hará falta al salir a la nube.
//
// Es idempotente por diseño: se ejecuta en cada arranque del stack y no hace
// nada si los buckets ya están.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"mooc/backend/internal/platform/config"
	"mooc/backend/internal/platform/logging"
)

// politicaLecturaPublica permite descargar los objetos de un bucket sin firma.
//
// La lleva solo el de insignias: verificar una insignia es público (CA-07), y
// la imagen tiene que poder abrirse desde cualquier parte sin credenciales. Los
// otros tres siguen cerrados.
const politicaLecturaPublica = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": ["*"]},
      "Action": ["s3:GetObject"],
      "Resource": ["arn:aws:s3:::%s/*"]
    }
  ]
}`

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "buckets:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.LogLevel, cfg.LogFormat)

	// En Cloud Storage los buckets, su ciclo de vida y la lectura pública de
	// insignias los crea Terraform (infra/modules/gcs), y las cuentas de las
	// máquinas no tienen permiso para crear buckets. No hay nada que hacer.
	if cfg.ObjectStore == "gcs" {
		log.Info("OBJECT_STORE=gcs: los buckets los gestiona Terraform")
		return nil
	}

	cliente, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
	})
	if err != nil {
		return fmt.Errorf("abrir el cliente de objetos: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// El servicio arranca con su healthcheck, pero entre «sano» y «acepta
	// peticiones con firma» hay unos segundos. Se espera en vez de fallar.
	if err := esperar(ctx, cliente, log); err != nil {
		return err
	}

	for _, bucket := range []string{
		cfg.S3Buckets.Originals,
		cfg.S3Buckets.Derived,
		cfg.S3Buckets.Badges,
		cfg.S3Buckets.Quarantine,
	} {
		existe, err := cliente.BucketExists(ctx, bucket)
		if err != nil {
			return fmt.Errorf("comprobar el bucket %s: %w", bucket, err)
		}
		if !existe {
			if err := cliente.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				return fmt.Errorf("crear el bucket %s: %w", bucket, err)
			}
			log.Info("bucket creado", "bucket", bucket)
			continue
		}
		log.Info("el bucket ya existía", "bucket", bucket)
	}

	badges := cfg.S3Buckets.Badges
	if err := cliente.SetBucketPolicy(ctx, badges,
		fmt.Sprintf(politicaLecturaPublica, badges)); err != nil {
		return fmt.Errorf("abrir la lectura pública de %s: %w", badges, err)
	}
	log.Info("lectura pública fijada", "bucket", badges)

	log.Info("buckets listos")
	return nil
}

// esperar reintenta hasta que el almacén responde.
func esperar(ctx context.Context, cliente *minio.Client, log *slog.Logger) error {
	var ultimo error
	for intento := 1; intento <= 30; intento++ {
		if _, err := cliente.ListBuckets(ctx); err == nil {
			return nil
		} else {
			ultimo = err
		}
		if intento == 1 {
			log.Info("esperando al almacén de objetos")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("el almacén de objetos no respondió: %w", ultimo)
}
