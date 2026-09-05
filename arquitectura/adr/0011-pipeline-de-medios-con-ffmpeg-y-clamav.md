# ADR-0011 — Pipeline de medios: carga multipart, ClamAV y HLS con FFmpeg

- **Estado:** Aceptado
- **Fecha:** 2026-09-05
- **Requisitos:** `RF-05`, `RF-06`, `CA-02`, `CE-04`, `RNF-07`

## Contexto

`RF-05` pide carga multipart directa a objetos, reanudable durante 24 h, con
verificación de integridad, MIME real y escaneo antimalware. `RF-06` pide
transcodificación asíncrona a HLS conservando el original. SEG-3 y SEG-4 exigen
demostrar una carga interrumpida y reanudada, y un procesamiento con fallo
inyectado.

## Decisión

### Carga

Multipart de S3, con URLs prefirmadas por parte. La API nunca recibe bytes.

```
POST /assets:init            → asset_id, upload_id, tamaño de parte, URLs por parte
PUT  <url prefirmada>        → el cliente sube cada parte directamente (repetible)
GET  /assets/{id}/upload     → partes ya recibidas (esto es lo que permite reanudar)
POST /assets/{id}:complete   → la API valida y completa la multipart
```

- Tamaño de parte: 8 MiB. Máximo 5 GiB por archivo.
- Las URL de parte se firman a **24 h** (`RNF-07`). Reanudar es pedir de nuevo
  `GET /assets/{id}/upload` y subir solo lo que falta.
- El cliente declara `sha256` y tamaño en `:init`. En `:complete` la API compara
  el tamaño real y encola `media.probe`, que **recalcula el SHA-256 leyendo del
  bucket**; si no coincide, el asset pasa a `rejected`. La integridad la verifica
  el servidor, no la promesa del cliente.
- El **MIME real** se determina por *magic bytes* (`http.DetectContentType` más
  una tabla propia para MP4/MKV/PDF), nunca por la extensión ni por el
  `Content-Type` declarado. Si el MIME real no está en la lista blanca del tipo
  de recurso, se rechaza.
- Las multipart abortadas y las cargas sin completar se purgan a las 24 h por
  ciclo de vida del bucket y por el worker `media.sweep_uploads`.

### Escaneo antimalware

- **ClamAV** en su propio contenedor (`clamav/clamav`), consultado por el worker
  `media.scan` vía protocolo INSTREAM.
- El asset queda en `scanning` hasta que hay veredicto. Un asset infectado se
  mueve a `mooc-quarantine`, pasa a `infected` y genera evento de auditoría.
- El escaneo ocurre **antes** de la transcodificación: no se procesa lo que no
  está limpio.
- Prueba reproducible con el archivo **EICAR**, que es inofensivo y detectado por
  cualquier antivirus. Va en los datos sintéticos de la demo.

### Transcodificación

- **FFmpeg** invocado con `exec.CommandContext` desde el worker, en una imagen
  propia (`worker-media`) que trae el binario.
- Escalera HLS, **sin *upscaling*** (lo exige SEG-4): se generan solo las
  variantes cuya altura sea ≤ a la del original.

| Variante | Resolución | Video | Audio |
| --- | --- | --- | --- |
| `360p` | 640×360 | 800 kbps | 96 kbps |
| `720p` | 1280×720 | 2.500 kbps | 128 kbps |
| `1080p` | 1920×1080 | 5.000 kbps | 192 kbps |

- Audio puro: una sola variante AAC 128 kbps.
- Segmentos de 6 s, `master.m3u8` con las variantes generadas, póster en el
  segundo 3.
- El original **se conserva intacto** en `mooc-originals` (`CA-02`).
- Salida a claves deterministas (ADR-0005): reintentar sobrescribe con lo mismo.
- Trabajo largo: `heartbeat_at` cada 30 s (ADR-0004) y `context` cancelable, de
  modo que un worker que muere no bloquea el asset.
- Los temporales viven en el `tmpfs` del contenedor y se borran siempre
  (`defer`), incluso al fallar: los workers no acumulan estado (ADR-0001).

### Estados del asset

```
uploading → uploaded → scanning → clean → processing → ready
                 │          │                   │
                 │          └──> infected       └──> failed ──(3 reintentos)──> dead_letter
                 └──> rejected (checksum o MIME)
```

El recurso que referencia el asset expone `processing_status`; un recurso visible
que no esté `ready` bloquea la publicación (`CA-01`, ADR-0007).

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| Subir a través de la API | Validación central, simple | La API se vuelve cuello de botella y deja de ser sin estado en la práctica | §4 exige carga directa |
| `tus` para reanudación | Protocolo pensado para esto | Servidor adicional; S3 multipart ya reanuda | Menos piezas |
| Servicio gestionado de transcodificación | Sin CPU propia | No existe en local; costo; el enunciado sugiere medir antes de decidir (§11) | Se mide con FFmpeg y se decide en E3 |
| Escaneo con API externa (VirusTotal) | Sin contenedor extra | Depende de red y cuota; envía contenido a terceros | ClamAV local es reproducible |
| Transcodificar con `ffmpeg-go` | API en Go | Envuelve el mismo binario y añade dependencia | `exec` directo es más transparente |

## Consecuencias

- FFmpeg consume CPU: los workers de medios se escalan por separado y viven en la
  cola `bulk` (ADR-0004), para no ahogar los correos ni las insignias.
- La imagen `worker-media` pesa (~400 MB con FFmpeg); se separa de la imagen de la
  API para que esta siga siendo ligera.
- ClamAV tarda ~1 min en arrancar y descargar firmas; el `depends_on` con
  healthcheck evita fallos falsos al levantar el stack.
- `job_duration_seconds{type="media.transcode_hls"}` da el costo por minuto que
  el enunciado (§11) recomienda medir desde el MVP.

## Cómo se verifica

- SEG-3: subir un archivo, cortar a media carga, consultar `GET /assets/{id}/upload`
  y completar solo las partes faltantes.
- Subir EICAR → `infected`, objeto en cuarentena, evento de auditoría.
- Declarar un `sha256` falso → `rejected`.
- Subir un video de 480p → el `master.m3u8` no incluye `720p` ni `1080p`.
- `FAIL_TRANSCODE=1` → 3 intentos, DLQ, alerta, y reencolado con la misma clave.
