# Alcance de la Entrega 4 — Cloud Run y Cloud Pub/Sub

Fuente: `2026-20 Entrega 4 - Cloud Run y Pub-Sub.pdf`, transcrito en
[`enunciado-entrega-4.md`](enunciado-entrega-4.md). Este documento es nuestra
lectura: qué cambia, qué decidimos y en qué orden se hace. Ante discrepancia,
manda el PDF.

## Lo que ordena todo

> Sustituir el mecanismo de mensajería previo por un topic de Pub/Sub y una
> suscripción push autenticada dirigida al servicio de workers.

Las máquinas virtuales de la Entrega 3 dejan paso a tres servicios de Cloud
Run, y la cola asynq sobre Redis deja paso a Pub/Sub con entrega push. El
dominio no cambia. Cambian dos adaptadores (publicar y consumir trabajos), el
arranque de los procesos y la infraestructura.

## Rúbrica y estado

| Criterio | Puntos | Qué lo demuestra | Estado al 7 de noviembre |
| --- | --- | --- | --- |
| Despliegue e integración | 30 | Frontend, backend y workers en Cloud Run, integrados con Cloud SQL, Cloud Storage y Pub/Sub | No empezado |
| Pub/Sub y workers | 25 | Flujo completo autenticado, confirmación tras persistir, reentregas sin efectos duplicados | La idempotencia ya existe (ADR-0008). Falta el adaptador |
| Escalamiento | 20 | Máximo de 3 por servicio y métricas de instancias y mensajes pendientes con carga controlada | No empezado |
| Configuración y acceso | 15 | Cuentas mínimas, workers solo invocables por la suscripción, secretos en Secret Manager | Secret Manager y cuentas por papel ya existen |
| Documentación y video | 10 | Guía reproducible, diagrama, evidencias y video de hasta 20 minutos | No empezado |

## Decisiones

| # | Decisión | Por qué |
| --- | --- | --- |
| D1 | Adaptador `pubsub` junto al de asynq, elegido con `QUEUE_BACKEND`. Local y CI siguen con asynq, la nube usa Pub/Sub | Los módulos publican por una interfaz y no cambian. El stack local sigue probando el dominio entero en segundos |
| D2 | El worker atiende la suscripción push en `POST /pubsub/push`. Responde 2xx solo cuando el trabajo terminó y quedó persistido | Es la confirmación que pide el enunciado. Un error o un corte provocan la reentrega |
| D3 | Reentregas con el reclamo atómico de `job_runs`. Si otro worker tiene el trabajo en curso, se responde 409 y Pub/Sub reintenta. Si ya terminó, se responde 2xx sin repetirlo | Ningún efecto se duplica y ningún mensaje se pierde porque otro worker se cayera a medias |
| D4 | Política de reintentos de 10 a 600 s, plazo de confirmación de 600 s, cinco entregas y un topic de mensajes muertos | El plazo cubre la transcodificación más larga medida (196 s). Al quinto intento el trabajo queda `dead` en la base y el mensaje en el topic de muertos |
| D5 | Un solo servicio de workers con FFmpeg y ClamAV en la misma imagen. `clamd` arranca antes que el worker | Un contenedor de aplicación por instancia. Las firmas van en la imagen (ADR-0011) |
| D6 | Workers con ingreso interno y `roles/run.invoker` solo para la cuenta de la suscripción y la de Cloud Scheduler | Nadie más puede invocarlos, ni desde Internet ni desde el proyecto |
| D7 | Redis sigue en `mooc-cola` para sesiones, caché y límites de tasa, alcanzado con salida directa a la VPC. Mailpit sigue ahí | Pub/Sub sustituye la mensajería, no el resto de usos de Redis (ADR-0006) |
| D8 | El frontend llama al backend en otro origen. El backend responde CORS solo al origen del frontend, y el frontend recibe la URL del backend por variable de entorno | Son dos servicios con dos URL. La sesión viaja en `Authorization`, sin cookies |
| D9 | Migraciones y semilla como trabajos de Cloud Run | Corren una vez por despliegue, fuera de las réplicas |
| D10 | El reaper y la limpieza los dispara Cloud Scheduler contra el worker | Cloud Run solo tiene CPU durante una petición. Un bucle en segundo plano no es fiable |
| D11 | Entorno de Terraform `entrega4`, que hereda el estado de `entrega3`. Borra grupos, balanceador y autoscalers, y conserva red, base, buckets, secretos, registro, cola y presupuesto | El mismo patrón de la Entrega 3 (ADR-0019, D1). La línea base queda en el tag `entrega-3` |
| D12 | Nueva dependencia `cloud.google.com/go/pubsub/v2`, registrada en un ADR | Cliente oficial con lotes, reintentos y credenciales del entorno |

## Configuración de los servicios

| Servicio | CPU | Memoria | Concurrencia | Tiempo máximo | Instancias |
| --- | --- | --- | --- | --- | --- |
| `mooc-frontend` | 1 | 512 MiB | 80 | 30 s | 0 a 3 |
| `mooc-backend` | 1 | 2 GiB | 20 | 60 s | 0 a 3 |
| `mooc-workers` | 2 | 4 GiB | 1 | 600 s | 0 a 3 |

El backend tiene 2 GiB y concurrencia 20 porque cada inicio de sesión reserva
64 MiB para argon2id, y en la Entrega 2 una ráfaga de inicios agotó la memoria.
Los workers procesan un trabajo a la vez porque FFmpeg ocupa los dos núcleos.
Con tres instancias hay tres trabajos en curso y el resto espera en Pub/Sub,
que es la métrica de mensajes pendientes.

## Fases

| Fase | Contenido | Se verifica con | Costo |
| --- | --- | --- | --- |
| F1 | Código: adaptador de Pub/Sub, manejador push, CORS, URL del backend en el frontend, imagen de workers con ClamAV, ADR | `make ci` y una prueba local con el emulador de Pub/Sub | Ninguno |
| F2 | Terraform del entorno `entrega4` | `plan` revisado, sin tocar datos | Ninguno hasta aplicar |
| F3 | Despliegue: imágenes, `apply`, migraciones, semilla | URL del frontend y del backend respondiendo, recorrido de Postman y prueba de navegador | Por uso |
| F4 | Evidencias: una tarea seguida en los registros, reentrega y confirmación, escalamiento con carga controlada | Registros de Cloud Logging y métricas de Cloud Monitoring | Por uso |
| F5 | Documentación: guía de despliegue, diagrama, evidencias y video | Revisión contra la rúbrica | Ninguno |

## Entregables

| Entregable | Ubicación |
| --- | --- |
| Guía reproducible de despliegue | `docs/entrega4/despliegue.md` |
| Diagrama actualizado | `docs/entrega4/arquitectura.md` |
| Evidencias de funcionamiento y de escalamiento | `docs/entrega4/evidencias/` |
| Video de sustentación | Enlazado desde el README |
| Versión evaluada | Tag `entrega-4` |
