# ADR-0012 — Progreso verificado en el servidor

- **Estado:** Aceptado
- **Fecha:** 2026-09-05
- **Requisitos:** `RF-09`, `CA-05`, `CE-06`

## Contexto

`CA-05` es inusualmente específica: "Heartbeats, permanencia y eventos de apertura
determinan el avance; porcentajes enviados por el cliente se rechazan y auditan".
SEG-7 exige demostrar el rechazo con **señales fraudulentas**. Es decir, no basta
con calcular el progreso en el servidor: hay que demostrar que el intento de
manipulación se detecta, se rechaza y queda registrado.

## Decisión

### El cliente reporta evidencias, nunca conclusiones

Un único endpoint de ingesta:

```
POST /api/v1/enrollments/{id}/progress
{
  "resource_stable_id": "…",
  "kind": "open" | "heartbeat" | "close",
  "position_seconds": 128.4,     // solo audio/video
  "client_ts": "2026-09-05T…Z"
}
```

El cuerpo **no tiene** campo de porcentaje ni de "completado". Si llega
`progress_percent`, `completed` o `state`, la respuesta es
`422 progress.client_computed_value` y se escribe un evento de auditoría
(`progress.tampering_attempt`) con usuario, recurso y cuerpo recibido. Esa es la
demostración literal de `CA-05`.

### Reglas de validación de evidencias

Se aplican en el servidor y descartan la evidencia sospechosa:

| Regla | Umbral | Motivo |
| --- | --- | --- |
| Cadencia mínima entre heartbeats | ≥ 10 s de reloj del servidor | Impide inundar para simular permanencia |
| Avance de posición por heartbeat | ≤ 2× el tiempo real transcurrido | Impide saltar al final del video |
| `position_seconds` dentro de la duración | `0 ≤ pos ≤ duration + 2 s` | Coherencia |
| Se exige `open` antes del primer `heartbeat` | — | Ordena la secuencia |
| El reloj lo pone el servidor | `client_ts` se registra, no se usa | El cliente no controla el tiempo |
| Solo con inscripción activa | — | `CA-06` |

Toda evidencia descartada incrementa `progress_rejected_total{reason}` y, si es
manipulación evidente, se audita.

### Criterio de completado por tipo de recurso

| Tipo | Se marca `completed` cuando |
| --- | --- |
| Video / audio | Permanencia acumulada ≥ 90 % de la duración **y** posición máxima ≥ 90 % |
| PDF / presentación | `open` más permanencia ≥ 30 s |
| Texto enriquecido | `open` más permanencia ≥ 15 s |
| Imagen, enlace, descargable, iframe | `open` |
| Quiz | Intento enviado con nota ≥ la mínima del curso |

La permanencia se **acumula** entre sesiones, así que un video visto en tres
tandas cuenta. Se guarda en `progress.resource_progress` por
`(enrollment_id, resource_stable_id)` — `stable_id`, para sobrevivir a versiones
nuevas (ADR-0007).

### Estados de la inscripción

```
active ──todos los obligatorios completed──> completed ──criterios de aprobación──> approved
   │                                                                                    │
   └── withdrawn ──reinscripción (conserva progreso, RF-10)──> active                   └──> badge.issue
```

- `completed`: 100 % de los recursos **obligatorios y visibles** de la versión
  vigente están `completed`. Los opcionales no cuentan para el porcentaje.
- `approved`: `completed` **y** cumple los criterios definidos por el profesor en
  la versión (porcentaje mínimo de obligatorios y nota mínima de quizzes).
- La transición a `approved` encola `badge.issue` con `job_key`
  `badge.issue:{enrollment_id}` (ADR-0008): la insignia es única aunque la
  transición se dispare dos veces.
- El recálculo ocurre de forma síncrona al ingerir una evidencia que cierra un
  recurso, y de forma asíncrona (`progress.recompute`) cuando se publica una
  versión nueva del curso.

### Retiro y reinscripción

`withdrawn` conserva `resource_progress` y los intentos de quiz. Reinscribirse
reactiva la fila; nada se borra (`RF-10`).

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| Confiar en el porcentaje del cliente | Trivial | Lo prohíbe `CA-05` explícitamente | Requisito |
| Solo evento de "completado" por recurso | Simple | No hay permanencia ni heartbeats; no distingue abrir de estudiar | `CA-05` los nombra |
| Recalcular todo en cada lectura | Siempre consistente | Costoso con 2.000 concurrentes | Se materializa y se recalcula por evento |
| Detección estadística de fraude (ML) | Más fina | Desproporcionada | Reglas explícitas y auditables bastan y se explican mejor |

## Consecuencias

- Los umbrales son configurables por variable de entorno, porque van a ajustarse
  al grabar la demo (un video de 40 min no se ve entero en cámara).
- La ingesta de evidencias es el endpoint de mayor tasa de escritura: se agrupa
  por `enrollment_id` y se aplica rate limiting por sesión.
- Las reglas explícitas hacen la demo de SEG-7 fácil de guionizar: hay un caso
  legítimo y varios fraudulentos, cada uno con su motivo de rechazo.

## Cómo se verifica

- Enviar `{"progress_percent": 100}` → `422` + evento en `audit.events`.
- 20 heartbeats en 2 s → todos menos el primero descartados, métrica
  `progress_rejected_total{reason="cadence"}`.
- Heartbeat con `position_seconds` saltando de 10 a 3.000 → descartado.
- Recorrido legítimo → `active → completed → approved` y una sola insignia.
