package gcs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	"mooc/backend/internal/adapters/objectstore"
)

// Cloud Storage no tiene carga multipart con URL firmadas por parte: su carga
// reanudable es una sola sesión secuencial, que no deja subir partes en
// paralelo ni preguntar cuáles faltan. Se emula así (ADR-0015, D1):
//
//	tmp/{uploadID}/meta          marcador de la carga: destino y tipo MIME
//	tmp/{uploadID}/part-00001    cada parte, subida con su propia URL firmada
//	tmp/{uploadID}/n0-00000      intermedios de compose, si hay más de 32
//
// Al completar, las partes se unen con `compose`, que combina objetos del
// mismo bucket en el servidor, sin descargarlos. Por eso las partes viven en
// el bucket de destino y no en uno aparte. Al terminar se borra todo tmp/; si
// una carga no se completa nunca, lo limpia la regla de ciclo de vida del
// bucket a los siete días (infra/modules/gcs).
//
// El contrato de la API no cambia: una URL por parte, reanudación preguntando
// qué partes hay (RF-05) y ETag por parte al completar.

const (
	// Límite de fuentes de una sola operación compose.
	maxFuentes = 32
	// Límite de componentes de un objeto compuesto. Con partes de 8 MiB son
	// 8 GiB, por encima del tope de 5 GiB del ADR-0011.
	maxComponentes = 1024
	metaDestino    = "destino"
)

// idValido evita que un uploadID manipulado se salga de tmp/{uploadID}/.
var idValido = regexp.MustCompile(`^[0-9a-f]{32}$`)

// ErrCargaDesconocida equivale al NoSuchUpload de S3: no hay carga abierta con
// ese identificador para esa clave.
var ErrCargaDesconocida = errors.New("no hay una carga abierta con ese identificador")

func prefijo(id string) string   { return "tmp/" + id + "/" }
func claveMeta(id string) string { return prefijo(id) + "meta" }
func clavePart(id string, n int) string {
	return fmt.Sprintf("%spart-%05d", prefijo(id), n)
}

// CrearMultipart abre la carga: deja el marcador con el destino y el tipo, que
// hace falta al componer, y devuelve el identificador.
func (s *Store) CrearMultipart(ctx context.Context, bucket, key, contentType string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generar el identificador de carga: %w", err)
	}
	id := hex.EncodeToString(b)

	w := s.client.Bucket(bucket).Object(claveMeta(id)).NewWriter(ctx)
	w.ContentType = contentType
	w.Metadata = map[string]string{metaDestino: key}
	w.ChunkSize = 0
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("abrir la carga en %s/%s: %w", bucket, key, err)
	}
	return id, nil
}

// abierta comprueba que la carga exista y sea de esa clave, y devuelve el tipo
// MIME con que se abrió.
func (s *Store) abierta(ctx context.Context, bucket, key, id string) (string, error) {
	if !idValido.MatchString(id) {
		return "", ErrCargaDesconocida
	}
	a, err := s.client.Bucket(bucket).Object(claveMeta(id)).Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return "", ErrCargaDesconocida
	}
	if err != nil {
		return "", fmt.Errorf("consultar la carga %s: %w", id, err)
	}
	if a.Metadata[metaDestino] != key {
		return "", ErrCargaDesconocida
	}
	return a.ContentType, nil
}

// PresignPart firma la subida de UNA parte como objeto propio. El cliente sube
// directamente al almacén: los bytes no pasan por la API (enunciado §4).
//
// No consulta el marcador: firmar es local salvo la llamada a signBlob, y
// hacerlo por cada parte multiplicaría las lecturas. Una URL de una carga
// inexistente sube una parte huérfana que nadie compone y que borra el ciclo
// de vida.
func (s *Store) PresignPart(_ context.Context, bucket, key, uploadID string,
	parte int, ttl time.Duration) (string, error) {

	if !idValido.MatchString(uploadID) {
		return "", ErrCargaDesconocida
	}
	if parte < 1 || parte > maxComponentes {
		return "", fmt.Errorf("parte %d fuera de rango (1..%d)", parte, maxComponentes)
	}
	u, err := s.firmar(bucket, clavePart(uploadID, parte), http.MethodPut, ttl)
	if err != nil {
		return "", fmt.Errorf("firmar la parte %d: %w", parte, err)
	}
	return u, nil
}

// PartesSubidas pregunta al almacén qué partes tiene ya. Es lo que permite
// reanudar: el cliente sube solo lo que falta (RF-05).
//
// El ETag es el MD5 en hexadecimal, sin comillas: el mismo valor que devuelve
// GCS en la cabecera ETag al subir la parte con la URL firmada, y el mismo
// formato que da el adaptador de S3.
func (s *Store) PartesSubidas(ctx context.Context, bucket, key, uploadID string) ([]objectstore.Parte, error) {
	if _, err := s.abierta(ctx, bucket, key, uploadID); err != nil {
		return nil, err
	}
	partes, err := s.listarPartes(ctx, bucket, uploadID)
	if err != nil {
		return nil, err
	}
	out := make([]objectstore.Parte, 0, len(partes))
	for _, p := range partes {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Numero < out[j].Numero })
	return out, nil
}

func (s *Store) listarPartes(ctx context.Context, bucket, id string) (map[int]objectstore.Parte, error) {
	it := s.client.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: prefijo(id) + "part-"})
	out := map[int]objectstore.Parte{}
	for {
		a, err := it.Next()
		if errors.Is(err, iterator.Done) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("listar partes de %s: %w", id, err)
		}
		n, err := strconv.Atoi(strings.TrimPrefix(a.Name, prefijo(id)+"part-"))
		if err != nil {
			continue // no es una parte nuestra
		}
		out[n] = objectstore.Parte{Numero: n, ETag: hex.EncodeToString(a.MD5), Bytes: a.Size}
	}
}

// CompletarMultipart une las partes en el objeto final y limpia tmp/.
//
// Como S3, exige que cada parte pedida exista y que su ETag coincida: si el
// cliente cree haber subido algo distinto de lo que hay, la carga no se cierra
// con un contenido que nadie pidió.
func (s *Store) CompletarMultipart(ctx context.Context, bucket, key, uploadID string,
	partes []objectstore.Parte) error {

	contentType, err := s.abierta(ctx, bucket, key, uploadID)
	if err != nil {
		return err
	}
	if len(partes) == 0 {
		return errors.New("completar sin partes")
	}
	if len(partes) > maxComponentes {
		return fmt.Errorf("%d partes: el máximo de un objeto compuesto es %d", len(partes), maxComponentes)
	}
	subidas, err := s.listarPartes(ctx, bucket, uploadID)
	if err != nil {
		return err
	}

	pedidas := append([]objectstore.Parte(nil), partes...)
	sort.Slice(pedidas, func(i, j int) bool { return pedidas[i].Numero < pedidas[j].Numero })
	bh := s.client.Bucket(bucket)
	fuentes := make([]*storage.ObjectHandle, 0, len(pedidas))
	for i, p := range pedidas {
		if i > 0 && p.Numero == pedidas[i-1].Numero {
			return fmt.Errorf("la parte %d aparece dos veces", p.Numero)
		}
		sub, ok := subidas[p.Numero]
		if !ok {
			return fmt.Errorf("la parte %d no se ha subido", p.Numero)
		}
		if strings.Trim(p.ETag, `"`) != sub.ETag {
			return fmt.Errorf("la parte %d no coincide con su ETag", p.Numero)
		}
		fuentes = append(fuentes, bh.Object(clavePart(uploadID, p.Numero)))
	}

	// Más de 32 partes: se componen por niveles. Con el tope de 1024 son a lo
	// sumo dos niveles de intermedios.
	for nivel := 0; len(fuentes) > maxFuentes; nivel++ {
		siguientes := make([]*storage.ObjectHandle, 0, (len(fuentes)+maxFuentes-1)/maxFuentes)
		for i := 0; i < len(fuentes); i += maxFuentes {
			fin := min(i+maxFuentes, len(fuentes))
			dst := bh.Object(fmt.Sprintf("%sn%d-%05d", prefijo(uploadID), nivel, i/maxFuentes))
			if err := componer(ctx, dst, fuentes[i:fin], ""); err != nil {
				return err
			}
			siguientes = append(siguientes, dst)
		}
		fuentes = siguientes
	}
	if err := componer(ctx, bh.Object(key), fuentes, contentType); err != nil {
		return err
	}

	// La carga ya está completa; si la limpieza falla, lo que quede lo borra
	// el ciclo de vida. No se devuelve error por eso.
	_ = s.borrarPrefijo(ctx, bucket, prefijo(uploadID))
	return nil
}

// componer reintenta con backoff exponencial, incluido rateLimitExceeded, que
// aparece al componer grupos en ráfaga. Reintentar siempre es seguro aquí: las
// mismas fuentes sobre el mismo destino dan el mismo objeto.
func componer(ctx context.Context, dst *storage.ObjectHandle, fuentes []*storage.ObjectHandle, contentType string) error {
	c := dst.Retryer(storage.WithPolicy(storage.RetryAlways)).ComposerFrom(fuentes...)
	if contentType != "" {
		c.ContentType = contentType
	}
	if _, err := c.Run(ctx); err != nil {
		return fmt.Errorf("componer %s: %w", dst.ObjectName(), err)
	}
	return nil
}

// AbortarMultipart descarta una carga a medias: borra el marcador y las
// partes.
func (s *Store) AbortarMultipart(ctx context.Context, bucket, key, uploadID string) error {
	if _, err := s.abierta(ctx, bucket, key, uploadID); err != nil {
		return err
	}
	return s.borrarPrefijo(ctx, bucket, prefijo(uploadID))
}

func (s *Store) borrarPrefijo(ctx context.Context, bucket, p string) error {
	bh := s.client.Bucket(bucket)
	it := bh.Objects(ctx, &storage.Query{Prefix: p})
	var primero error
	for {
		a, err := it.Next()
		if errors.Is(err, iterator.Done) {
			return primero
		}
		if err != nil {
			return fmt.Errorf("listar %s: %w", p, err)
		}
		if err := bh.Object(a.Name).Delete(ctx); err != nil &&
			!errors.Is(err, storage.ErrObjectNotExist) && primero == nil {
			primero = fmt.Errorf("borrar %s: %w", a.Name, err)
		}
	}
}
