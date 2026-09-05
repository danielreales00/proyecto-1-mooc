# Alcance — Entrega 1

**Objetivo:** backend y capa de workers completos y funcionales, empaquetados en
Docker, demostrables de punta a punta con Postman. Sin frontend.

Fuente: `aclaraciones-profesor.md` (aclaración de alcance) + `enunciado.md` §5.1.

## Entra

### Plataforma
- Monolito modular en Go (`cmd/api`) y workers independientes (`cmd/worker`),
  ambos sin estado y escalables con `--scale`.
- `docker compose` con: `api`, `worker`, `postgres`, `redis`, `minio`,
  `mailpit`, `clamav`, `prometheus`, `grafana`, `jaeger` y un proxy de entrada.
- Migraciones SQL versionadas, ejecutadas por un job de arranque.
- OpenAPI 3.1 generado/versionado y validado en CI.

### Funcional (RF-01 … RF-10, lado servidor)
- Identidad: registro, verificación de correo, login, sesiones revocables,
  recuperación de contraseña, rate limiting.
- Administración: alta de profesores, roles, estados, listado y revocación de
  sesiones, auditoría inmutable, protección del último administrador.
- Autoría: CRUD de curso, versiones, módulos, unidades y recursos; reordenamiento;
  autosave de bloques; validación de publicación con lista exhaustiva de errores;
  publicación inmutable.
- Multimedia: inicio de carga multipart con URLs prefirmadas, reanudación 24 h,
  verificación de checksum y MIME real, escaneo antimalware, encolamiento del
  trabajo de transcodificación.
- Workers: transcodificación a HLS con FFmpeg, conservación del original,
  generación de póster, emisión de insignias, envío de correo, barrido de cargas
  vencidas y reencolado de trabajos sin heartbeat.
- Consumo: catálogo con búsqueda, filtros y cursores; inscripción, retiro y
  reinscripción; entrega de material privado por URL firmada; reporte y
  reanudación de posición.
- Quiz: autoría, snapshot del intento, guardado parcial, expiración, envío
  idempotente, calificación en servidor y retroalimentación según política.
- Progreso e insignias: ingesta de evidencias, cálculo en servidor sobre recursos
  obligatorios, transición a `completed` y `approved`, insignia única con imagen y
  URL pública de verificación, revocación auditada.

### Calidad y evidencia
- Colección de Postman versionada, una carpeta por segmento SEG-1 … SEG-9.
- Semilla de datos sintéticos (`make seed`).
- Pruebas unitarias del dominio y de integración contra contenedores reales.
- Prueba de carga con objetivos documentados en `disenos/objetivos-de-servicio.md`.
- Logs estructurados, métricas Prometheus, trazas OpenTelemetry, alerta de DLQ.
- Backup y restauración de PostgreSQL, con medición de RPO/RTO.
- CI: build, lint, análisis de seguridad, migraciones y pruebas.

## No entra

| Fuera de alcance | Por qué | Cuándo |
| --- | --- | --- |
| Frontend web | El profesor lo relevó para esta entrega. | E2 |
| Editor visual de bloques | Es UI. La API de bloques sí queda lista. | E2 |
| Visor PDF y reproductor HLS | Son UI. La entrega firmada del contenido sí queda lista. | E2 |
| Auditoría WCAG 2.2 AA | No hay interfaz que auditar. | E2 |
| Despliegue en GCP | El enunciado pide Docker Compose ahora. | E3–E4 |
| CDN real | En local se sirve desde MinIO con cabeceras de caché. | E3 |
| Conversión PPTX/ODP → PDF (RO-01) | Opcional. | Si sobra tiempo |
| Borradores de actualización con migración de progreso (RO-04) | Opcional; en E1 se despublica para editar, como permite §5.1. | E3+ |

## Riesgos de esta entrega

| Riesgo | Mitigación |
| --- | --- |
| La transcodificación HLS con FFmpeg consume CPU y alarga la demo. | Perfiles de salida cortos para datos sintéticos; medir costo por minuto (recomendación §11 del enunciado). |
| ClamAV tarda en arrancar y descargar firmas. | Contenedor con firmas precargadas y healthcheck; el worker de escaneo espera al healthcheck. |
| La carga multipart prefirmada es incómoda desde Postman. | Script pre-request en la colección que parte el archivo y sube las partes. |
| El equipo es de 4 personas y el alcance es amplio. | Cortes por módulo con contratos de API acordados primero; ver `pendientes.md`. |

## Lista de verificación de la entrega

El profesor no califica por módulos: califica siete criterios a través de los
nueve segmentos de la demostración (§10.2). Esta es la vista que importa el día
de la entrega.

Estado a 5 de septiembre de 2026.

| Segmento | Qué debe demostrar | Necesita | Estado |
| --- | --- | --- | --- |
| **1.** Identidad y administración | Registro, invitación de profesor, revocación inmediata de sesiones, suspensión auditada, rechazo de operaciones no autorizadas | `identity` completo + `admin` | Parcial |
| **2.** Autoría y publicación | Borrador, módulos, unidades y recursos; previsualización; lista exhaustiva de errores; versión inmutable | `authoring` + Markdown canónico | Nada |
| **3.** Carga multimedia | URLs prefirmadas, reanudación, checksum, MIME real, antimalware, encolamiento, original conservado | `media` (carga) + ClamAV | Nada |
| **4.** Procesamiento y fallos | Estados del recurso, HLS sin *upscaling*, doble entrega idempotente, backoff, DLQ, alerta, reencolado con la misma clave | Workers de medios + observabilidad + fallos inyectables | El mecanismo existe, sin nada que procesar ni alerta |
| **5.** Consumo de contenido | Inscripción, streaming adaptativo, reanudación, entrega autorizada | `enrollment` + entrega firmada | Nada |
| **6.** Quiz | Snapshot, guardados parciales, sin claves en el cliente, envío idempotente, expiración, cálculo de la nota | `assessment` | Nada |
| **7.** Progreso y aprobación | Heartbeats, permanencia, rechazo de manipulación, porcentaje de obligatorios, `completed` y `approved` | `progress` | Nada |
| **8.** Insignia y actualización | Emisión única, URL sin correo, revocación auditada, `stable_id` y progreso conservado | `badges` + versión nueva | Nada |
| **9.** Operación | p95, escalamiento, observabilidad, fallos, backup, restauración, RTO/RPO | Carga con k6 + backup/restauración + observabilidad | Solo el escalamiento, verificado |

### Requisitos formales de la demostración (§10.1)

Son condiciones de aceptación, no buenas prácticas opcionales. Ninguna existe
todavía.

| Requisito | Cómo lo cumplimos | Estado |
| --- | --- | --- |
| Datos sintéticos sobre el sistema desplegado | `make seed` | Pendiente |
| Evidencia con respuesta de la API, estado persistido y logs o trazas correlacionados | `trace_id` en cada respuesta y en cada log; colección de Postman | Parcial: el `trace_id` ya viaja |
| Pruebas que cubran roles, propiedad, idempotencia, fallos inyectados y límites de rendimiento | Pruebas unitarias, de integración y la colección | Pendiente |
| CI que complete build, lint, análisis de seguridad, migraciones y pruebas **antes** de la demostración | Flujo de trabajo en el repositorio | Pendiente |

### Andamiaje de evidencia

Cuatro piezas producen toda la evidencia anterior. Se construyen **junto al
primer módulo grande, no después**: una vez que un módulo se da por hecho sin
pruebas, nadie vuelve a escribirlas.

1. **Pruebas automáticas** — unitarias del dominio, de la capa HTTP con dobles,
   y de integración contra los contenedores reales.
2. **Semilla de datos sintéticos** — administrador, profesor y estudiantes, con
   correos ya verificados, para que la demo no empiece en cero.
3. **Colección de Postman** — una carpeta por segmento, con aserciones. Es la
   interfaz de la demostración: sin ella no hay demo.
4. **CI** — build, `gofmt`, `go vet`, `govulncheck`, migraciones y pruebas.

## Definición de terminado

Un requisito está terminado cuando: hay código, hay migración si toca esquema,
hay prueba automática, aparece en la colección de Postman, está reflejado en el
OpenAPI y su ID (`RF-xx` / `CA-xx`) figura en el commit.
