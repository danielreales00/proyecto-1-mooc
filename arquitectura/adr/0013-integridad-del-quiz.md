# ADR-0013 — Integridad del quiz: snapshot y clave exclusiva del servidor

- **Estado:** Aceptado
- **Fecha:** 2026-09-05
- **Requisitos:** `RF-08`, `CA-04`, `CE-05`

## Contexto

`CA-04` exige tres cosas a la vez: la clave correcta nunca llega al cliente, el
envío definitivo es idempotente y la calificación se calcula en el servidor.
SEG-6 añade snapshot, guardados parciales y expiración. El riesgo evidente es
filtrar `is_correct` en alguna respuesta; el riesgo menos evidente es que el
profesor edite el quiz mientras un estudiante lo está presentando.

## Decisión

### La clave no sale, por construcción

`quiz_options.is_correct` **no aparece en ninguna estructura de salida**. No se
resuelve confiando en el `omitempty` de un DTO compartido: hay dos tipos
distintos en Go.

```go
// dominio, solo servidor
type Option struct { StableID uuid.UUID; TextMD string; IsCorrect bool }

// salida al cliente: no existe el campo
type OptionView struct { StableID uuid.UUID `json:"stable_id"`; TextMD string `json:"text_md"` }
```

Una prueba de contrato recorre todas las respuestas del flujo de quiz y falla si
aparece la subcadena `is_correct` o `correct`. Es fea y es la que da la garantía.

### Snapshot al iniciar el intento

`POST /quizzes/{stable_id}/attempts` crea el intento y **congela** en
`quiz_attempts.snapshot` (JSONB) las preguntas y opciones tal como estaban, con
sus `stable_id`, ya barajadas si la política del quiz lo pide.

Consecuencias buscadas:

- El estudiante ve siempre el mismo examen aunque el curso se despublique y se
  edite (ADR-0007).
- La calificación es **reproducible**: se recalcula contra el snapshot, no contra
  el estado actual del quiz.
- El snapshot **no incluye** `is_correct`. La clave se lee de la base al
  calificar, por `stable_id`.

### Guardado parcial

`PATCH /attempts/{id}/answers` guarda selecciones sin calificar ni cerrar. Es
repetible, no exige `Idempotency-Key` (es naturalmente idempotente: sustituye la
respuesta de esa pregunta) y responde con `ETag` para detectar escrituras
concurrentes desde dos pestañas.

### Expiración

`quiz_attempts.expires_at = started_at + time_limit` cuando el quiz tiene límite.
Lo evalúa el **servidor** con su reloj:

- Un `PATCH` posterior a `expires_at` → `409 attempt.expired`.
- Un `:submit` posterior → se acepta pero **se califica solo lo guardado antes de
  expirar**, y el intento queda `expired_submitted`. Se prefiere calificar a
  descartar el trabajo del estudiante.
- Un worker programado (`assessment.expire_attempts`, cada minuto) cierra los
  intentos vencidos que nadie envió.

### Envío definitivo

`POST /attempts/{id}:submit` con `Idempotency-Key` obligatoria (ADR-0008).
Califica en el servidor:

- Pregunta de respuesta única: acierto exacto.
- Pregunta de respuesta múltiple: **todo o nada** por defecto; el quiz puede
  configurar puntuación parcial (`partial_credit`), y entonces
  `max(0, aciertos − errores) / correctas`.
- Nota final en escala 0–100, redondeada a dos decimales. Se guardan la nota, el
  puntaje por pregunta y la versión del algoritmo (`grading_version`), para que
  un recálculo futuro sea auditable.

### Retroalimentación configurable

`quizzes.feedback_policy ∈ {none, score_only, correctness, full}`:

| Política | Tras enviar, el estudiante ve |
| --- | --- |
| `none` | Nada hasta que el profesor libere |
| `score_only` | Solo la nota |
| `correctness` | Nota y qué preguntas acertó, **sin** cuáles eran las correctas |
| `full` | Nota, aciertos y explicación por pregunta |

Solo `full` revela la clave, y solo después de que el intento está cerrado y
solo si ya no quedan intentos disponibles. Con intentos restantes, `full`
degrada a `correctness`.

### Intentos

`max_attempts` por quiz (`0` = ilimitados). Un índice único parcial impide dos
intentos `in_progress` simultáneos del mismo estudiante en el mismo quiz
(ADR-0008).

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| Enviar el quiz completo con `is_correct` y calificar en el cliente | Instantáneo | Prohibido explícitamente | `CA-04` |
| Sin snapshot, leer el quiz vigente al calificar | Menos almacenamiento | Editar el quiz altera intentos en curso; la calificación deja de ser reproducible | SEG-6 pide snapshot |
| Snapshot con la clave incluida, filtrada al serializar | Un solo objeto | Un `json.Marshal` descuidado la filtra | El campo no debe existir en la ruta de salida |
| Expiración validada en el cliente | Simple | El cliente controla su reloj | El servidor decide |

## Consecuencias

- El snapshot duplica el enunciado del quiz por intento. Con 50.000 usuarios es
  aceptable en JSONB comprimido; se poda junto con los intentos antiguos.
- Barajar dentro del snapshot obliga a calificar por `stable_id` de opción, nunca
  por posición.
- `grading_version` permite recalificar tras corregir un error sin perder la
  historia.

## Cómo se verifica

- Prueba de contrato: ninguna respuesta del flujo contiene `is_correct`.
- Iniciar intento, despublicar el curso y editar el quiz → el intento en curso
  conserva sus preguntas.
- Reenviar `:submit` con la misma clave → misma nota, `Idempotency-Replayed: true`.
- Agotar el tiempo y enviar → `expired_submitted` con lo guardado a tiempo.
