// Package objectstore implementa el puerto de almacenamiento de objetos sobre
// el protocolo S3, que sirve tanto para MinIO en local como para Cloud Storage
// en GCP (ADR-0005, ADR-0010).
package objectstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Store struct {
	client   *minio.Client
	endpoint string
	opciones *minio.Options
	// firmante usa el endpoint PÚBLICO. Las URLs firmadas las abre un cliente
	// de fuera de la red de Compose, donde `minio:9000` no resuelve. Es el
	// mismo caso que en GCP: se firma contra el endpoint público.
	firmante *minio.Client
	// Bucket de referencia para la comprobación de readiness.
	probeBucket string
}

// region es la que MinIO usa por defecto. Solo interviene en el cálculo de la
// firma V4; no implica nada sobre dónde vive el dato.
const region = "us-east-1"

func Open(endpoint, publico, accessKey, secretKey string, useSSL bool, probeBucket string) (*Store, error) {
	opciones := &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	}
	c, err := minio.New(endpoint, opciones)
	if err != nil {
		return nil, fmt.Errorf("cliente de objetos: %w", err)
	}

	firmante := c
	if publico != "" && publico != endpoint {
		// La región se fija a propósito: sin ella, minio-go consulta la
		// ubicación del bucket contra el endpoint público, y ese nombre no
		// resuelve DESDE DENTRO del contenedor. Firmar no necesita esa
		// consulta; solo necesita la región para calcular la firma V4.
		opcionesFirma := &minio.Options{
			Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure: useSSL,
			Region: region,
		}
		firmante, err = minio.New(publico, opcionesFirma)
		if err != nil {
			return nil, fmt.Errorf("cliente de firma: %w", err)
		}
	}
	return &Store{client: c, endpoint: endpoint, opciones: opciones,
		firmante: firmante, probeBucket: probeBucket}, nil
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
	u, err := s.firmante.PresignedGetObject(ctx, bucket, key, ttl, nil)
	if err != nil {
		return "", fmt.Errorf("firmar lectura de %s/%s: %w", bucket, key, err)
	}
	return u.String(), nil
}

// Parte es una parte subida de una carga multipart.
type Parte struct {
	Numero int
	ETag   string
	Bytes  int64
}

// CrearMultipart abre una carga multipart y devuelve su identificador.
func (s *Store) CrearMultipart(ctx context.Context, bucket, key, contentType string) (string, error) {
	c, err := minio.NewCore(s.endpoint, s.opciones)
	if err != nil {
		return "", fmt.Errorf("cliente multipart: %w", err)
	}
	id, err := c.NewMultipartUpload(ctx, bucket, key,
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", fmt.Errorf("abrir multipart en %s/%s: %w", bucket, key, err)
	}
	return id, nil
}

// PresignPart firma la subida de UNA parte. El cliente sube directamente al
// almacén: los bytes no pasan por la API (enunciado §4).
func (s *Store) PresignPart(ctx context.Context, bucket, key, uploadID string,
	parte int, ttl time.Duration) (string, error) {

	q := url.Values{}
	q.Set("uploadId", uploadID)
	q.Set("partNumber", strconv.Itoa(parte))
	u, err := s.firmante.Presign(ctx, http.MethodPut, bucket, key, ttl, q)
	if err != nil {
		return "", fmt.Errorf("firmar la parte %d: %w", parte, err)
	}
	return u.String(), nil
}

// PartesSubidas pregunta al almacén qué partes tiene ya. Es lo que permite
// reanudar: el cliente sube solo lo que falta (RF-05).
func (s *Store) PartesSubidas(ctx context.Context, bucket, key, uploadID string) ([]Parte, error) {
	c, err := minio.NewCore(s.endpoint, s.opciones)
	if err != nil {
		return nil, err
	}
	var out []Parte
	marcador := 0
	for {
		res, err := c.ListObjectParts(ctx, bucket, key, uploadID, marcador, 1000)
		if err != nil {
			return nil, fmt.Errorf("listar partes de %s: %w", uploadID, err)
		}
		for _, p := range res.ObjectParts {
			out = append(out, Parte{Numero: p.PartNumber, ETag: p.ETag, Bytes: p.Size})
		}
		if !res.IsTruncated {
			break
		}
		marcador = res.NextPartNumberMarker
	}
	return out, nil
}

// CompletarMultipart cierra la carga y deja el objeto final.
func (s *Store) CompletarMultipart(ctx context.Context, bucket, key, uploadID string, partes []Parte) error {
	c, err := minio.NewCore(s.endpoint, s.opciones)
	if err != nil {
		return err
	}
	sort.Slice(partes, func(i, j int) bool { return partes[i].Numero < partes[j].Numero })
	completas := make([]minio.CompletePart, 0, len(partes))
	for _, p := range partes {
		completas = append(completas, minio.CompletePart{PartNumber: p.Numero, ETag: p.ETag})
	}
	if _, err := c.CompleteMultipartUpload(ctx, bucket, key, uploadID, completas,
		minio.PutObjectOptions{}); err != nil {
		return fmt.Errorf("completar multipart %s: %w", uploadID, err)
	}
	return nil
}

// AbortarMultipart descarta una carga a medias y libera las partes.
func (s *Store) AbortarMultipart(ctx context.Context, bucket, key, uploadID string) error {
	c, err := minio.NewCore(s.endpoint, s.opciones)
	if err != nil {
		return err
	}
	return c.AbortMultipartUpload(ctx, bucket, key, uploadID)
}

// Abrir devuelve el contenido del objeto. Lo usa el worker para recalcular el
// checksum y detectar el MIME real: la palabra del cliente no basta.
func (s *Store) Abrir(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	o, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("abrir %s/%s: %w", bucket, key, err)
	}
	return o, nil
}

// Info devuelve el tamaño y los metadatos del objeto.
func (s *Store) Info(ctx context.Context, bucket, key string) (int64, error) {
	i, err := s.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return 0, fmt.Errorf("consultar %s/%s: %w", bucket, key, err)
	}
	return i.Size, nil
}

// Mover copia el objeto a otro bucket y borra el original. Se usa para llevar
// a cuarentena lo que no supere una comprobación.
func (s *Store) Mover(ctx context.Context, origenBucket, origenKey, destinoBucket, destinoKey string) error {
	_, err := s.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: destinoBucket, Object: destinoKey},
		minio.CopySrcOptions{Bucket: origenBucket, Object: origenKey})
	if err != nil {
		return fmt.Errorf("copiar a cuarentena: %w", err)
	}
	return s.client.RemoveObject(ctx, origenBucket, origenKey, minio.RemoveObjectOptions{})
}
