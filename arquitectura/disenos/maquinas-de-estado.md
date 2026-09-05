# Máquinas de estado

Las cinco transiciones que concentran las reglas de negocio del proyecto. Toda
transición inválida responde `409 conflict`, nunca modifica en silencio.

## Curso y versión (ADR-0007)

```mermaid
stateDiagram-v2
  direction LR
  [*] --> draft: POST /courses
  draft --> published: :publish (pasa todas las validaciones)
  published --> unpublished: :unpublish
  unpublished --> published: :publish
  unpublished --> unpublished: editar (permitido por §5.1)
  published --> archived: se publica una versión posterior
  draft --> [*]: DELETE (solo si nunca se publicó)
```

- `published` es **inmutable**: trigger en PostgreSQL sobre módulos, unidades,
  recursos, quizzes, preguntas y opciones.
- Publicar la versión N+1 archiva la N y actualiza `courses.current_version_id`.
- `unpublished` bloquea inscripciones nuevas; las existentes y su progreso se
  conservan.

## Asset de medios (ADR-0011)

```mermaid
stateDiagram-v2
  [*] --> uploading: POST /assets:init
  uploading --> uploaded: :complete
  uploading --> aborted: :abort o 24 h sin completar
  uploaded --> rejected: checksum o MIME real inválido
  uploaded --> scanning: media.probe ok
  scanning --> infected: ClamAV detecta → cuarentena + auditoría
  scanning --> clean: limpio
  clean --> ready: no requiere transcodificación (imagen, PDF, archivo)
  clean --> processing: video o audio → media.transcode_hls
  processing --> ready: HLS y póster generados
  processing --> failed: error transitorio
  failed --> processing: reintento
  failed --> dead: 3 intentos agotados → DLQ + alerta
  dead --> processing: requeue admin (misma job_key)
```

Un recurso **visible** cuyo asset no esté `ready` bloquea la publicación
(`CA-01`).

## Intento de quiz (ADR-0013)

```mermaid
stateDiagram-v2
  [*] --> in_progress: POST /attempts (congela el snapshot)
  in_progress --> in_progress: PATCH /answers (guardado parcial)
  in_progress --> submitted: :submit antes de expires_at
  in_progress --> expired: el worker cierra un intento vencido
  in_progress --> expired_submitted: :submit después de expires_at
  submitted --> [*]
  expired --> [*]
  expired_submitted --> [*]
```

- Un solo intento `in_progress` por quiz y estudiante (índice único parcial).
- `expired_submitted` **se califica** con lo guardado antes de expirar: se prefiere
  puntuar el trabajo hecho a descartarlo.
- `:submit` es idempotente por `Idempotency-Key`; reenviar devuelve la misma nota.

## Inscripción y progreso (ADR-0012)

```mermaid
stateDiagram-v2
  [*] --> active: POST /enrollments
  active --> withdrawn: :withdraw (no borra progreso)
  withdrawn --> active: reinscripción (conserva progreso, RF-10)

  state active {
    [*] --> in_progress
    in_progress --> completed: 100 % de obligatorios completed
    completed --> approved: cumple approval_criteria → encola badge.issue
    completed --> in_progress: versión nueva añade obligatorios
    approved --> approved: una versión nueva NO revoca la aprobación
  }
```

La última transición es una decisión de negocio explícita del ADR-0007: aprobar
es un hecho histórico contra la versión que se cursó, registrada en
`enrollments.approved_version_id`.

## Insignia (`CA-07`)

```mermaid
stateDiagram-v2
  [*] --> issued: badge.issue (una por enrollment_id, UNIQUE)
  issued --> revoked: POST /badges/{id}:revoke (auditado)
  revoked --> revoked: idempotente
```

- Doble protección: `job_key` determinista **y** `UNIQUE (enrollment_id)`.
- `GET /verify/{public_code}` es pública y no expone el correo del estudiante.
- Una insignia revocada sigue siendo consultable; la respuesta indica
  `"status": "revoked"` con la fecha. Se prefiere transparencia a un `404`.
