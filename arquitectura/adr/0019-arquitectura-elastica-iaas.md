# ADR-0019 — Arquitectura elástica IaaS para la Entrega 3

- **Estado:** Propuesto — 2026-10-21. Pasa a Aceptado cuando se hayan comprobado
  las tres verificaciones previas (D9) y el primer ciclo de escalado de cada
  grupo.
- **Fecha:** 2026-10-21
- **Requisitos:** `RT-01`, `RT-06`, `CE-01`
- **Corrige:** ADR-0016 (D1, D2 y D4). Mantiene D3, D5, D6 y D7.

## Contexto

El enunciado de la Entrega 3 cambia la pregunta. En la Entrega 2 se medía la
capacidad de dos máquinas fijas; ahora la organización necesita «responder a
variaciones en la demanda sin aumentar o reducir manualmente el número de
servidores de la API y de procesamiento asíncrono». Fija:

- **Dos grupos independientes, de 1 a 3 instancias**, uno de API y otro de
  workers, con escalamiento automático en los dos. «No satisface el componente
  de arquitectura un grupo ampliado únicamente de forma manual ni una solución
  que escale la API pero mantenga fija la capa worker.»
- **Balanceador externo con HTTPS** y verificaciones de salud delante de la API.
- **La cola fuera de los dos grupos**: «No debe crearse una cola independiente
  por réplica ni depender su disponibilidad de una máquina que el autoscaling
  pueda retirar.»
- **Arranque autónomo**: una máquina creada por un grupo queda lista «sin
  acceso manual ni ajustes posteriores».
- **Retiro seguro de workers**: el que se retira deja de tomar trabajos y
  termina los activos, o hay un reencolado verificable.

En la implementación de referencia de GCP son grupos de instancias
administrados (MIG) de Compute Engine. Siguen fuera Cloud Run, Kubernetes, CDN
y alta disponibilidad multizonal.

## Decisión

### D1 — Un entorno nuevo, `entrega3`, que hereda el estado de `entrega2`

`infra/environments/entrega3` nace como copia del directorio de la Entrega 2 y
su estado como copia del objeto `entrega2/default.tfstate` del mismo bucket.
Así la base, los buckets, los secretos, el registro y el presupuesto **pasan al
entorno nuevo sin recrearse**: el primer plan solo destruye las dos máquinas y
sus direcciones y crea lo nuevo.

`entrega2` queda congelado. `make tf ENTORNO=entrega2` se niega a correr, porque
aplicarlo pelearía con `entrega3` por los mismos recursos. Su configuración, y
por tanto la línea base, se reconstruye desde el tag `entrega-2`.

### D2 — Dos grupos y una máquina para la cola

| Pieza | Qué es | Contenedores |
| --- | --- | --- |
| `mooc-api` | MIG zonal (`us-central1-a`), 1 a 3 `e2-highcpu-2` | `caddy`, `api`, `frontend`, `swagger` |
| `mooc-worker` | MIG zonal, 1 a 3 `e2-highcpu-2` | `worker`, `worker-media`, `clamav` |
| `mooc-cola` | VM suelta `e2-small`, IP interna fija `10.10.0.30` | `redis` (AOF), `mailpit` |

El tipo de las instancias es el de la Entrega 2 para que **la capacidad por
instancia sea comparable** con la línea base; lo que cambia es cuántas hay. La
concurrencia de cada worker tampoco cambia (`critical=6,default=3`; medios
`bulk=1` con concurrencia 1): el enunciado aclara que aumentar procesos dentro
de una máquina no acredita escalamiento horizontal.

El frontend se queda junto a la API, como dejó el ADR-0017: mismo origen, sin
CORS para la API, y su versión es la de la plantilla.

Redis va en su propia máquina y no en Memorystore: unos 0,45 USD al día frente
a 1,2, y el mismo contenedor que ya se midió. Sigue sirviendo cola, sesiones,
caché y límites (ADR-0016, D2), y sigue siendo **un punto único de falla**: si
cae, caen las sesiones de todo el mundo y la cola deja de aceptar trabajos. Los
trabajos aceptados sobreviven en PostgreSQL (ADR-0004) y el AOF conserva la
cola a través de un reinicio. Se documenta como limitación, no se resuelve.

Las cuentas de servicio `mooc-web` y `mooc-worker` pasan a los grupos con sus
permisos intactos. `mooc-cola` es nueva y solo escribe registros y métricas.

### D3 — Balanceador de aplicaciones externo global con certificado gestionado

- IP global estática `mooc-lb` y nombre `<ip>.sslip.io` (ADR-0016, D4, sigue
  valiendo: no hace falta comprar dominio).
- Certificado **gestionado por Google**: ya no hay Let's Encrypt en las
  instancias. El 80 redirige al 443.
- Servicio de backend `EXTERNAL_MANAGED` hacia el puerto con nombre `http:80`
  del grupo de API, en modo `RATE`, con registro de cada petición (muestreo 1)
  para identificar qué instancia la atendió.
- **Ninguna instancia tiene IP externa.** El firewall solo deja entrar al 80 de
  la API desde los rangos del balanceador y de las verificaciones de Google
  (`130.211.0.0/22`, `35.191.0.0/16`). La salida es por Cloud NAT.

Caddy sigue en cada instancia, ahora en el 80 y sin TLS. Dos cambios:

- **`X-Forwarded-For` pasa tal cual.** El balanceador añade
  `<cliente>, <ip del balanceador>` y la API toma la IP real a un salto de la
  derecha (`TRUSTED_PROXY_HOPS=1`, como ya preveía `config.go`). En la Entrega 2
  Caddy la sobrescribía; detrás del balanceador eso haría que todos los usuarios
  compartieran la IP de un proxy de Google y el límite de tasa por IP los
  sumaría.
- **`X-Instancia`** en cada respuesta, con el nombre de la máquina: la
  evidencia del reparto sin tocar Go.

### D4 — Verificaciones de salud: preparación para el balanceador, vida para el grupo

| Verificación | Endpoint | Para qué | Intervalo / timeout / umbrales |
| --- | --- | --- | --- |
| Preparación (balanceador) | `GET /readyz` por el 80 | Comprueba Postgres, Redis y objetos. Una instancia que falla sale del reparto | 5 s / 5 s / 2 sanas, 3 fallidas |
| Vida API (autorreparación) | `GET /healthz` por el 80 | Solo que Caddy y la API responden | 12 s / 5 s / 2 sanas, 10 fallidas (~2 min) |
| Vida workers (autorreparación) | `GET /healthz` por el 8081 | Que el proceso worker responde | 12 s / 5 s / 2 sanas, 10 fallidas (~2 min) |

Separarlas es deliberado. Si cae Redis, `/readyz` falla en todas las instancias
a la vez: el balanceador las saca del reparto, pero recrearlas no arreglaría
nada y solo añadiría un bucle de reemplazos. La autorreparación usa la vida,
que solo falla cuando el proceso de esa instancia dejó de contestar. Esto es
justo lo que el enunciado pide distinguir: **exclusión del balanceador** frente
a **reparación o reemplazo por el grupo**.

La vida es más tolerante que la preparación, como recomienda Google para la
autorreparación. Con 3 fallos a 10 s, como estaba al principio, un reinicio de
la VM tardaba más que los ~30 s de margen y el grupo la recreaba con disco
nuevo: pasó en la Sesión A de la prueba del despliegue (A4). Con 10 fallos a
12 s (Google no acepta más de 10) el grupo espera ~2 min, más de lo que tarda un reinicio de la VM. Una instancia que
de verdad dejó de responder se recrea igual, solo ~2 min más tarde. El
balanceador sigue sacándola del reparto en ~15 s, así que la tolerancia no se
paga en errores para el usuario.

Al retirar una instancia, el balanceador deja de mandarle peticiones nuevas y
da 30 s a las que están en curso (`connection_draining_timeout_sec`).

### D5 — Señales de escalado

| Grupo | Señal | Valor inicial | Por qué |
| --- | --- | --- | --- |
| API | Utilización del balanceador contra `max_rate_per_instance` | 40 peticiones/s por instancia, objetivo 0,8 | En la Entrega 2 una instancia atendió 61-68 peticiones/s con p95 de 25 ms y 35 % de CPU, y se degradó a 91 **por la base**, no por la API. Un objetivo de CPU podría no dispararse nunca; las peticiones por instancia sí crecen con la demanda. Con 0,8 × 40 el grupo crece hacia las 32 por instancia, antes de la degradación |
| Workers | `custom.googleapis.com/mooc/cola_bulk_trabajos`: trabajos de `bulk` **pendientes + en curso**, métrica de grupo | 2 trabajos por instancia | FFmpeg fue el cuello de botella del escenario 2. Con un trabajo a la vez por instancia, 2 por instancia es uno en curso y uno esperando: 3 trabajos piden 2 instancias y 5 o más, 3 |

La señal de workers la **publica la máquina de la cola** cada 30 s
(`deploy/publicar-cola.sh`), no los workers: cada worker exporta la misma
profundidad y con tres instancias habría tres series iguales. Cuenta también
los trabajos **en curso** porque el enunciado advierte que una cola sin
pendientes puede tener trabajos en ejecución: con solo pendientes, el grupo
encogería mientras una instancia todavía transcodifica.

Dependencia con el *reaper* (punto 4 del alcance): si republica trabajos que
ya esperan, infla esta señal y el grupo crece por trabajos que no existen. Se
corrige en el código, no aquí.

Parámetros comunes a los dos autoscalers:

- **Límites:** mínimo 1, máximo 3. Al llegar a 3 el grupo no crece más; en la
  API sube la latencia y en los workers crece la espera en cola, y las dos cosas
  se registran.
- **Periodo de inicialización** (`cooldown_period`): el arranque medido. 180 s
  en la API y 240 s en workers con el arranque por script; se reemplazan por
  los medidos con la imagen horneada (D7).
- **Reducción:** `scale_in_control` retira como mucho una instancia cada 600 s.
  Además GCE estabiliza la reducción con el máximo de los últimos diez
  minutos, así que una bajada breve no retira nada.

Los valores son iniciales. Si una corrida no dispara el mecanismo, se revisan
la carga, la señal y estos valores, y el cambio se registra aquí y en el
informe.

### D6 — Retiro seguro de workers

Al reducir, GCE ejecuta el `shutdown-script` de la plantilla
(`deploy/apagado-worker.sh`), que hace `docker compose stop`. Compose respeta el
`stop_grace_period` de cada servicio: **300 s para `worker-media`**, porque la
transcodificación más larga de la Entrega 2 duró 196 s, y 60 s para `worker`.
El `ShutdownTimeout` de asynq en `cmd/worker` va por debajo de esos 300 s; lo
fija el frente de backend.

Si el trabajo no termina dentro del margen, asynq lo devuelve a la cola y otra
instancia lo repite. La idempotencia del worker (ADR-0008) evita derivados
duplicados. La recuperación se demuestra con un retiro controlado durante una
transcodificación.

### D7 — Arranque: script primero, imagen horneada para medir

Cada instancia arranca con el mismo `deploy/arranque.sh` de la Entrega 2,
ampliado con los papeles `web`, `worker`, `cola` y `hornear`. Es idempotente:
instala lo que falte, escribe la configuración y el `.env` (secretos de Secret
Manager con la identidad de la instancia, permisos `0600`) y levanta la
composición. Un reinicio no duplica nada. Las migraciones siguen en el arranque
de cada API, serializadas por un bloqueo consultivo (`cmd/migrate`).

Mientras se ponen en marcha los grupos y el balanceador, las plantillas usan
Debian 12 limpio. **Antes de la primera corrida medida** se hornea una imagen
(`make hornear-imagen`) con Docker, el agente de operaciones y todas las
imágenes de contenedor ya descargadas, y las plantillas pasan a ella. Desde ese
momento el método de arranque no cambia entre corridas: el tiempo de arranque es
parte de lo que se mide.

La imagen se crea con el mismo `arranque.sh` en modo `hornear`, así que lo que
lleva es exactamente lo que el arranque instalaría. Su nombre lleva el SHA de
las imágenes (`mooc-base-<sha>`), y las instancias llevan la etiqueta
`version`: todas las de una corrida ejecutan el mismo código.

El arranque registra en el diario cuánto tardó en dejar todos los contenedores
sanos. La aplicación está lista cuando la API pasa `/readyz` en el balanceador
o cuando el worker toma su primer trabajo.

### D8 — Operación y costo

- `entorno_encendido = false` deja los grupos en cero, detiene la cola y borra
  las dos reglas de reenvío, que cobran por hora aunque no haya tráfico. La IP
  y el certificado se quedan: la URL pública no cambia entre sesiones.
- `autoescalado = false` fija cada grupo en una instancia, sin autoscaler. Es
  solo para la puesta en marcha; **no se activa con más de una instancia por
  grupo hasta repartir los pools de conexiones** (punto 3 del alcance): con los
  valores de la Entrega 2, tres instancias de cada grupo pedirían 135
  conexiones contra las ~47 de `db-g1-small`.
- Un tablero de Cloud Monitoring (`tablero.tf`) muestra en una pantalla las
  instancias de cada grupo, la señal de workers, las peticiones y la latencia
  del balanceador y la CPU de cada grupo: la línea de tiempo de 3A y 3B.

### D9 — Verificaciones previas

Tres cosas se comprueban en la nube antes de dar el ADR por aceptado. Cada una
tiene una alternativa si falla.

| Verificación | Si falla |
| --- | --- |
| El autoscaler acepta una métrica de grupo de `custom.googleapis.com` sobre el recurso `global` con `single_instance_assignment` | Publicar la métrica sobre el `gce_instance` de `mooc-cola` y filtrar por su etiqueta |
| Al retirar una instancia, el `shutdown-script` dispone de los 300 s | GCE da unos 90 s a una instancia normal. Si no hay más margen, el apagado ordenado cubre los trabajos cortos y los largos se recuperan por reencolado (D6), que el enunciado admite. Se puede activar el apagado ordenado de GCE (`graceful_shutdown`) si el proveedor de Terraform lo expone |
| Google emite el certificado gestionado para `<ip>.sslip.io` | Dominio propio en `nombre_publico` |

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| Memorystore para Redis | Administrado, sin máquina que cuidar | ~1,2 USD al día frente a ~0,45; otra pieza que la línea base no midió | El presupuesto es de 50 USD y Redis no fue el cuello de botella en la Entrega 2 |
| Redis en el grupo de workers | Una máquina menos | El autoscaler podría retirarlo; el enunciado lo prohíbe | — |
| CPU como señal de la API | Lo más sencillo; viene por defecto | En la Entrega 2 la CPU no pasó del 60 % mientras la base se saturaba | Podría no escalar nunca |
| `queue_depth` de los workers como señal | Ya existe y llega a Cloud Monitoring | Una serie por instancia con el mismo valor; solo pendientes | Dividiría la cola entre instancias que miden lo mismo y encogería con trabajos en curso |
| Antigüedad de la cola como señal | Mide la espera que siente el usuario | Crece con retraso y no dice cuántas instancias hacen falta | La profundidad se traduce directa en instancias |
| Imagen horneada desde el primer día | Sin cambio a mitad de camino | Pone la imagen en el camino crítico antes de probar nada | Se hornea antes de medir, que es cuando importa |
| Mantener el `entrega2` y cambiarlo en sitio | Sin copiar estado | La configuración de la línea base solo quedaría en el tag; el directorio mentiría | Un entorno por forma de despliegue |
| Grupo regional (multizona) | Sobrevive a la caída de una zona | Más instancias mínimas y el enunciado no lo pide | Fuera de alcance |

## Consecuencias

**A favor:**

- Cada capa crece con su propia demanda: la API con las peticiones, los workers
  con la cola de medios. Se pueden observar por separado, que es lo que piden
  3A y 3B.
- La superficie pública se reduce al balanceador. Ninguna instancia tiene IP
  externa.
- El certificado deja de depender de Let's Encrypt y de su límite de cinco por
  semana: cada instancia nueva no pide uno.
- Se reutilizan las imágenes, las composiciones y el script de arranque de la
  Entrega 2. Lo que se compara es la arquitectura, no otro código.

**En contra:**

- **Puntos únicos de falla:** la máquina de la cola y Cloud SQL zonal. Si
  cualquiera cae, la plataforma cae, por muchas instancias que haya.
- **La base es ahora el límite más probable.** Escalar la API multiplica pools
  contra una base de núcleo compartido que ya fue el cuello de botella del
  escenario 1. Lo esperable es que el límite se desplace a la base, y el
  informe tiene que mostrarlo, no esconderlo.
- **Más costo fijo:** dos reglas de reenvío, la IP global y la máquina de la
  cola cobran aunque no haya carga. Se apagan juntas.
- **El tiempo de arranque es visible:** una instancia nueva tarda minutos en
  atender. Durante ese tiempo la latencia sube, y eso es parte de lo que se
  mide.

## Cómo se verifica

- `make tf ENTORNO=entrega3 ARGS=plan`, recién copiado el estado, no cambia la
  base, los secretos ni el registro, y de los buckets solo actualiza el CORS.
  Tras `apply`, un segundo plan
  sale vacío.
- `make instancias`: ninguna instancia de los grupos ni la cola tiene IP
  externa, y todas las de un grupo llevan la misma etiqueta `version`.
- `https://<ip>.sslip.io/healthz` responde con el certificado de Google y
  `http://` redirige.
- Con dos instancias de API, `curl -sI` repetido muestra al menos dos
  `X-Instancia` distintos.
- Detener `api` en una instancia la saca del reparto sin que el grupo la
  recree; borrar la máquina hace que el grupo cree otra.
- 3A y 3B: el tablero muestra la señal subiendo, una instancia nueva creada y
  atendiendo o consumiendo, y la reducción posterior.
- Retiro de un worker durante una transcodificación: el trabajo termina o se
  repite una sola vez, con un único juego de derivados.
