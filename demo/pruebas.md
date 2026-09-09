# Cómo probar la aplicación

**No hay interfaz de usuario.** El profesor relevó el frontend para esta
entrega, así que no existe pantalla de registro ni de login: se prueba por API.
El frontend llega en la Entrega 2.

Lo que sí hay es **Swagger UI en http://localhost:8090/docs**, con las 88
operaciones navegables y ejecutables. No es la aplicación: es la documentación
del contrato, y sirve para probarlo todo sin escribir un `curl`.

Tres formas, de menos a más manual. Si solo tienes cinco minutos, ve al
apartado 1.

**Índice:** [Arrancar](#arrancar) · [1 · Automático](#1--automático-2-minutos) ·
[2 · Swagger UI](#2--swagger-ui) · [3 · Postman](#3--postman) ·
[Subir un vídeo](#subir-un-vídeo) · [4 · A mano](#4--a-mano-con-curl) ·
[Lo que te va a morder](#lo-que-te-va-a-morder) ·
[Si algo falla](#si-algo-falla)

## Arrancar

```bash
cd proyecto-1-mooc
make clean          # borra volúmenes: se empieza de cero
make up             # construye y levanta; la primera vez tarda unos minutos
make seed           # 8 cuentas sintéticas
```

| Servicio | URL |
| --- | --- |
| **Swagger UI** | **http://localhost:8090/docs** |
| API | http://localhost:8090/readyz |
| Mailpit (correo) | http://localhost:8026 |
| MinIO | http://localhost:9011 |
| Grafana | http://localhost:3002 *(solo tras `make obs`)* |
| Prometheus | http://localhost:9091 *(idem)* |

**Ojo con el puerto: 8090, no 8080.** El 8080 lo ocupa otro stack de la
máquina, así que este proyecto usa un rango propio.

Comprueba que todo está sano antes de seguir:

```bash
docker compose ps          # los 9 servicios en healthy
curl -s localhost:8090/readyz
```

### Las cuentas

Contraseña única para todas: `Contrasena-Demo-2026`

| Correo | Rol | Para qué |
| --- | --- | --- |
| `admin@mooc.local` | admin | Administración y auditoría |
| `profesor@mooc.local` | profesor | Crear y publicar cursos |
| `profesora2@mooc.local` | profesor | Probar el control por propiedad |
| `estudiante1@mooc.local` | estudiante | Recorrido feliz |
| `estudiante2@mooc.local` | estudiante | Segundo inscrito |
| `estudiante3@mooc.local` | estudiante | Señales fraudulentas |
| `sinverificar@mooc.local` | estudiante | No debe poder entrar |
| `suspendido@mooc.local` | estudiante | No debe poder entrar |

## 1 · Automático (2 minutos)

Lo más rápido y lo más completo. Recorre el flujo entero y comprueba cada
condición del enunciado.

```bash
make demo
```

Verás 33 aserciones. Cada una prueba algo concreto: que la validación devuelve
todos los errores, que la versión publicada es inmutable, que el snapshot del
quiz no lleva las claves, que enviar el porcentaje desde el cliente se rechaza y
se audita, que el reaper reejecuta un trabajo sin duplicar la insignia.

Otros dos:

```bash
make invariantes   # ~1 min: lo que prometen los ADR y `make demo` no cubre
make ci            # ~8 min: TODO lo que corre el CI, en local
```

`make invariantes` es el que prueba lo más difícil de ver a mano: que el
progreso de un estudiante **sobrevive a publicar una versión nueva del curso**.

## 2 · Swagger UI

http://localhost:8090/docs

Las 88 operaciones del contrato, navegables y **ejecutables**. Va detrás del
mismo proxy que la API, así que comparten origen y el botón «Try it out» llama
a los endpoints de verdad, no a un simulador.

Para usarlo con sesión:

1. Despliega `POST /api/v1/auth/login`, «Try it out», y envía
   `{"email":"profesor@mooc.local","password":"Contrasena-Demo-2026"}`.
2. Copia el `token` de la respuesta.
3. Botón **Authorize** arriba a la derecha, pégalo y confirma.

A partir de ahí todas las peticiones van firmadas. El token se conserva entre
recargas.

Las operaciones que aún no responden llevan la marca `x-estado: planificado` en
su descripción.

## 3 · Postman

Es la interfaz de la demostración.

```bash
make postman            # segmentos rápidos, ~30 s
make postman-completo   # todo, con 11 s entre peticiones (~8 min)
```

O a mano: importa `postman/mooc.postman_collection.json` y
`postman/mooc.postman_environment.json`, elige el entorno **MOOC local** y
ejecuta las carpetas **en orden** — encadenan variables entre sí.

**SEG-7 necesita paciencia:** el servidor exige 10 segundos entre heartbeats, a
propósito. Con el Collection Runner hay que poner `--delay-request 11000`, o
ejecutar esas peticiones a mano esperando entre una y otra.

## Subir un vídeo

No hay pantalla para arrastrar un archivo. La carga es multipart directa al
almacén —la API solo firma las URLs y los bytes nunca pasan por ella—, así que
hay que declarar el archivo, subir cada parte, recoger su ETag y completar.

### Desde Postman (carpeta SEG-3)

`postman/archivo-de-prueba.mp4` viene en el repositorio: son 256 KiB con
contenido determinista, de modo que **cabe en una sola parte** y su SHA-256 es
siempre el mismo. Eso importa porque `POST /assets/init` exige el hash por
adelantado y Postman no sabe calcular el de un archivo local.

Ejecuta la carpeta **SEG-3 · Carga multimedia** entera. Un único paso es
manual: en la petición **«2 · Subir la parte 1»**, pestaña *Body* → *binary*,
elige `postman/archivo-de-prueba.mp4`. Postman guarda la ruta pero no el
archivo, así que hay que señalarlo una vez en cada máquina.

Las siete peticiones muestran el flujo completo: declaración, subida al
almacén, estado de la carga (lo que permite reanudar), completado, verificación
del worker, descarga y un rechazo por checksum que no cuadra.

Dos detalles que valen la pena en la demostración:

- **«5 · Verificación del worker» reintenta sola.** `complete` responde `202`,
  no `200`: la verificación la hace otro proceso. La petición sondea hasta
  doce veces hasta ver `clean`. Si se queda en `uploaded`, el worker no está
  corriendo (`docker compose logs worker`).
- **«6 · Descargar» no sigue la redirección, a propósito.** La URL prefirmada
  lleva la credencial en la query; si además se reenvía la cabecera
  `Authorization`, S3 la rechaza con `400` porque solo admite un mecanismo de
  autenticación a la vez. La prueba comprueba el `302` y que el `Location`
  venga firmado y con caducidad.

### Desde la terminal

Con cualquier archivo tuyo, hay un guion que hace lo mismo de principio a fin:

```bash
make subir ARCHIVO=mi-video.mp4

# o directamente, indicando el tipo
./scripts/subir-video.sh documento.pdf pdf
```

Verás los seis pasos: declaración, URLs prefirmadas, subida por partes, estado
de la carga (lo que permite reanudar), completado y verificación. A diferencia
de la carpeta de Postman, este guion sí calcula el hash y parte el archivo, así
que acepta vídeos de cualquier tamaño.

Lo interesante está en el paso 6: el servidor **recalcula el SHA-256 leyendo
del bucket** y **detecta el tipo real por los bytes**, no por la extensión.
Pruébalo con un PDF renombrado a `.mp4` y declarado como vídeo: se rechaza y va
a cuarentena.

**Lo que no puedes hacer es reproducirlo.** No hay transcodificación a HLS ni
escaneo antimalware; son los pasos 2 y 3 de `arquitectura/media-plan.md`. Sí
puedes descargarlo por URL firmada:

```bash
curl -L -H "Authorization: Bearer $TOKEN" \
  http://localhost:8090/api/v1/assets/<asset_id>/content -o descargado.mp4
```

## 4 · A mano con curl

```bash
API=http://localhost:8090
CLAVE=Contrasena-Demo-2026

# Iniciar sesión
TOKEN=$(curl -s -X POST $API/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"profesor@mooc.local\",\"password\":\"$CLAVE\"}" \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["token"])')

# Crear un curso  (¡la cabecera Idempotency-Key es obligatoria!)
curl -s -X POST $API/api/v1/courses \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $(cat /proc/sys/kernel/random/uuid)" \
  -d '{"title":"Mi curso","summary":"Prueba","category":"cloud","language":"es"}'
```

El contrato completo está en http://localhost:8090/openapi.yaml — la API sirve
su propia documentación.

## Lo que te va a morder

Cinco comportamientos que **son correctos** pero parecen fallos si no los
esperas:

**1 · `400 idempotency_key_required`.** Once operaciones exigen la cabecera
`Idempotency-Key`: crear curso, versionar, publicar, inscribirse, iniciar y
enviar un intento, alta de usuario, reencolar, iniciar y completar una carga, y
revocar una insignia. Cualquier valor único de 8 o más caracteres sirve.

**2 · `428 if_match_required`.** El `PATCH` de contenido exige `If-Match` con el
ETag que devolvió la lectura. Es lo que impide que dos pestañas se pisen.

**3 · `reject_reason: "cadence"` en los heartbeats.** El servidor exige 10
segundos entre uno y otro. Inundar de heartbeats no debe simular permanencia.
Espera 11 segundos entre llamadas.

**4 · `429` tras cinco intentos de login fallidos.** Es por cuenta, no por IP:
otra cuenta desde la misma máquina entra sin problema. Espera un minuto.

**5 · La insignia tarda unos segundos.** La emite un worker, no la petición.
Consulta `GET /api/v1/me/badges` un par de veces.

Y uno que **sí es un límite real**: no se puede subir un vídeo y verlo. La carga
funciona —con reanudación y verificación de checksum y tipo—, pero no hay
transcodificación a HLS ni escaneo antimalware. Está en
`arquitectura/media-plan.md`.

## Si algo falla

| Síntoma | Qué mirar |
| --- | --- |
| Un servicio no arranca | `docker compose logs <servicio> --tail=50`. Casi siempre es un puerto ocupado; se cambian en `.env` |
| `500` sin explicación | `docker compose logs api \| grep "error interno"` — trae la causa y el `trace_id` |
| El correo no llega | `docker compose logs worker --tail=30` y comprueba que `worker` esté `healthy` |
| Todo raro tras muchas pruebas | `make clean && make up && make seed` deja la base a cero |

Comandos útiles para mirar por dentro:

```bash
make jobs      # el registro de trabajos: outbox, idempotencia y DLQ
make audit     # los últimos eventos de auditoría
make psql      # consola de PostgreSQL
make logs      # logs de api y worker
```
