# Estado por módulo

Qué hace la aplicación hoy, módulo a módulo. Medido contra el contrato
(`backend/openapi/openapi.yaml`), no de memoria.

**61 de 88 operaciones** · 8 módulos · 4 trabajos asíncronos · 37 pruebas

Actualizado el 7 de septiembre de 2026.

| Módulo | Ops | Qué hay implementado | Qué falta |
| --- | --- | --- | --- |
| **`identity`** | 5 / 12 | Registro público de estudiante con verificación por correo (el token lo envía un worker). Login con sesión opaca en Redis y `/me`. Logout con **revocación inmediata**, sin ventana de gracia. Contraseñas con Argon2id. Límite de tasa en dos niveles: 5/min por cuenta y 60/min por IP. | Recuperación y restablecimiento de contraseña. Reenvío de verificación. `PATCH /me`. Cambio de contraseña propia. Listado y revocación de sesiones propias. |
| **`admin`** | 9 / 9 | Invitación de profesores y administradores por correo. Listado con filtros por rol, estado y búsqueda. Cambio de rol y estado con **protección del último administrador activo**, comprobada dentro de la transacción. Sesiones ajenas con revocación inmediata. Consulta del rastro de auditoría. Listado de la cola y reencolado desde la DLQ conservando la misma `job_key`. | Nada del alcance mínimo. |
| **`authoring`** | 16 / 27 | Curso con su versión 1. Jerarquía módulo → unidad → recurso. Autosave con **normalización a Markdown canónico**. Previsualización con HTML saneado. Validación que devuelve **todos** los incumplimientos, no el primero. Publicar, despublicar y clonar versión conservando los `stable_id`. La versión publicada es **inmutable por trigger de PostgreSQL**. | Reordenamiento de módulos, unidades y recursos. Borrado de nodos y de curso. Historial de revisiones y restauración. |
| **`learning`** | 9 / 11 | Catálogo público con búsqueda de texto completo. Inscripción, retiro y reinscripción **conservando progreso**. Árbol del curso con avance por recurso. **Ingesta de evidencias**: enviar `progress_percent` da `422` y queda auditado. El servidor descarta heartbeats por cadencia, salto de posición y rango. Cálculo del avance sobre los obligatorios y transición a `approved`, que encola la insignia. | Última posición de reproducción para reanudar. Sesión de reproducción (`media-sessions`). |
| **`assessment`** | 8 / 11 | Configuración del quiz por el profesor: **única ruta por la que sale `is_correct`**. Intento con **snapshot congelado sin claves**. Guardado parcial. Expiración por reloj del servidor. Calificación en servidor y envío idempotente. La política `full` degrada a `correctness` mientras queden intentos. | CRUD suelto de preguntas (`POST /quizzes/{id}/questions`, `PATCH` y `DELETE` de pregunta). |
| **`badges`** | 3 / 5 | Un worker emite la insignia al aprobar, **única por inscripción** con dos barreras: clave determinista del trabajo y restricción `UNIQUE`. Verificación **pública sin el correo** del estudiante. Revocación auditada. | `GET /badges/{id}` y la redirección a la imagen. La generación del PNG. |
| **`media`** | 7 / 9 | Carga multipart con URLs prefirmadas: **los bytes no pasan por la API**. **Reanudación real**: consulta qué partes faltan y sube solo esas, con 24 h de ventana. Verificación en servidor: recalcula el SHA-256 leyendo del bucket y detecta el MIME por *magic bytes*. Lo rechazado va a cuarentena con el original intacto. | **Escaneo antimalware con ClamAV.** **Transcodificación a HLS con FFmpeg.** Manifiesto y entrega firmada. Ver [`media-plan.md`](media-plan.md). |
| **`audit`** | — | Rastro **inmutable por trigger**: PostgreSQL rechaza cualquier `UPDATE` o `DELETE`. Registra registro, login, publicación, cambios administrativos, revocaciones, intentos de manipulación del progreso y rechazos de archivos. | Exportación y retención automática. |
| **Operación** | 4 / 4 | `/healthz`, `/readyz` con las tres dependencias, `/metrics` y `/openapi.yaml` servido desde el binario. | — |

## Trabajos asíncronos

| Trabajo | Estado | Qué hace |
| --- | --- | --- |
| `email.send` | ✅ | Verificación de correo e invitaciones |
| `badge.issue` | ✅ | Emisión de la insignia al aprobar |
| `media.probe` | ✅ | Recalcula checksum y detecta el MIME real |
| `jobs.reaper` | ✅ | Republica los trabajos huérfanos cada 30 s |
| `media.scan` | ❌ | ClamAV |
| `media.transcode_hls` | ❌ | FFmpeg → HLS |
| `media.poster` | ❌ | Fotograma de portada |
| `progress.recompute` | ❌ | Recálculo tras publicar una versión |
| `assessment.expire_attempts` | ❌ | Cierre de intentos vencidos |
| `media.sweep_uploads` | ❌ | Barrido de cargas de más de 24 h |

## Plataforma

| Pieza | Estado |
| --- | --- |
| Errores uniformes RFC 9457 con `trace_id` | ✅ |
| Outbox, idempotencia y DLQ en `job_runs` | ✅ |
| Reclamo atómico antes de ejecutar | ✅ |
| Reintentos con backoff y alerta de DLQ | ✅ verificada disparando |
| Límites de tasa en Redis | ✅ |
| Métricas Prometheus y panel de Grafana | ✅ |
| Escalamiento horizontal | ✅ verificado con `api=3` |
| Migraciones SQL versionadas | ✅ |
| Middleware de `Idempotency-Key` | ✅ en 11 operaciones no repetibles |
| `ETag` / `If-Match` | ✅ en el autosave |
| Paginación por cursor | ⚠️ hecha en el catálogo; los demás listados aún devuelven `null` |
| Trazas OpenTelemetry | ❌ solo hay logs y métricas |

## Evidencia

| Pieza | Estado |
| --- | --- |
| `make demo` — flujo completo, 33 aserciones | ✅ |
| `make smoke` — identidad de punta a punta | ✅ |
| Colección de Postman, 39 peticiones | ✅ SEG-1, 2, 5, 6, 7 y 8 |
| `make ci` — todo lo que corre el CI, en local | ✅ |
| CI con 4 trabajos | ✅ |
| Semilla de datos sintéticos | ✅ 8 cuentas |
| Pruebas automáticas | ✅ 37, en 5 paquetes |
| Prueba de carga con k6 | ❌ |
| Backup y restauración con RTO medido | ❌ |

## Cobertura de la demostración

| Segmento | Estado |
| --- | --- |
| SEG-1 · Identidad | ✅ completo |
| SEG-2 · Autoría y publicación | ✅ completo |
| SEG-3 · Carga multimedia | ⚠️ carga y verificación sí; antimalware no |
| SEG-4 · Procesamiento y fallos | ⚠️ con `badge.issue`; falta con transcodificación |
| SEG-5 · Consumo de contenido | ✅ por API |
| SEG-6 · Quiz | ✅ completo |
| SEG-7 · Progreso y aprobación | ✅ completo, con señal fraudulenta |
| SEG-8 · Insignia | ✅ completo |
| SEG-9 · Operación | ⚠️ escalado, auditoría y alerta sí; carga y restauración no |
