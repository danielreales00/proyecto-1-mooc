# Plataforma MOOC

Plataforma web de cursos masivos abiertos en línea. Un profesor autorizado crea
un curso estructurado en módulos, unidades y recursos, y lo publica como una
versión inmutable. Un estudiante se registra, se inscribe, consume el contenido
y presenta evaluaciones. El servidor calcula su avance a partir de evidencias
—nunca de lo que el cliente afirme— y, al aprobar, un worker emite una insignia
digital con una URL pública de verificación.

Backend en Go como monolito modular con workers independientes, sobre
PostgreSQL, Redis y almacenamiento de objetos. Todo se ejecuta en contenedores.

**Operación:** [Cómo usar](#cómo-usar) · [Comandos make](#comandos-make) ·
[Configuración](#configuración) · [Arquitectura](#arquitectura) ·
[Estructura del proyecto](#estructura-del-proyecto)

**Qué hace:** [El flujo completo](#el-flujo-completo) ·
[Autoría y publicación](#autoría-y-publicación) ·
[Evaluación](#evaluación) · [Progreso e insignias](#progreso-e-insignias) ·
[La API](#la-api)

**Cómo se comporta bajo estrés:**
[Idempotencia y tolerancia a fallos](#idempotencia-y-tolerancia-a-fallos) ·
[Escalamiento horizontal](#escalamiento-horizontal) ·
[Probar el stack levantado](#probar-el-stack-levantado)

**Entrega:** [Qué está hecho y qué no](#qué-está-hecho-y-qué-no) ·
[Guion de la demostración](demo/guion.md)

## Cómo usar

Requiere Docker y Docker Compose. **No hace falta tener Go instalado**: todo se
compila dentro de contenedores.

```bash
make up      # copia .env.example a .env si no existe, construye y levanta
make seed    # 8 cuentas sintéticas para la demostración
make demo    # recorre el flujo completo y comprueba cada condición
```

Una vez en ejecución:

| Servicio | URL |
| --- | --- |
| API (tras el proxy) | http://localhost:8090/readyz |
| Mailpit (correo) | http://localhost:8026 |
| MinIO (objetos) | http://localhost:9011 |

La API **no publica puerto propio**: la única entrada es el proxy. Eso es lo que
permite `--scale api=3` sin colisiones de puerto.

Los puertos del host se configuran en `.env` y usan un rango propio del
proyecto, para poder convivir con otros stacks de Docker en la misma máquina.

### Cuentas sintéticas

`make seed` es idempotente: se puede correr las veces que haga falta.
Contraseña única para todas: `Contrasena-Demo-2026`.

| Correo | Rol | Para qué |
| --- | --- | --- |
| `admin@mooc.local` | admin | Administra usuarios y auditoría |
| `profesor@mooc.local` | profesor | Crea y publica cursos |
| `profesora2@mooc.local` | profesor | Probar el control por propiedad |
| `estudiante1@mooc.local` | estudiante | Recorrido feliz |
| `estudiante2@mooc.local` | estudiante | Segundo inscrito |
| `estudiante3@mooc.local` | estudiante | Señales de progreso fraudulentas |
| `sinverificar@mooc.local` | estudiante | Sin verificar: no debe poder entrar |
| `suspendido@mooc.local` | estudiante | Suspendido: no debe poder entrar |

## Comandos make

```
make up          Levanta el stack completo y aplica migraciones
make down        Detiene, conservando los volúmenes
make clean       Detiene y borra los volúmenes
make seed        Carga las cuentas sintéticas
make demo        Recorrido completo de la demostración (33 aserciones)
make smoke       Comprobación rápida de identidad
make postman     Colección de Postman, segmentos rápidos
make postman-completo  La colección entera, con 11 s entre peticiones
make scale       3 instancias de API y 3 de worker
make test        Pruebas unitarias con detector de carreras
make cover       Pruebas con informe de cobertura
make sec         govulncheck, gosec y búsqueda de credenciales
make openapi     Valida el contrato con redocly
make contrato    Comprueba que el contrato y la API no se hayan separado
make logs        Sigue los logs de api y worker
make jobs        Muestra el registro de trabajos
make audit       Muestra los últimos eventos de auditoría
make psql        Consola de PostgreSQL
```

## Configuración

Todo sale del entorno: no hay ficheros de configuración por entorno ni valores
de producción en el código. `.env.example` está comentado y es el punto de
partida; `.env` no se versiona.

Las variables que más se tocan:

| Variable | Para qué |
| --- | --- |
| `API_PORT`, `MAILPIT_UI_PORT`, `MINIO_*_PORT` | Puertos publicados en el host |
| `SESSION_TTL` | Vida de la sesión, con TTL deslizante |
| `VERIFY_TOKEN_TTL` | Caducidad del token de verificación de correo |
| `WORKER_QUEUES` | Pesos por cola: `critical=6,default=3,bulk=1` |
| `WORKER_CONCURRENCY` | Trabajos simultáneos por worker |

## Arquitectura

```
                    ┌─────────┐
   Postman ────────▶│  proxy  │  única entrada; la API no publica puerto
                    └────┬────┘
                         │  reparto por turnos
           ┌─────────────┼─────────────┐
           ▼             ▼             ▼
        ┌──────┐     ┌──────┐     ┌──────┐
        │ api  │     │ api  │     │ api  │   sin estado
        └───┬──┘     └───┬──┘     └───┬──┘
            └────────────┼────────────┘
                         │
     ┌───────────────────┼───────────────────┐
     ▼                   ▼                   ▼
┌──────────┐       ┌──────────┐       ┌──────────┐
│PostgreSQL│       │  Redis   │       │  MinIO   │
│ verdad   │       │ sesiones │       │ objetos  │
│transacc. │       │ + cola   │       │          │
└────┬─────┘       └────┬─────┘       └──────────┘
     │                  │
     │                  ▼
     │            ┌──────────┐      ┌──────────┐
     └───────────▶│  worker  │─────▶│ Mailpit  │
       job_runs   └──────────┘      └──────────┘
```

Un esquema de PostgreSQL por módulo de dominio: `identity`, `authoring`,
`media`, `assessment`, `progress`, `badges`, `audit` y `platform`. La frontera
entre módulos se ve en la base, no solo en el árbol de paquetes.

Las decisiones y sus alternativas descartadas están en
[`arquitectura/adr/`](arquitectura/adr/README.md), quince documentos.

## El flujo completo

```
registro ──▶ correo (worker) ──▶ verificación ──▶ login
                                                    │
   profesor: curso ▸ módulo ▸ unidad ▸ recurso ▸ publicar
                                                    │
                                        catálogo ◀──┘
                                            │
   estudiante: inscripción ▸ consumo ▸ quiz ▸ evidencias de progreso
                                                    │
                              aprobación ──▶ badge.issue (worker)
                                                    │
                                   URL pública de verificación
```

`make demo` lo recorre entero y comprueba cada condición del enunciado.

## Autoría y publicación

Un curso contiene **versiones numeradas**. Cada nodo del árbol lleva dos
identificadores: el `id` de la fila, que cambia en cada versión, y el
`stable_id`, que es la identidad pedagógica y se copia al clonar. Todo lo que
registra actividad del estudiante referencia el `stable_id`, y por eso el
progreso sobrevive a la publicación de una versión nueva.

**Una versión publicada es inmutable.** No lo impide Go: lo impide un trigger de
PostgreSQL sobre módulos, unidades y recursos. La regla vive donde están los
datos, porque es la que más fácil se salta un `UPDATE` de mantenimiento. Para
editar hay que despublicar, como permite el §5.1 del enunciado.

**La validación de publicación no para en el primer error.** Recorre todas las
reglas —metadatos, estructura mínima, disponibilidad de los binarios, criterios
de aprobación, quizzes con opción correcta, iframes en la lista blanca— y
devuelve la lista completa en un solo `422`.

El contenido textual se guarda como **Markdown canónico**: el servidor no
guarda lo que le mandan, sino que parsea, valida y vuelve a serializar. Guardar
dos veces lo mismo produce byte a byte lo mismo, y por eso el `ETag` es estable
y el autosave no genera revisiones falsas. El HTML crudo se descarta en el
parseo y lo renderizado se sanea con lista blanca: dos barreras contra XSS.

## Evaluación

Al iniciar un intento se **congela un snapshot** de las preguntas. Si el
profesor edita el quiz después, el estudiante sigue viendo el mismo examen y la
nota es reproducible.

**La clave correcta nunca sale del servidor por las rutas del estudiante.** No
se resuelve con un `omitempty`: hay dos tipos distintos en Go, y el del
estudiante no tiene el campo. Cada opción que ve el estudiante trae exactamente
`stable_id` y `text_md`.

La nota se calcula en el servidor contra las claves de la base. Reenviar el
intento devuelve la misma nota sin recalcular. La retroalimentación depende de
la política del quiz, y `full` degrada a `correctness` mientras al estudiante le
queden intentos: revelar la clave con intentos por delante convierte el examen
en un ejercicio de memoria.

## Progreso e insignias

**El cliente reporta evidencias, nunca conclusiones.** El cuerpo de
`POST /progress` no tiene ni puede tener un campo de porcentaje. Si llega
`progress_percent`, la respuesta es `422` y el intento queda **auditado** con el
cuerpo recibido y quién lo envió.

El servidor descarta además la evidencia sospechosa:

| Regla | Umbral |
| --- | --- |
| Cadencia mínima entre heartbeats | 10 s de reloj del servidor |
| Avance de posición por heartbeat | ≤ 2× el tiempo real transcurrido |
| Posición dentro de la duración | `0 ≤ pos ≤ duración + 2 s` |
| Apertura antes del primer heartbeat | obligatoria |

El `client_ts` se registra para diagnóstico pero **no interviene en ningún
cálculo**: el reloj lo pone el servidor.

El porcentaje sale de los recursos obligatorios y visibles; los opcionales no
cuentan. Al aprobar se registra `badge.issue` en la misma transacción y se
publica en la cola después del commit. La insignia es única por inscripción, con
dos barreras: la clave determinista del trabajo y una restricción `UNIQUE` en la
base. Su URL pública **no expone el correo del estudiante**.

## Límites de tasa

El login lleva **dos niveles a la vez**: 5 por minuto y cuenta, y 60 por minuto
e IP. Limitar solo por IP castigaría a una universidad entera detrás de un NAT;
limitar solo por cuenta dejaría rociar contraseñas contra muchas cuentas desde
una sola máquina. Con los dos, un ataque contra una cuenta se frena en cinco
intentos y el vecino de al lado sigue entrando.

También se limitan el registro, la verificación de correo y la ingesta de
evidencias de progreso, esta última por sesión y no por IP.

Las respuestas llevan `RateLimit-Limit`, `RateLimit-Remaining` y
`RateLimit-Reset`; el `429` añade `Retry-After`. **Si Redis falla, la petición
se permite**: un limitador caído no debe dejar el servicio inaccesible.

## Idempotencia y tolerancia a fallos

### El trabajo vive en PostgreSQL, no en Redis

Cada trabajo deja una fila en `platform.job_runs` **en la misma transacción**
que el cambio de dominio que lo origina. La publicación en la cola ocurre
después del commit. Si esa publicación falla —Redis caído, proceso muerto entre
commit y publish— la fila queda en `queued` y el **reaper** la republica.

### Una entrega duplicada no produce dos salidas

Antes de trabajar, el worker reclama la ejecución con una sola sentencia:

```sql
UPDATE platform.job_runs SET status='running', attempt=attempt+1
 WHERE job_key=$1 AND status IN ('queued','failed')
RETURNING true;
```

Sin fila devuelta, otro worker lo tiene o ya terminó, y el trabajo se descarta.

`make demo` lo demuestra sin trampa: devuelve el trabajo de la insignia a la
cola, espera a que el reaper lo republique, comprueba que el worker lo
**reejecuta** —se ve el `intento 2`— y que sigue habiendo **una sola insignia**.
Sin el reaper, esa prueba pasaría por el motivo equivocado: porque nadie lo
ejecutó.

### Reintentos y DLQ

Tres intentos con backoff exponencial. Al agotarse, el trabajo pasa a `dead` y
se emite una alerta. Un reencolado administrativo conserva la misma `job_key`.

## Observabilidad

```bash
make obs    # Prometheus en :9091, Grafana en :3002
```

El panel «MOOC · Operación» trae ocho gráficas, y ninguna está de adorno: cada
una sostiene un objetivo concreto. Latencia p95 por ruta, entregas duplicadas
descartadas, evidencias de progreso rechazadas por motivo, peticiones frenadas
por límite de tasa, y la dead-letter queue.

Cuatro reglas de alerta. La que importa es `TrabajoEnDeadLetterQueue`, que
implementa literalmente el §6 del enunciado: *tras tres reintentos fallidos, el
trabajo llega a la DLQ y emite una alerta*.

**Los contadores se declaran en cero al arrancar.** Sin eso, `increase()` no
detecta el salto de una serie que no existía a su primer valor, y la alerta se
quedaría muda justo el día que hace falta. Se descubrió probándola: la primera
versión no disparaba.

## Escalamiento horizontal

```bash
make scale     # docker compose up -d --scale api=3 --scale worker=3
make demo      # el mismo recorrido, con tres instancias
```

Nada guarda estado en el proceso: las sesiones viven en Redis, los datos en
PostgreSQL y los binarios en el almacén de objetos. La sesión se abre en una
instancia y `GET /me` la resuelve en otra.

## Probar el stack levantado

Las pruebas unitarias corren contra dobles y no tocan la infraestructura. Lo que
hay que demostrar solo se ve con todo levantado.

```bash
make demo       # ~2 min: el flujo completo, 33 aserciones
make smoke      # ~20 s: identidad de punta a punta
make postman    # ~30 s: la colección, segmentos rápidos
make contrato   # el contrato y la API no se han separado
make sec        # govulncheck, gosec y credenciales
```

`make demo` sondea en vez de esperar un tiempo fijo: el worker tarda lo que
tarde, y con un `sleep` la prueba fallaría sin que falle nada.

`make arch` comprueba sobre el propio código que el dominio no importa
infraestructura, que ningún módulo escribe en el esquema de otro, y que no ha
entrado ninguna dependencia fuera de la lista del ADR-0002.

`make invariantes` prueba contra el sistema en marcha lo que los ADR prometen y
`make demo` no cubre: que el `stable_id` sobreviva a una versión nueva del
curso, que la clave del quiz no salga por ninguna ruta del estudiante, que el
token de sesión no esté en claro en Redis, y que los buckets privados no
respondan sin firma.

`make contrato` comprueba dos cosas contra la API en marcha: que sirva
exactamente el contrato del repositorio, y que **toda ruta declarada como
implementada exista de verdad en el router**.

## La API

Contrato completo en [`backend/openapi/openapi.yaml`](backend/openapi/openapi.yaml):
las 88 operaciones del inventario, validadas con redocly. Las que aún no
responden llevan `x-estado: planificado`.

La API sirve su propio contrato en `GET /openapi.yaml`, empotrado en el binario:
si el archivo desaparece del árbol de fuentes, no compila.

Convenciones: token opaco en `Authorization: Bearer`, errores uniformes en
`application/problem+json` (RFC 9457) con `trace_id`, paginación por cursor,
`ETag`/`If-Match` en lo editable e `Idempotency-Key` en lo que no es repetible.

## Qué está hecho y qué no

**61 de las 88 operaciones** responden hoy.

| Módulo | Estado |
| --- | --- |
| `identity` | Registro, verificación, login, sesiones. Falta recuperación de contraseña |
| `audit` | Completo, inmutable por trigger |
| `authoring` | Completo: jerarquía, publicación, inmutabilidad, versiones |
| `learning` | Completo: catálogo, inscripción, progreso, aprobación |
| `assessment` | Completo: quizzes, intentos, calificación |
| `badges` | Completo: emisión, verificación pública, revocación |
| `media` | Carga multipart reanudable y verificación en servidor. Faltan ClamAV y FFmpeg; ver [`media-plan.md`](arquitectura/media-plan.md) |
| `admin` | Completo: invitación de profesores, roles, estados, sesiones ajenas, auditoría y cola |

Trabajos asíncronos: `email.send`, `badge.issue` y el `reaper`. Faltan los de
medios.

El desglose completo módulo a módulo está en
[`arquitectura/estado-modulos.md`](arquitectura/estado-modulos.md).
El detalle de lo que falta de plataforma y evidencia, en
[`arquitectura/pendientes.md`](arquitectura/pendientes.md).

## Estructura del proyecto

```
demo/             guion del video
arquitectura/     enunciado, requisitos, ADRs, diseños, pendientes
backend/
  cmd/            api · worker · migrate · seed
  internal/
    platform/     config · logging · problem · httpx · ids · jobs
                  passwords · dbx · markdown
    adapters/     postgres · rediscli · sessions · objectstore
                  mailer · queue
    modules/      identity · audit · authoring · learning
                  assessment · badges
  migrations/     SQL versionado, hacia adelante
  openapi/        openapi.yaml, servido en GET /openapi.yaml
postman/          colección de la demostración
scripts/          demo.sh · smoke.sh · contrato.sh · sin-credenciales.sh
deploy/           Caddyfile del proxy
.github/          CI: build, lint, seguridad, migraciones, pruebas, E2E
```
