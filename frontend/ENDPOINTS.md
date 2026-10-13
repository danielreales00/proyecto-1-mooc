# Inventario de endpoints

Generado con `docker compose run --rm herramientas python3 scripts/frontend-endpoints.py`.

El router registra **65 operaciones disponibles**. El contrato declara **90 operaciones**; las que no están registradas no se presentan como funciones disponibles en la interfaz.

| Módulo | Método | Ruta real | Interfaz |
| --- | --- | --- | --- |
| admin | DELETE | `/api/v1/admin/users/{id}/sessions` | Administración |
| admin | GET | `/api/v1/admin/audit` | Administración |
| admin | GET | `/api/v1/admin/jobs` | Administración |
| admin | GET | `/api/v1/admin/users` | Administración |
| admin | GET | `/api/v1/admin/users/{id}` | Administración |
| admin | GET | `/api/v1/admin/users/{id}/sessions` | Administración |
| admin | PATCH | `/api/v1/admin/users/{id}` | Administración |
| admin | POST | `/api/v1/admin/jobs/{id}/requeue` | Administración |
| admin | POST | `/api/v1/admin/users` | Administración |
| assessment | GET | `/api/v1/attempts/{id}` | Evaluación y editor de quiz |
| assessment | GET | `/api/v1/attempts/{id}/result` | Evaluación y editor de quiz |
| assessment | GET | `/api/v1/enrollments/{eid}/quizzes/{stableId}/attempts` | Evaluación y editor de quiz |
| assessment | GET | `/api/v1/resources/{resourceId}/quiz` | Evaluación y editor de quiz |
| assessment | PATCH | `/api/v1/attempts/{id}/answers` | Evaluación y editor de quiz |
| assessment | POST | `/api/v1/attempts/{id}/submit` | Evaluación y editor de quiz |
| assessment | POST | `/api/v1/enrollments/{eid}/quizzes/{stableId}/attempts` | Evaluación y editor de quiz |
| assessment | PUT | `/api/v1/resources/{resourceId}/quiz` | Evaluación y editor de quiz |
| authoring | GET | `/api/v1/courses` | Estudio docente |
| authoring | GET | `/api/v1/courses/{courseId}` | Estudio docente |
| authoring | GET | `/api/v1/courses/{courseId}/versions` | Estudio docente |
| authoring | GET | `/api/v1/courses/{courseId}/versions/{n}` | Estudio docente |
| authoring | GET | `/api/v1/courses/{courseId}/versions/{n}/preview` | Estudio docente |
| authoring | PATCH | `/api/v1/courses/{courseId}` | Estudio docente |
| authoring | PATCH | `/api/v1/resources/{resourceId}` | Estudio docente |
| authoring | PATCH | `/api/v1/resources/{resourceId}/content` | Estudio docente |
| authoring | POST | `/api/v1/courses` | Estudio docente |
| authoring | POST | `/api/v1/courses/{courseId}/versions` | Estudio docente |
| authoring | POST | `/api/v1/courses/{courseId}/versions/{n}/publish` | Estudio docente |
| authoring | POST | `/api/v1/courses/{courseId}/versions/{n}/unpublish` | Estudio docente |
| authoring | POST | `/api/v1/courses/{courseId}/versions/{n}/validate` | Estudio docente |
| authoring | POST | `/api/v1/modules/{moduleId}/units` | Estudio docente |
| authoring | POST | `/api/v1/units/{unitId}/resources` | Estudio docente |
| authoring | POST | `/api/v1/versions/{versionId}/modules` | Estudio docente |
| badges | GET | `/api/v1/me/badges` | Insignias y verificación |
| badges | GET | `/api/v1/verify/{code}` | Insignias y verificación |
| badges | POST | `/api/v1/badges/{id}/revoke` | Insignias y verificación |
| identity | GET | `/api/v1/me` | Cuenta y sesión |
| identity | POST | `/api/v1/auth/login` | Cuenta y sesión |
| identity | POST | `/api/v1/auth/logout` | Cuenta y sesión |
| identity | POST | `/api/v1/auth/register` | Cuenta y sesión |
| identity | POST | `/api/v1/auth/verify-email` | Cuenta y sesión |
| learning | GET | `/api/v1/catalog/courses` | Catálogo y aula |
| learning | GET | `/api/v1/catalog/courses/{slug}` | Catálogo y aula |
| learning | GET | `/api/v1/enrollments` | Catálogo y aula |
| learning | GET | `/api/v1/enrollments/{id}` | Catálogo y aula |
| learning | GET | `/api/v1/enrollments/{id}/content` | Catálogo y aula |
| learning | GET | `/api/v1/enrollments/{id}/progress` | Catálogo y aula |
| learning | POST | `/api/v1/enrollments` | Catálogo y aula |
| learning | POST | `/api/v1/enrollments/{id}/progress` | Catálogo y aula |
| learning | POST | `/api/v1/enrollments/{id}/withdraw` | Catálogo y aula |
| media | GET | `/api/v1/assets/{id}` | Biblioteca y reproductor |
| media | GET | `/api/v1/assets/{id}/content` | Biblioteca y reproductor |
| media | GET | `/api/v1/assets/{id}/hls/master.m3u8` | Biblioteca y reproductor |
| media | GET | `/api/v1/assets/{id}/hls/{playlist}` | Biblioteca y reproductor |
| media | GET | `/api/v1/assets/{id}/upload` | Biblioteca y reproductor |
| media | GET | `/api/v1/media-sessions/{token}/master.m3u8` | Biblioteca y reproductor |
| media | POST | `/api/v1/assets/init` | Biblioteca y reproductor |
| media | POST | `/api/v1/assets/{id}/abort` | Biblioteca y reproductor |
| media | POST | `/api/v1/assets/{id}/complete` | Biblioteca y reproductor |
| media | POST | `/api/v1/assets/{id}/upload/parts` | Biblioteca y reproductor |
| media | POST | `/api/v1/enrollments/{id}/media-sessions` | Biblioteca y reproductor |
| platform | GET | `/healthz` | Operación y documentación |
| platform | GET | `/metrics` | Operación y documentación |
| platform | GET | `/openapi.yaml` | Operación y documentación |
| platform | GET | `/readyz` | Operación y documentación |

## Operaciones del contrato sin ruta registrada

| Método | Ruta | Estado declarado |
| --- | --- | --- |
| POST | `/api/v1/auth/resend-verification` | planificado |
| POST | `/api/v1/auth/password/forgot` | planificado |
| POST | `/api/v1/auth/password/reset` | planificado |
| PATCH | `/api/v1/me` | planificado |
| POST | `/api/v1/me/password` | planificado |
| GET | `/api/v1/me/sessions` | planificado |
| DELETE | `/api/v1/me/sessions/{sessionId}` | planificado |
| DELETE | `/api/v1/courses/{courseId}` | planificado |
| POST | `/api/v1/versions/{versionId}/modules/reorder` | planificado |
| PATCH | `/api/v1/modules/{moduleId}` | planificado |
| DELETE | `/api/v1/modules/{moduleId}` | planificado |
| POST | `/api/v1/modules/{moduleId}/units/reorder` | planificado |
| PATCH | `/api/v1/units/{unitId}` | planificado |
| DELETE | `/api/v1/units/{unitId}` | planificado |
| POST | `/api/v1/units/{unitId}/resources/reorder` | planificado |
| DELETE | `/api/v1/resources/{resourceId}` | planificado |
| GET | `/api/v1/resources/{resourceId}/revisions` | planificado |
| POST | `/api/v1/resources/{resourceId}/revisions/{revisionId}/restore` | planificado |
| GET | `/api/v1/enrollments/{enrollmentId}/resources/{resourceStableId}` | planificado |
| POST | `/api/v1/quizzes/{quizId}/questions` | planificado |
| PATCH | `/api/v1/questions/{questionId}` | planificado |
| DELETE | `/api/v1/questions/{questionId}` | planificado |
| GET | `/api/v1/enrollments/{enrollmentId}/resources/{resourceStableId}/position` | planificado |
| GET | `/api/v1/badges/{badgeId}` | planificado |
| GET | `/api/v1/verify/{publicCode}/image` | planificado |

Los manifiestos y playlists HLS los consume el reproductor. Las partes de carga se envían por PUT a URLs firmadas del almacén, sin token de usuario. `/metrics` solo se expone dentro de la máquina en producción.
