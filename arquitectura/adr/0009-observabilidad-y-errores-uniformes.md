# ADR-0009 — Observabilidad y formato uniforme de errores

- **Estado:** Aceptado
- **Fecha:** 2026-09-05
- **Requisitos:** `RT-05`, `RT-06`, `CE-07`, `CA-03`

## Contexto

El enunciado exige OpenTelemetry (§7), errores uniformes en la API (§7) y, en la
demostración, "trazas o logs correlacionados" (§10.1) más una alerta cuando un
trabajo llega a la DLQ (§6). Un comportamiento que no se pueda observar no
acredita el requisito, aunque el código exista (§10).

## Decisión

### Errores: RFC 9457 (`application/problem+json`)

Un solo formato para toda la API, incluidos los `panic` recuperados:

```json
{
  "type": "https://mooc.local/errors/validation_failed",
  "title": "La solicitud no es válida",
  "status": 422,
  "detail": "El curso no cumple los requisitos de publicación.",
  "instance": "/api/v1/courses/9f.../versions/1:publish",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "errors": [
    { "code": "structure.minimum_not_met", "field": "modules", "detail": "..." },
    { "code": "resource.not_available", "field": "modules[0].units[1].resources[0]", "detail": "..." }
  ]
}
```

El arreglo `errors` es lo que hace posible la "lista exhaustiva de errores" que
pide SEG-2. El `trace_id` en el cuerpo es lo que permite pegar una respuesta de
Postman con su traza en Jaeger durante la demo.

### Trazas — OpenTelemetry

- Exportador OTLP a Jaeger en local; a Cloud Trace en GCP (solo cambia el
  endpoint).
- Instrumentación en el middleware HTTP, en el pool de pgx, en las llamadas a
  Redis y a S3, y en cada `asynq.Handler`.
- **El contexto de traza viaja dentro del payload del job**, de modo que la
  transcodificación aparece como hijo de la petición que subió el archivo. Sin
  esto, SEG-4 no puede mostrar el recorrido completo.
- Muestreo: 100 % en local y en la demo; `parentbased_traceidratio` al 10 % bajo
  la prueba de carga, con errores siempre muestreados.

### Métricas — Prometheus

Mínimo exigible por la evaluación:

| Métrica | Tipo | Para qué |
| --- | --- | --- |
| `http_request_duration_seconds{route,method,status}` | histograma | p95 de `RNF-05` |
| `http_requests_total` | contador | tasa de error |
| `jobs_processed_total{type,result}` | contador | rendimiento de workers |
| `job_duration_seconds{type}` | histograma | costo por minuto de transcodificación (§11) |
| `jobs_retried_total{type}` | contador | backoff visible |
| `jobs_dead_letter_total{type}` | contador | **dispara la alerta de `CA-03`** |
| `jobs_duplicate_skipped_total{type}` | contador | evidencia de idempotencia |
| `queue_depth{queue}` | gauge | saturación |
| `asset_processing_seconds` | histograma | tiempo hasta video disponible (§11) |
| `db_pool_in_use` | gauge | ajuste del pool |

### Logs

JSON estructurado con `log/slog`. Campos obligatorios: `ts`, `level`, `msg`,
`trace_id`, `span_id`, `request_id`, `user_id` (si hay sesión), `module`.
Nunca se registran: contraseñas, tokens de sesión, URLs firmadas completas,
correos en logs de insignia (`CA-07`), ni claves correctas de quiz.

### Alerta

Regla de Prometheus: `increase(jobs_dead_letter_total[5m]) > 0` →
`severity: critical`, visible en Grafana y en el log de Alertmanager. Es la
evidencia literal de "tras tres reintentos fallidos, el trabajo llega a la DLQ y
emite una alerta".

### Health

`/healthz` (proceso vivo) y `/readyz` (PostgreSQL, Redis y el almacén de objetos
responden). Compose usa `readyz` para el orden de arranque.

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| Errores con formato propio | Libertad | Hay un estándar y el enunciado pide uniformidad | RFC 9457 no cuesta nada |
| Solo logs, sin trazas | Menos piezas | OTel es requisito explícito | §7 |
| Stack ELK | Búsqueda potente | Pesado para 13 semanas y para la máquina de la demo | Prometheus + Jaeger + Grafana basta |

## Consecuencias

- El `trace_id` en cada respuesta hace la sustentación mucho más convincente.
- Propagar el contexto de traza al job obliga a serializarlo en el payload; son
  dos campos.
- Cuatro servicios más en Compose. Se agrupan en el perfil `observability` para
  poder levantar el stack sin ellos durante el desarrollo.

## Cómo se verifica

- Cualquier `4xx`/`5xx` responde `application/problem+json` (prueba de contrato
  sobre todos los endpoints del OpenAPI).
- Subir un video y encontrar en Jaeger una sola traza que abarque petición,
  encolado y transcodificación.
- Forzar la DLQ y ver la alerta en Grafana.
