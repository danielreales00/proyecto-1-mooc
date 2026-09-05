# ADR-0005 — Almacenamiento de objetos compatible con S3, tras un puerto propio

- **Estado:** Aceptado
- **Fecha:** 2026-09-05
- **Requisitos:** `RF-05`, `RF-06`, `RT-01`, `RT-03`, `CA-02`, `CA-06`

## Contexto

Ningún binario puede vivir en PostgreSQL. Los originales, los derivados HLS, los
PDF y las imágenes de insignias van a almacenamiento de objetos. El entorno local
usa MinIO; en entregas futuras el destino es GCP. El dominio debe quedar
desacoplado del proveedor (§7).

## Decisión

Un puerto `objectstore.Store` en el dominio, con una única implementación en
la Entrega 1 (`adapters/objectstore/s3`) sobre `minio-go`, que habla el protocolo
S3 y por tanto sirve tanto para MinIO como para Google Cloud Storage en su modo
de interoperabilidad XML.

```go
type Store interface {
    PresignPut(ctx, key string, ttl time.Duration, ct string) (string, error)
    PresignGet(ctx, key string, ttl time.Duration, filename string) (string, error)
    CreateMultipart(ctx, key, contentType string) (uploadID string, err error)
    PresignPart(ctx, key, uploadID string, part int, ttl time.Duration) (string, error)
    CompleteMultipart(ctx, key, uploadID string, parts []Part) (etag string, err error)
    AbortMultipart(ctx, key, uploadID string) error
    Stat(ctx, key string) (ObjectInfo, error)
    Get(ctx, key string) (io.ReadCloser, error)
    Put(ctx, key string, r io.Reader, size int64, ct string) error
    Delete(ctx, key string) error
}
```

**Buckets:**

| Bucket | Contenido | Acceso |
| --- | --- | --- |
| `mooc-originals` | Archivo tal como lo subió el profesor | Privado siempre |
| `mooc-derived` | HLS (`.m3u8`, `.ts`), pósteres, PDF convertidos | Privado, servido por URL firmada o CDN |
| `mooc-badges` | Imágenes PNG de insignias | Público de lectura: la URL de verificación es pública (`CA-07`) |
| `mooc-quarantine` | Cargas que fallaron el escaneo antimalware | Privado, retención 30 días |

**Convención de claves** — determinista, para que un reintento escriba en el
mismo sitio y la operación sea idempotente:

```
originals/{course_id}/{asset_id}/original{ext}
derived/{asset_id}/hls/{ladder}/index.m3u8
derived/{asset_id}/hls/master.m3u8
derived/{asset_id}/poster.jpg
badges/{badge_public_code}.png
```

**Reglas:**

- Los binarios **nunca pasan por la API**. Subida: URL prefirmada `PUT` (o
  multipart prefirmado). Bajada: URL prefirmada `GET` emitida *después* de
  verificar rol, propiedad e inscripción (`CA-06`).
- TTL de las URL firmadas: **15 min** para lectura de material de curso, **24 h**
  para las partes de una carga multipart en curso (`RF-05` exige reanudación 24 h).
- El original **nunca se borra ni se sobrescribe** al transcodificar (`CA-02`).
- Cifrado en reposo: SSE-S3 en MinIO; CMEK en GCS cuando toque (`RT-06`).
- Ciclo de vida: multipart abortadas se purgan a las 24 h; `mooc-quarantine` a
  los 30 días.

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| SDK de GCS directamente | Nativo del destino final | No sirve en local sin emulador; acopla al proveedor | Rompe `RT-01` |
| `aws-sdk-go-v2` | Estándar de facto | Mucho más pesado que `minio-go` para lo que usamos | Peso |
| Subir a través de la API | Un solo camino, validación central | La API deja de ser sin estado en la práctica y se convierte en cuello de botella | El enunciado exige carga directa (§4) |
| Un solo bucket con prefijos | Menos configuración | Políticas de acceso y ciclo de vida distintas por tipo | Las insignias son públicas y los originales no |

## Consecuencias

- Cambiar a GCS en E3 es implementar el mismo puerto (o reusar el adaptador S3
  contra el endpoint de interoperabilidad de GCS) y cambiar variables de entorno.
- Las URL firmadas caducan; el cliente debe pedirlas de nuevo, lo que se
  documenta en el OpenAPI y en la colección de Postman.
- Servir HLS por URL firmada obliga a firmar el manifiesto **y** los segmentos, o
  a usar una cookie firmada. En E1 se firma cada segmento vía un endpoint de
  redirección; en E3, con CDN, se pasa a cookie firmada.

## Cómo se verifica

- Un `GET` directo al bucket sin firma responde `403`.
- Tras transcodificar, el objeto en `mooc-originals` conserva el mismo ETag.
- URL firmada expirada responde `403` (prueba con reloj adelantado).
