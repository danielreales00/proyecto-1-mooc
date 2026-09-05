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

## Definición de terminado

Un requisito está terminado cuando: hay código, hay migración si toca esquema,
hay prueba automática, aparece en la colección de Postman, está reflejado en el
OpenAPI y su ID (`RF-xx` / `CA-xx`) figura en el commit.
