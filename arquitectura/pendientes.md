# Pendientes

Notas de trabajo del equipo. **No es entregable.**

Estado a 5 de septiembre de 2026: documentación de arquitectura lista y
**rebanada vertical escrita** en `feat/esqueleto-backend` (Compose, migración
0001, plataforma, `identity` con registro/verificación/login/me y el worker
`email.send`).

**Verificado en local:** `make up` levanta los 9 servicios, `make smoke` pasa sus
12 comprobaciones, y `make scale` (3 api + 3 worker) las vuelve a pasar — la
sesión se abre en una instancia y `/me` la resuelve otra, que es la evidencia de
que la API no guarda estado (CE-01).

## Lo siguiente, en orden

Lo de arriba desbloquea lo de abajo.

| # | Tarea | Quién | Estado |
| --- | --- | --- | --- |
| 1 | Acordar entre los cuatro el OpenAPI y las migraciones iniciales. Sin esto no se puede trabajar en paralelo | Todos | Pendiente |
| 2 | Esqueleto del repo: `backend/` con `cmd/`, `internal/platform/`, `Dockerfile` multi-stage, `Makefile` | — | **Hecho** |
| 3 | `docker-compose.yml` con los servicios de `disenos/contexto-y-componentes.md` y healthchecks | — | **Hecho** salvo `clamav`, `worker-media` y observabilidad |
| 4 | Migraciones `0001`–`00NN` según `disenos/modelo-de-datos.md`, con los triggers de inmutabilidad y auditoría | — | **0001 escrita**: `identity`, `audit`, `platform` |
| 5 | Plataforma: errores RFC 9457, middleware de idempotencia, rate limiting, logs, métricas, OTel | — | **Parcial**: RFC 9457, logs y `job_runs` listos. Faltan idempotencia HTTP, rate limiting, métricas y OTel |
| 6 | `identity` + `admin` + `audit` | — | **Parcial**: registro, verificación, login, `/me`, logout y auditoría. Faltan recuperación de contraseña, sesiones propias y todo `admin` |
| 7 | Normalizador de Markdown canónico y su prueba de ida y vuelta (§11 lo recomienda hacer primero) | — | Pendiente |
| 8 | `authoring` con validación de publicación e inmutabilidad | — | Pendiente |
| 9 | `media` + workers de medios + ClamAV + FFmpeg | — | Pendiente |
| 10 | `catalog` + `enrollment` | — | Pendiente |
| 11 | `assessment` con snapshot y calificación | — | Pendiente |
| 12 | `progress` + `badges` | — | Pendiente |
| 13 | Semilla de datos sintéticos (`make seed`) | — | Pendiente |
| 14 | Colección de Postman, una carpeta por segmento SEG-1…SEG-9 | — | Pendiente |
| 15 | Pruebas de carga con k6 y medición de p95 con `api=1` frente a `api=3` | — | Pendiente |
| 16 | `make restore-test` y medición de RTO | — | Pendiente |
| 17 | CI: build, lint, `gosec`/`govulncheck`, migraciones, pruebas | — | Pendiente |
| 18 | Grabar el video de la demostración | Todos | Pendiente |

## Decisiones abiertas

Cuando se resuelvan, cada una se convierte en un ADR.

| Tema | Opciones | Quién decide | Notas |
| --- | --- | --- | --- |
| Caché del catálogo en Redis | Cachear la lista con invalidación al publicar / no cachear en E1 | Equipo | Depende de lo que muestre la prueba de carga. Medir antes de optimizar |
| Búsqueda del catálogo | `tsvector` de PostgreSQL / `pg_trgm` para tolerar erratas | Equipo | Empezar con `tsvector`; ya cubre `RF-10` |
| Generación de la imagen de insignia | Plantilla SVG → PNG en Go / composición con `ffmpeg` | Equipo | La SVG es más fácil de ajustar |
| Proxy de entrada | Caddy (TLS automático) / Nginx | Equipo | Caddy resuelve TLS sin configuración; se decidió provisionalmente |
| `sqlc` para el acceso a datos | SQL a mano / `sqlc` | Equipo | Reconsiderar si el SQL a mano se vuelve pesado (ADR-0002) |
| Tecnología del frontend (E2) | React / Angular / Svelte | Equipo | No bloquea la Entrega 1 |
| Token en claro dentro de `job_runs.payload` | Dejarlo / cifrar el payload / que el worker genere el token | Equipo | Hoy el token de verificación viaja en claro en el payload del trabajo, porque el correo debe contenerlo y `one_time_tokens` solo guarda el hash. Acotado: expira en 24 h y la fila se poda. Ver `internal/modules/identity/service.go` |
| Recorte de la escalera HLS para la demo | 360p+720p / la escalera completa | Equipo | La completa alarga mucho la grabación |

## Riesgos vigilados

| Riesgo | Señal temprana | Qué haríamos |
| --- | --- | --- |
| El alcance de E1 es amplio para 4 personas | La tarea 1 no está lista al final de la semana 1 | Congelar el contrato aunque esté imperfecto; se puede corregir con migraciones |
| FFmpeg alarga la grabación de la demo | Un video de prueba tarda más de 5 min | Usar clips de 30 s y una escalera recortada |
| ClamAV falla al arrancar en la máquina de la demo | El healthcheck no pasa en 3 min | Imagen con firmas precargadas; ensayar en la máquina donde se grabará |
| La carga multipart es incómoda desde Postman | El script pre-request no parte bien el archivo | Un `scripts/upload.sh` con `curl` como respaldo, mostrado en el video |
| Dos personas tocan las mismas migraciones | Números repetidos en un PR | Verificación en CI; renumerar al rebasar |
| Colisión de puertos con otros stacks de Docker de la máquina | `port is already allocated` al levantar | Ya resuelto: los puertos del host se parametrizan en `.env` y el proyecto usa un rango propio (8090, 8026, 9010/9011, 5433, 6380) |

## Convenciones acordadas

- Ramas: `feat/<modulo>-<descripcion>`, `fix/…`, `docs/…`.
- Commits en español, con el ID del requisito: `feat(media): carga multipart reanudable (RF-05)`.
- Un PR por módulo o por tarea de esta lista, no por sesión de trabajo.
- Nadie hace merge de un PR propio sin revisión de otro del equipo.
