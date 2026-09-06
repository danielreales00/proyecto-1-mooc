# API v1 — inventario de endpoints

Prefijo `/api/v1`. Contrato formal en `backend/openapi/openapi.yaml` (OpenAPI
3.1); este documento es el mapa para repartir el trabajo entre las 4 personas del
equipo.

## Convenciones transversales

| Aspecto | Regla |
| --- | --- |
| Autenticación | `Authorization: Bearer <token opaco>` (ADR-0006) |
| Errores | `application/problem+json`, RFC 9457, con `trace_id` y arreglo `errors` (ADR-0009) |
| Paginación | Cursor: `?limit=20&cursor=<opaco>`; respuesta `{ "items": [...], "next_cursor": "..." }`. Sin `offset` |
| Concurrencia | `ETag` en `GET` de recursos editables; `If-Match` obligatorio en su `PATCH` → `412` si no coincide |
| Idempotencia | `Idempotency-Key` obligatoria en los endpoints marcados **[IK]** (ADR-0008) |
| Acciones | Verbo con dos puntos: `POST /courses/{id}/versions/{n}:publish` |
| Rate limiting | Por IP y por sesión; `429` con `Retry-After` |
| Fechas | ISO 8601 en UTC |
| Roles | `A` administrador · `P` profesor · `E` estudiante · `—` público |

## Identidad — `identity` (`RF-01`)

| Método | Ruta | Rol | Notas |
| --- | --- | --- | --- |
| POST | `/auth/register` | — | Solo estudiantes. Envía correo de verificación |
| POST | `/auth/verify-email` | — | Consume el token de un solo uso |
| POST | `/auth/resend-verification` | — | Limitado a 3/hora |
| POST | `/auth/login` | — | 5/min por IP. Devuelve token opaco |
| POST | `/auth/logout` | A P E | Revoca la sesión actual |
| POST | `/auth/password/forgot` | — | Responde `202` siempre, no revela si el correo existe |
| POST | `/auth/password/reset` | — | Revoca **todas** las sesiones del usuario |
| GET | `/me` | A P E | Perfil |
| PATCH | `/me` | A P E | Nombre; el correo no cambia en E1 |
| POST | `/me/password` | A P E | Exige la contraseña actual |
| GET | `/me/sessions` | A P E | Sesiones propias |
| DELETE | `/me/sessions/{id}` | A P E | Revocación propia |

## Administración — `admin` (`RF-02`)

| Método | Ruta | Rol | Notas |
| --- | --- | --- | --- |
| GET | `/admin/users` | A | Filtros por rol, estado, búsqueda |
| POST | `/admin/users` **[IK]** | A | Crea profesor o administrador; envía invitación |
| GET | `/admin/users/{id}` | A | |
| PATCH | `/admin/users/{id}` | A | Rol y estado. `409` si degrada al último administrador activo |
| GET | `/admin/users/{id}/sessions` | A | |
| DELETE | `/admin/users/{id}/sessions` | A | Revocación inmediata de todas (SEG-1) |
| GET | `/admin/audit` | A | Filtros por actor, acción, entidad, rango; cursor |
| GET | `/admin/jobs` | A | Cola, DLQ, reintentos |
| POST | `/admin/jobs/{id}:requeue` **[IK]** | A | Reencola con la **misma** `job_key` (SEG-4) |

## Autoría — `authoring` (`RF-03`, `RF-04`)

| Método | Ruta | Rol | Notas |
| --- | --- | --- | --- |
| GET | `/courses` | P A | Cursos propios (P) o todos (A) |
| POST | `/courses` **[IK]** | P A | Crea curso + versión 1 en `draft` |
| GET | `/courses/{id}` | P A | |
| PATCH | `/courses/{id}` | P A | Metadatos. `If-Match` |
| DELETE | `/courses/{id}` | P A | Solo si nunca se publicó |
| GET | `/courses/{id}/versions` | P A | |
| POST | `/courses/{id}/versions` **[IK]** | P A | Clona la vigente a un `draft` nuevo |
| GET | `/courses/{id}/versions/{n}` | P A | Árbol completo |
| POST | `/courses/{id}/versions/{n}:validate` | P A | Lista exhaustiva de errores sin publicar |
| POST | `/courses/{id}/versions/{n}:publish` **[IK]** | P A | `422` con **todos** los errores, o publica (`CA-01`) |
| POST | `/courses/{id}/versions/{n}:unpublish` **[IK]** | P A | Requerido para editar en E1 (§5.1) |
| GET | `/courses/{id}/versions/{n}/preview` | P A | Previsualización con `content_html` sanitizado |
| POST | `/versions/{vid}/modules` | P A | |
| PATCH/DELETE | `/modules/{id}` | P A | |
| POST | `/versions/{vid}/modules:reorder` | P A | `{"order": ["stable_id", ...]}` |
| POST | `/modules/{id}/units` | P A | |
| PATCH/DELETE | `/units/{id}` | P A | |
| POST | `/modules/{id}/units:reorder` | P A | |
| POST | `/units/{id}/resources` | P A | |
| PATCH/DELETE | `/resources/{id}` | P A | Título, visibilidad, obligatoriedad, descarga |
| POST | `/units/{id}/resources:reorder` | P A | |
| PATCH | `/resources/{id}/content` | P A | **Autosave.** `If-Match`, normaliza a Markdown canónico (ADR-0014) |
| GET | `/resources/{id}/revisions` | P A | Últimas 20 (`RF-04`) |
| POST | `/resources/{id}/revisions/{rid}:restore` | P A | Recuperación |

## Medios — `media` (`RF-05`, `RF-06`)

| Método | Ruta | Rol | Notas |
| --- | --- | --- | --- |
| POST | `/assets:init` **[IK]** | P A | Crea el asset y la multipart. Devuelve `upload_id`, `part_size` y URLs por parte (24 h) |
| GET | `/assets/{id}/upload` | P A | Partes ya recibidas: **esto es lo que permite reanudar** (SEG-3) |
| POST | `/assets/{id}/upload/parts` | P A | Renueva URLs de las partes que faltan |
| POST | `/assets/{id}:complete` **[IK]** | P A | Completa la multipart y encola `media.probe`. Responde `202` |
| POST | `/assets/{id}:abort` | P A | |
| GET | `/assets/{id}` | P A | Estado, MIME detectado, sha256, derivados |
| GET | `/assets/{id}/content` | A P E* | `302` a URL firmada (15 min) tras verificar el derecho (`CA-06`) |
| GET | `/assets/{id}/hls/master.m3u8` | A P E* | Manifiesto. La credencial la emite `media-sessions`, no este endpoint |

`E*` = estudiante **con inscripción activa** en un curso que publica ese asset.

## Catálogo e inscripción — `catalog`, `enrollment` (`RF-10`)

| Método | Ruta | Rol | Notas |
| --- | --- | --- | --- |
| GET | `/catalog/courses` | — | Solo publicados. `?q=&category=&language=&cursor=`. Full-text |
| GET | `/catalog/courses/{slug}` | — | Ficha pública: estructura sin contenido privado |
| POST | `/enrollments/{id}/media-sessions` | E | **Abre una sesión de reproducción.** Emite la credencial una sola vez por sesión, en lugar de firmar cada segmento: detrás de un CDN eso no escala (ADR-0015, D2). Ya está en el OpenAPI |
| POST | `/enrollments` **[IK]** | E | Inscribe. Reinscribir reactiva y **conserva progreso** |
| GET | `/enrollments` | E | Propias |
| GET | `/enrollments/{id}` | E | Progreso, estado, insignia |
| POST | `/enrollments/{id}:withdraw` | E | `withdrawn`, sin borrar nada |
| GET | `/enrollments/{id}/content` | E | Árbol de la versión vigente con estado de progreso por recurso |
| GET | `/enrollments/{id}/resources/{stable_id}` | E | Detalle del recurso; para quiz, **sin claves** |

## Evaluación — `assessment` (`RF-08`)

| Método | Ruta | Rol | Notas |
| --- | --- | --- | --- |
| GET | `/resources/{id}/quiz` | P A | Autoría: aquí sí viaja `is_correct` (rol profesor, dueño) |
| PUT | `/resources/{id}/quiz` | P A | Configura el quiz completo |
| POST | `/quizzes/{qid}/questions` | P A | |
| PATCH/DELETE | `/questions/{id}` | P A | |
| POST | `/enrollments/{eid}/quizzes/{stable_id}/attempts` **[IK]** | E | Crea el intento y **congela el snapshot** (ADR-0013) |
| GET | `/attempts/{id}` | E | Snapshot y respuestas guardadas. **Nunca `is_correct`** |
| PATCH | `/attempts/{id}/answers` | E | Guardado parcial. `If-Match` |
| POST | `/attempts/{id}:submit` **[IK]** | E | Califica en servidor. Idempotente (`CA-04`) |
| GET | `/attempts/{id}/result` | E | Según `feedback_policy` |
| GET | `/enrollments/{eid}/quizzes/{stable_id}/attempts` | E | Historial |

## Progreso — `progress` (`RF-09`)

| Método | Ruta | Rol | Notas |
| --- | --- | --- | --- |
| POST | `/enrollments/{id}/progress` | E | Ingesta de evidencias. **Rechaza `progress_percent` con `422` y audita** (`CA-05`, ADR-0012) |
| GET | `/enrollments/{id}/progress` | E | Porcentaje calculado por el servidor y detalle por recurso |
| GET | `/enrollments/{id}/resources/{stable_id}/position` | E | Última posición para reanudar (`RF-07`) |

## Insignias — `badges` (`RF-09`)

| Método | Ruta | Rol | Notas |
| --- | --- | --- | --- |
| GET | `/me/badges` | E | |
| GET | `/badges/{id}` | E A | |
| GET | `/verify/{public_code}` | — | **Pública.** Sin correo del estudiante (`CA-07`) |
| GET | `/verify/{public_code}/image` | — | `302` a la imagen en `mooc-badges` |
| POST | `/badges/{id}:revoke` **[IK]** | A | Auditada (SEG-8) |

## Operación

| Método | Ruta | Rol | |
| --- | --- | --- | --- |
| GET | `/healthz` | — | **Implementado** |
| GET | `/readyz` | — | **Implementado** |
| GET | `/metrics` | red interna | |
| GET | `/openapi.yaml` | — | **Implementado** |

## Catálogo de códigos de error

`type` = `https://mooc.local/errors/{código}`.

| Código | HTTP | Cuándo |
| --- | --- | --- |
| `validation_failed` | 422 | Cuerpo inválido; detalle en `errors` |
| `unauthorized` | 401 | Sin sesión o sesión revocada |
| `forbidden` | 403 | Rol, propiedad o inscripción insuficientes |
| `not_found` | 404 | |
| `conflict` | 409 | Estado incompatible (intento expirado, última cuenta de administrador) |
| `precondition_failed` | 412 | `If-Match` no coincide |
| `idempotency_key_reuse` | 422 | Misma clave, cuerpo distinto |
| `request_in_flight` | 409 | Misma clave, petición aún en curso |
| `rate_limited` | 429 | Con `Retry-After` |
| `payload_too_large` | 413 | |
| `unsupported_media_type` | 415 | MIME real fuera de la lista blanca |
| `publication_invalid` | 422 | `errors` trae **todos** los incumplimientos (`CA-01`) |
| `progress.client_computed_value` | 422 | El cliente envió un porcentaje (`CA-05`) |
| `attempt.expired` | 409 | Guardado tras la expiración |
| `internal` | 500 | Con `trace_id` para correlacionar |

## Reparto sugerido (4 personas)

| Persona | Módulos | Endpoints |
| --- | --- | --- |
| 1 | `identity`, `admin`, `audit`, plataforma (errores, idempotencia, rate limit, observabilidad) | ~22 |
| 2 | `authoring` + Markdown canónico | ~20 |
| 3 | `media` + workers de medios + `docker compose` | ~9 + 4 workers |
| 4 | `catalog`, `enrollment`, `assessment`, `progress`, `badges` | ~22 + 3 workers |

Lo primero que se acuerda entre los cuatro es el OpenAPI y las migraciones; con
el contrato fijo, los cuatro frentes avanzan en paralelo.
