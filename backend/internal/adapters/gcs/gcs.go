// Package gcs implementa el puerto de almacenamiento de objetos sobre la API
// nativa de Cloud Storage (ADR-0015, D1).
//
// No usa la interoperabilidad S3 de GCS porque exige claves HMAC, que son la
// credencial estática de larga vida que ADR-0015 (D3) prohíbe. El cliente se
// autentica con las credenciales por defecto de la aplicación: la identidad de
// la máquina en GCP, el ADC de usuario en la máquina de quien desarrolla.
//
// Lo que S3 da hecho y GCS no, la carga por partes con URL firmadas, se emula
// con `compose`; el detalle está en multipart.go.
package gcs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	"mooc/backend/internal/adapters/objectstore"
)

type Store struct {
	client *storage.Client
	// firmante es la cuenta de servicio con la que se firman las URL. Vacío en
	// una máquina de GCP: la librería toma la de la instancia del servidor de
	// metadatos. Hay que darlo cuando firma alguien que no es una cuenta de
	// servicio —el ADC de usuario de quien desarrolla—, y entonces esa persona
	// necesita serviceAccountTokenCreator sobre la cuenta.
	firmante string
	// Bucket de referencia para la comprobación de readiness.
	probeBucket string
}

// Open abre el cliente con las credenciales por defecto de la aplicación.
func Open(ctx context.Context, firmante, probeBucket string) (*Store, error) {
	c, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("cliente de Cloud Storage: %w", err)
	}
	return &Store{client: c, firmante: firmante, probeBucket: probeBucket}, nil
}

func (s *Store) Close() error { return s.client.Close() }

// Ping lista a lo sumo un objeto del bucket de referencia. No consulta los
// atributos del bucket: eso pide storage.buckets.get, que las cuentas de las
// máquinas no tienen, porque solo necesitan los objetos.
func (s *Store) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	it := s.client.Bucket(s.probeBucket).Objects(ctx, &storage.Query{Prefix: "readyz/"})
	if _, err := it.Next(); err != nil && !errors.Is(err, iterator.Done) {
		return fmt.Errorf("almacén de objetos: %w", err)
	}
	return nil
}

// firmar emite una URL V4 sin clave privada: la librería llama a signBlob de
// IAM Credentials con la identidad del cliente. Es lo que exige que la cuenta
// firmante sea creadora de tokens sobre sí misma.
func (s *Store) firmar(bucket, key, metodo string, ttl time.Duration) (string, error) {
	return s.client.Bucket(bucket).SignedURL(key, &storage.SignedURLOptions{
		Scheme:         storage.SigningSchemeV4,
		Method:         metodo,
		Expires:        time.Now().Add(ttl),
		GoogleAccessID: s.firmante,
	})
}

// PresignGet emite una URL firmada de lectura. Se llama SIEMPRE después de
// verificar rol, propiedad e inscripción (CA-06).
func (s *Store) PresignGet(_ context.Context, bucket, key string, ttl time.Duration) (string, error) {
	u, err := s.firmar(bucket, key, http.MethodGet, ttl)
	if err != nil {
		return "", fmt.Errorf("firmar lectura de %s/%s: %w", bucket, key, err)
	}
	return u, nil
}

// Abrir devuelve el contenido del objeto. Lo usa el worker para recalcular el
// checksum y detectar el MIME real: la palabra del cliente no basta.
func (s *Store) Abrir(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	r, err := s.client.Bucket(bucket).Object(key).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("abrir %s/%s: %w", bucket, key, err)
	}
	return r, nil
}

// Info devuelve el tamaño del objeto.
func (s *Store) Info(ctx context.Context, bucket, key string) (int64, error) {
	a, err := s.client.Bucket(bucket).Object(key).Attrs(ctx)
	if err != nil {
		return 0, fmt.Errorf("consultar %s/%s: %w", bucket, key, err)
	}
	return a.Size, nil
}

// Subir escribe un objeto pequeño de una sola vez. Para los archivos que sube
// un usuario se usa la carga por partes; esto es para lo que genera el
// servidor.
func (s *Store) Subir(ctx context.Context, bucket, key, contentType string, datos []byte) error {
	w := s.client.Bucket(bucket).Object(key).NewWriter(ctx)
	w.ContentType = contentType
	// De una vez, sin sesión reanudable: son objetos pequeños.
	w.ChunkSize = 0
	if _, err := io.Copy(w, bytes.NewReader(datos)); err != nil {
		_ = w.Close()
		return fmt.Errorf("subir %s/%s: %w", bucket, key, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("subir %s/%s: %w", bucket, key, err)
	}
	return nil
}

// Mover copia el objeto y borra el original: GCS no tiene renombrado. Se usa
// para llevar a cuarentena lo que no supere una comprobación. La copia es del
// lado del servidor; los bytes no pasan por la máquina.
func (s *Store) Mover(ctx context.Context, origenBucket, origenKey, destinoBucket, destinoKey string) error {
	origen := s.client.Bucket(origenBucket).Object(origenKey)
	destino := s.client.Bucket(destinoBucket).Object(destinoKey)
	if _, err := destino.CopierFrom(origen).Run(ctx); err != nil {
		return fmt.Errorf("copiar %s/%s a %s/%s: %w", origenBucket, origenKey,
			destinoBucket, destinoKey, err)
	}
	if err := origen.Delete(ctx); err != nil {
		return fmt.Errorf("borrar %s/%s tras copiarlo: %w", origenBucket, origenKey, err)
	}
	return nil
}

// Comprobación en tiempo de compilación: el mismo contrato que el adaptador
// de S3.
var _ objectstore.Almacen = (*Store)(nil)
