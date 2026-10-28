# Pruebas de carga. Entrega 3

Informe de capacidad de la plataforma MOOC sobre la arquitectura elástica de
Google Cloud: dos grupos de instancias que escalan solos, un balanceador HTTPS
y la cola en su propia máquina. Repite los dos escenarios de la Entrega 2 y
añade un tercero dedicado al autoscaling. La línea base es el
[informe de la Entrega 2](pruebas_de_carga_entrega2.md), medido con el tag
`entrega-2`.

> **Estado del documento.** Las secciones 2 a 7 y 10 están cerradas: fijan qué
> se mide y cómo, antes de medir. Las marcadas como pendientes se completan con
> la sesión de medición.

## 1. Conclusiones

**Del escenario 1, medido el 28 de octubre.** Los escenarios 2 y 3 siguen
pendientes.

1. **El autoscaling de la API mueve el codo de la curva, y mucho.** A 91
   peticiones/s la Entrega 2 daba un p95 de 272 ms; la arquitectura elástica da
   **28 ms**, diez veces mejor, con la misma base de datos y las mismas
   imágenes. De 30 a 91 peticiones/s el p95 no se mueve del intervalo 26–31 ms.

2. **El límite no ha desaparecido: se ha mudado al disco de la base.** A 117
   peticiones/s la plataforma satura con un p95 de 5,5 s. La CPU de la API va al
   19–21 % con tres instancias, las conexiones llegan a 31 de 47 y Redis atiende
   659 órdenes/s: ninguno es techo. El techo son las **escrituras de disco de
   Cloud SQL**, que se aplanan en 206–339 por segundo justo cuando la carga sube
   un 30 % más. La instancia tiene un PD-SSD de **10 GB**, y un SSD de Google da
   30 escrituras por segundo y gigabyte: **300**.

3. **Saturar no es fallar.** Ni un `5xx` ni un *timeout* en las nueve corridas,
   incluidas las dos saturadas, y el 100 % de las comprobaciones funcionales.
   La plataforma se vuelve lenta, no incorrecta.

4. **La integridad aguanta el escalado.** 940 aserciones de integridad sin un
   fallo, con el grupo creciendo y decreciendo durante las corridas: el snapshot
   no filtra la clave, el intento ajeno se rechaza, el envío simultáneo produce
   una sola calificación, el reenvío es idempotente y el porcentaje enviado por
   el cliente se rechaza.

5. **La propuesta de evolución es agrandar el disco de la base, y es barata.**
   En Cloud SQL las operaciones por segundo escalan con el tamaño del disco, así
   que pasar de 10 GB a 100 GB multiplica por diez el techo de escritura —de 300
   a 3.000— por unos pocos dólares al mes, sin tocar el `tier` ni una línea de
   código. El disco está al **4 % de ocupación**: se agrandaría por rendimiento,
   no por espacio, que es justo el ajuste que no se descubre sin medir. Subir a
   un núcleo dedicado cuesta mucho más y no ataca lo medido.

## 2. Condiciones fijas y diferencias con la línea base

| Elemento | Entrega 2 | Entrega 3 |
| --- | --- | --- |
| Región y zona | `us-central1`, `us-central1-a` | Igual |
| Entrada | Caddy con TLS en el Web Server | Balanceador externo con certificado gestionado, modo `RATE` |
| API | Una `e2-highcpu-2` | Grupo de 1 a 3 `e2-highcpu-2`, objetivo 0,8 de 40 peticiones/s por instancia |
| Workers | Una `e2-highcpu-2` con `worker` y `worker-media` | Grupo de 1 a 3 `e2-highcpu-2`, 2 trabajos de `bulk` por instancia |
| Concurrencia por instancia | `worker` 20 en `critical=6,default=3`, `worker-media` 1 en `bulk=1` | Igual |
| Redis | En el Worker Server | En `mooc-cola`, `e2-small` dedicada |
| Base de datos | Cloud SQL `db-g1-small`, zonal | Igual |
| Pool por proceso | API 25, worker 10 | API 8, worker 3. Repartido para 3 + 3 instancias contra las 47 conexiones útiles |
| Arranque de instancias | Máquinas fijas | Imagen horneada `mooc-base-<sha>` |
| Límites de tasa por IP | Multiplicados por 100 | Igual |
| Generador | `e2-standard-4` con IP externa, en `us-central1-a` | Igual |
| Herramienta | k6 2.3.0 | Igual |
| Guiones | Tag `entrega-2` | Mismo recorrido, mezcla, pausas y tasas. La aleatoriedad pasa de `Math.random` a la Web Crypto API, con la misma distribución uniforme |
| Imágenes de la aplicación | `ef646a192a82` | **`d4738e9e7919`**, con imagen horneada `mooc-base-d4738e9e7919`. Las mismas en los tres escenarios |

Tres diferencias pueden mover los resultados y se discuten con ellos.

1. **Pool de la API.** Con una sola instancia, la API tiene 8 conexiones frente a
   25. En la Entrega 2 el pool fue el segundo límite: con 10, el p95 de L3 era
   8,1 s, y con 25, 5,6 s. Con una instancia se espera más espera por el pool
   que en la línea base.
2. **Balanceador.** Añade un salto de red y termina TLS fuera de la instancia.
   Puede sumar latencia en cada petición y quitar a la instancia el trabajo de
   TLS. Las dos cosas se miden, no se suponen.
3. **Redis en otra máquina.** Cada comando cruza la red, frente a la red de
   Docker de la Entrega 2. La ida y vuelta medida entonces fue de 0,68 ms.

El generador no puede ser el límite. Se registra su CPU en cada corrida y la
corrida se descarta si pasa del 60 % o si pierde conexiones de su lado.

## 3. Herramienta

k6 2.3.0 en contenedor, desde el generador en la nube. Las peticiones a la API
entran por el balanceador. Las transferencias de archivos van directas a Cloud
Storage con URL firmadas, igual que en la Entrega 2. Los escenarios 1 y 2 usan
tasa de llegada fija (`constant-arrival-rate`), así que la carga ofrecida no baja
cuando el servidor se pone lento.

## 4. Datos sintéticos

Los mismos de la Entrega 2, creados de nuevo en la base de la Entrega 3.

| Dato | Cantidad | Cómo se crea |
| --- | --- | --- |
| Estudiantes y profesores | 600 y 10 | `make semilla-nube CARGA=1` |
| Cursos publicados | 20, con 60 lecturas y 3 quizzes de 10 preguntas cada uno | `k6/preparar.js` |
| Inscripciones e intentos previos | 510 y 200 | `k6/preparar.js` |
| Sesiones | 600, abiertas antes de medir | `k6/preparar.js` |
| Videos | Perfiles A (20 s, 360p), B (1 min, 720p) y C (2 min, 1080p) | `medios/generar.sh` y `k6/preparar-medios.js` |

## 5. Escenario 1. Actividad académica

**Recorrido.** El de la Entrega 2, sin cambios: catálogo, ficha, inscripción en
el 15 % de las iteraciones, contenido, cinco evidencias de progreso, progreso
calculado y un quiz en el 30 %. Unas 11 peticiones por iteración, 70 % lecturas.

| Nivel | Iteraciones/s | Peticiones/s aprox. | Calentamiento | Medición | Repeticiones |
| --- | --- | --- | --- | --- | --- |
| L1 | 4 | 30 | 2 min | 8 min | 2 |
| L2 | 8 | 61 | 2 min | 8 min, y una de 15 min | 2 |
| L2b | 12 | 91 | 2 min | 8 min | 2 |
| L3 | 16 | 110 | 2 min | 8 min | 2 |

Cada nivel empieza con el grupo de API en una instancia y espera a que vuelva a
una antes del siguiente. El grupo escala durante la corrida y se registra
cuántas instancias atendieron en cada minuto. Durante L2 corre el recorrido de
integridad de la Entrega 2: envío simultáneo, reenvío con la misma
`Idempotency-Key`, intento ajeno, clave del quiz en el snapshot y porcentaje
enviado por el cliente.

**Criterios.** Los de la Entrega 2. Éxito con el p95 de cada operación dentro de
su objetivo, `5xx` por debajo del 0,5 % y *timeouts* por debajo del 0,1 %.
Saturación con el p95 por encima de 3 veces el objetivo, `5xx` por encima del
1 % o un rendimiento que deja de crecer con la tasa. No se sube de nivel tras
uno saturado.

**Pregunta que responde.** Si añadir instancias de API mueve el límite, o si el
límite se queda en Cloud SQL `db-g1-small`, como en la Entrega 2.

## 6. Escenario 1. Resultados

Sesión del 28 de octubre de 2026, imágenes `d4738e9e7919`. Nueve corridas, más
una descartada. Evidencia y línea de tiempo en
[`docs/entrega3/evidencias/elastica/2026-10-28-sesion-d.md`](../docs/entrega3/evidencias/elastica/2026-10-28-sesion-d.md).

| Nivel | Req/s | p50 / p95 / p99 (ms) | Errores | Comprobaciones | Perdidas | Inst. API | CPU API / SQL | Conex. BD |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| L1-1 | 30,4 | 18 / 37 / 240 | 0 | 100 % | 0 | 1 → 2 | 9 % / 17 % | 22 |
| L1-2 | 30,5 | 18 / 28 / 150 | 0 | 99,99 % | 0 | 1 → 2 | 9 % / 17 % | 14 |
| L2-1 | 60,6 | 18 / 31 / 161 | 0 | 100 % | 0 | 1 → 3 | 12 % / 26 % | 23 |
| L2-2 | 60,7 | 18 / 27 / 188 | 0 | 100 % | 0 | 1 → 3 | 11 % / 26 % | 21 |
| L2, 15 min | 68,4 | 18 / 26 / 124 | 0 | 100 % | 0 | 1 → 3 | 12 % / 27 % | 23 |
| L2b-1 | 91,4 | 18 / 28 / 63 | 0 | 100 % | 0 | 1 → 3 | 17 % / 35 % | 27 |
| L2b-2 | 91,1 | 18 / 28 / 71 | 0 | 99,99 % | 0 | 1 → 3 | 17 % / 37 % | 30 |
| **L3-1** | **117,7** | **29 / 5.502 / 7.734** | 0 | 100 % | **161** | 1 → 3 | 19 % / 42 % | 31 |
| **L3-2** | **116,9** | **69 / 5.498 / 9.052** | 0 | 100 % | **183** | 1 → 3 | 21 % / 45 % | 30 |

**Cómo leer la columna «Req/s».** Son las peticiones de la fase medida divididas
entre el tiempo total de la corrida, **calentamiento incluido**. Con 8 minutos
medidos y 2 de calentamiento da 60,7; con 15 medidos y los mismos 2 da 68,4,
aunque las dos corrieron a la misma tasa de llegada (8 iteraciones/s, unas 80
peticiones/s reales durante la medición). Es la misma convención del informe de
la Entrega 2, y como allí todos los niveles duran lo mismo, las comparaciones
nivel a nivel siguen siendo válidas. La única fila que no se puede comparar por
esa columna es la de 15 minutos.

### Hasta L2b la plataforma no se inmuta

De 30 a 91 peticiones/s el p95 se queda entre **26 y 31 ms**, sin un solo `5xx`,
sin *timeouts* y sin iteraciones perdidas. El p99 incluso **mejora** al subir la
carga —240 ms en L1-1, 63 ms en L2b-1—: en los niveles bajos el grupo está en
una instancia y los percentiles altos los marca el arranque en frío de la
segunda; con el grupo ya en tres, todo va caliente.

Ese es el efecto del autoscaling, y se ve mejor en la comparación: **en la
Entrega 2 el mismo nivel L2b daba un p95 de 272 ms. Ahora da 28.** Casi diez
veces mejor, con la misma base de datos y las mismas imágenes.

### L3 satura, y el cuello de botella ya no es la API

A 117 peticiones/s el p95 se va a **5,5 segundos** y el generador pierde entre
161 y 183 iteraciones. Pero **no hay un solo error `5xx` ni un *timeout***, y las
comprobaciones funcionales siguen al 100 %: es saturación por latencia, no por
fallo. La plataforma se vuelve lenta, no incorrecta.

Lo interesante es **qué no es el límite**:

| Candidato | Medido en L3 | ¿Es el límite? |
| --- | --- | --- |
| CPU de la API | 19–21 % de media en tres instancias; ninguna pasó del 47 % | **No.** Sobra capacidad |
| Conexiones a la base | Máximo 31 de las 47 útiles | **No** |
| Redis | 659 órdenes/s de pico en `mooc-cola` | **No.** Es carga trivial para Redis |
| Cloud SQL | CPU media 42 %, pico 55 % | **No por CPU** |

Y qué sí apunta al culpable: el reparto por operación. Las escrituras se
degradan entre tres y seis veces más que las lecturas.

| Operación | p95 en L2b-1 | p95 en L3-1 | Factor |
| --- | --- | --- | --- |
| `GET /catalog/courses` | 14 ms | 1.269 ms | ×91 |
| `GET /content` | 28 ms | 4.599 ms | ×164 |
| `POST /progress` | 30 ms | 5.962 ms | ×199 |
| `POST /attempts` | 26 ms | 8.201 ms | ×315 |
| `POST /submit` | 28 ms | 8.532 ms | ×305 |
| `PATCH /answers` | 38 ms | 15.713 ms | ×413 |

Todas las operaciones se degradan a la vez, lo que señala un recurso compartido
aguas abajo, y las que escriben se llevan la peor parte. Con la CPU de la API
sobrada, las conexiones lejos del tope y Redis ocioso, **el recurso compartido
que queda es Cloud SQL `db-g1-small`**, y dentro de él la vía de escritura.

### Qué recurso de la base se agota: las escrituras del disco

La primera versión de esta sección decía que no se podía saber. Se añadieron las
métricas de disco de Cloud SQL a `exportar.py` y se reexportaron las ventanas ya
medidas, sin repetir ninguna corrida.

| Nivel | Peticiones/s | Escrituras de disco/s (media) | Pico | Transacciones/s | CPU de la base |
| --- | --- | --- | --- | --- | --- |
| L2 | 61 | 144 | 182 | 312 | 31 % |
| L2b | 91 | 218 | 260 | 476 | 43 % |
| **L3** | **118** | **206** | **339** | 587 | 55 % |

De L2 a L2b las escrituras **escalan con la carga**: +49 % de peticiones, +51 %
de escrituras. De L2b a L3 la carga sube un 30 % más y las escrituras **dejan de
subir**; la media incluso baja. Eso es la firma de un recurso saturado: el
rendimiento se aplana y lo que crece es la espera.

**El techo es el disco.** La instancia tiene un **PD-SSD de 10 GB**, y el disco
SSD de Google da 30 operaciones de escritura por segundo y gigabyte: **300**. El
pico medido en L3 es 339, por encima del régimen y dentro de lo que explica la
ráfaga.

Lo que descarta a los demás candidatos:

- **Lecturas de disco: 0,6 por segundo.** El conjunto de trabajo cabe en
  memoria; la base no lee del disco.
- **Memoria al 51 %.** No hay presión.
- **CPU al 55 %** sobre un núcleo compartido. No se puede descartar del todo
  que contribuya, pero no crece en proporción a la degradación.
- Las transacciones por segundo **siguen creciendo** (476 → 587) mientras las
  escrituras se aplanan, que es lo que ocurre cuando el límite es el `fsync`:
  al subir la concurrencia cada confirmación de grupo agrupa más transacciones
  por escritura, y cada una espera más.

**La propuesta de evolución sale sola, y es barata:** agrandar el disco. En
Cloud SQL las operaciones por segundo escalan linealmente con el tamaño, así que
pasar de 10 GB a 100 GB multiplica por diez el techo de escritura —de 300 a
3.000— por unos pocos dólares al mes, sin tocar el `tier` ni el código. Subir de
`db-g1-small` a un núcleo dedicado cuesta bastante más y no ataca lo que
medimos.

Conviene decir que el disco está **al 4 % de ocupación**: se agrandaría por
rendimiento, no por espacio. Es exactamente el tipo de ajuste que no se descubre
sin medir.

### Las comprobaciones que fallaron no son fallos

Cuatro comprobaciones de progreso sobre unas 40.800, un 0,01 %. Son respuestas
`2xx` con `accepted: false`: **el servidor rechazando la evidencia**, no un
error. La serie del servidor lo confirma para la ventana de 15 minutos, donde
el contador `progress_rejected_total{reason="cadence"}` sube exactamente **+2**,
que son las dos comprobaciones fallidas de esa ventana: dos latidos del mismo
recurso llegaron más juntos que el intervalo mínimo que admite la regla.

En la misma ventana, `reason="client_computed_value"` sube **+79**: es el
recorrido de integridad mandando porcentajes calculados por el cliente a
propósito, y el servidor rechazándolos todos (invariante 4, ADR-0012).

Las dos cosas son la regla funcionando. El informe las cuenta aparte de los
errores, como manda su propia definición de éxito.

## 7. Escenario 2. Carga y procesamiento multimedia

**Recorrido.** El de la Entrega 2. Un profesor autoriza la carga, sube las partes
directo a Cloud Storage, confirma y sondea hasta un estado terminal. Los
espectadores abren una sesión de reproducción y descargan segmentos a la
cadencia del video.

| Nivel | Cargas/min | Mezcla A/B/C | Espectadores | Duración | Repeticiones |
| --- | --- | --- | --- | --- | --- |
| M1 | 3 | 2/1/0 | 20 | 10 min | 2 |
| M2 | 6 | 3/2/1 | 45 | 10 min | 2 |
| M3 | 12 | 6/4/2 | 90 | 10 min | 1 |

Entre niveles se espera a que la cola `bulk` quede vacía y el grupo de workers
vuelva a una instancia. Cada instancia conserva la concurrencia de la Entrega 2.
Se registran las instancias activas y la concurrencia total de FFmpeg en cada
minuto, porque esa es la diferencia arquitectónica que se evalúa.

**Criterios.** Los de la Entrega 2. Éxito si la cola drena, no hay trabajos
muertos y cada video está listo en menos de 3 veces su duración. Saturación si
la cola crece de forma monótona durante 5 minutos.

**Pregunta que responde.** Cuánto acortan tres workers la espera en cola y el
tiempo hasta `ready`. La autorización, la transferencia y la reproducción no
dependen de los workers, y su cambio no se atribuye al autoscaling sin
evidencia.

## 8. Escenario 2. Resultados

Sesión del 28 de octubre. Cinco niveles válidos; dos de ellos se repitieron
tras descartarse por sesiones caducadas (ver «Dos niveles descartados»).

| Nivel | Cargas/min | p95 autorizar / confirmar (ms) | Transferencia (MiB/s) | Completa→`ready` p50 A / B / C (s) | p95 playlist / segmento (ms) | Segmentos fallidos |
| --- | --- | --- | --- | --- | --- | --- |
| M1-1 | 3 | 417 / 585 | 14,2 | 20 / 53 / — | 1.029 / 144 | 0 |
| M1-2 | 3 | 206 / 669 | 13,2 | 26 / 55 / — | 988 / 154 | 0 |
| M2-1 | 6 | 302 / 784 | 14,7 | 173 / 202 / 436 | 892 / 184 | 0 |
| M2-2 | 6 | 304 / 654 | 16,9 | 226 / 263 / 534 | 759 / 162 | 0 |
| M3-1 | 12 | 218 / 615 | 16,0 | **510 / 562 / 847** | 795 / 137 | 0 |

| Nivel | Cola `bulk` máx. | Antigüedad máx. (s) | Instancias worker | CPU worker media / máx. | Memoria worker máx. | Disco worker máx. |
| --- | --- | --- | --- | --- | --- | --- |
| M1-1 | 2 | 20 | 1 | 71 % / 85 % | 79 % | 15 % |
| M1-2 | 2 | 27 | 2 | 36 % / 86 % | 84 % | 15 % |
| M2-1 | 64 | 287 | 3 | 38 % / 99 % | 88 % | 15 % |
| M2-2 | 42 | 147 | 3 | 70 % / 93 % | 88 % | 15 % |
| M3-1 | **141** | **865** | 3 | 66 % / 98 % | 89 % | 15 % |

### El escalado de los workers funciona, y se ve en el tiempo hasta `ready`

A 3 cargas/min una sola instancia basta: la cola no pasa de 2 y un archivo
corto está listo en 20 segundos. A 6 el grupo sube a **tres** y la cola llega a
64 pendientes. A 12 la cola llega a **141** con 865 segundos de antigüedad, y el
archivo largo tarda 847 segundos en estar disponible.

La comparación con la Entrega 2 es la que importa: allí M2 dejó la cola en
**779 pendientes** y los tiempos hasta `ready` en 9–11 minutos. Aquí M2 deja la
cola en 42–64 y los tiempos en 3–9 minutos, con el mismo tamaño de instancia.
**Tres workers en lugar de uno es exactamente lo que explica la diferencia.**

### Lo que no cambia, y era de esperar

La autorización de carga (218–417 ms), la transferencia directa al
almacenamiento (13–17 MiB/s) y la reproducción (0 segmentos fallidos de todos
los niveles) **no dependen de los workers**, y en efecto no mejoran con el
escalado. Es la comprobación de que la mejora del tiempo hasta `ready` se
atribuye al autoscaling y no al ruido.

### La dispersión entre repeticiones es real

M2-1 y M2-2 corrieron con los mismos parámetros y dieron 173/202/436 s frente a
226/263/534 s, y colas de 64 frente a 42. Una diferencia del orden del 30 %
según cuánto trabajo quedara en vuelo al empezar. Es la razón de repetir los
niveles, y el informe reporta las dos medidas en vez de la mejor.

### Dos niveles descartados: la sesión preparada caduca a las 12 horas

M2-2 y M3-1 se corrieron primero a las 14:05 y 14:17 y **no subieron ni una
parte**: 27 y 26 respuestas `401`, «la sesión no existe, expiró o fue
revocada». Las sesiones preparadas se habían creado a la 01:25, y `SESSION_TTL`
son 12 horas.

Lo grave no es la caducidad, que está documentada, sino que **la sesión no se
renueva con el uso aunque el código diga que sí**. Está registrado como hallazgo
de deuda técnica. Para la metodología de este informe la consecuencia es
directa: **los datos preparados tienen fecha de caducidad y hay que renovarlos
si la campaña pasa de doce horas**.

Las dos corridas quedan archivadas como `-descartada-sesion-caducada` y se
repitieron con sesiones nuevas, sin un solo `401`.

## 9. Escenario 3. Definición del ciclo de autoscaling

### 3A. Escalado de la API

Recorrido del escenario 1, con escrituras, para que el ciclo también ponga a
prueba la integridad. Tres fases seguidas, desde el generador:

| Fase | Tasa | Duración | Qué se espera |
| --- | --- | --- | --- |
| Baja | L1, 4 iteraciones/s | 5 min | Una instancia |
| Alta | L3, 16 iteraciones/s, con el recorrido de integridad | 15 min | Crece hasta 3 instancias |
| Baja | L1 | 30 min | Reduce una instancia cada 10 min hasta 1 |

### 3B. Escalado de los workers

Cinco videos del perfil C a la vez (`k6/escalado-medios.js`), que piden tres
instancias. Después, 30 minutos sin carga para observar la reducción. Durante
la reducción se provoca el retiro de un worker con una transcodificación en
curso, subiendo dos videos más justo antes de la ventana de reducción.

### Qué se registra

| Grupo | Línea de tiempo |
| --- | --- |
| API | Peticiones/s, señal de utilización, decisión del autoscaler, instancias creadas, sanas en el balanceador y retiradas, latencias y errores |
| Workers | Trabajos de `bulk` pendientes y en curso, decisión del autoscaler, instancias creadas, primer trabajo tomado por cada una, retiros, trabajos completados, reintentos y tiempo hasta `ready` |

Se separan dos tiempos: de la subida de la demanda a la decisión del
autoscaler, y de la decisión a que la nueva instancia atiende o procesa. Al
terminar se reconcilian los trabajos aceptados con su estado final en
`platform.job_runs` y se comprueba que cada video tiene un solo juego de
derivados.

## 10. Métricas

| Métrica | Fuente | Ventana |
| --- | --- | --- |
| Latencia p50, p95 y p99, rendimiento, errores y *timeouts* | Resumen de k6 | La fase de medición, sin calentamiento |
| CPU, memoria, red y disco de cada instancia | Agente de operaciones en Cloud Monitoring | 60 s |
| Conexiones y CPU de Cloud SQL | Cloud Monitoring | 60 s |
| Profundidad, antigüedad y tasa de la cola | `/metrics` de los workers y `cola_bulk_trabajos` | 30 a 60 s |
| Instancias por grupo | Tamaño del grupo y salud en el balanceador | 60 s |
| Eventos de escalado | Registro de operaciones de Compute Engine | Por evento |

Los percentiles de corridas distintas no se promedian. Cada corrida se reporta
por separado y la variación se da como el rango entre repeticiones. La
duración de los trabajos asíncronos no se mezcla con la latencia HTTP.

## 11. Escenario 3. Resultados

Sesión del 28 de octubre. 3A de 16:01 a 17:00, 3B de 17:04 a 17:52. Evidencia
en [`docs/entrega3/evidencias/elastica/2026-10-28-sesion-d.md`](../docs/entrega3/evidencias/elastica/2026-10-28-sesion-d.md).

### 3A. La API

| Hora | Evento | Grupo |
| --- | --- | --- |
| 16:01:26 | Fase baja, 4 iteraciones/s | 1 |
| 16:03:02 | `insert mooc-api-hzhr` | → 2 |
| 16:09:07 | **Fase alta, 16 iteraciones/s** | 2 |
| **16:10:20** | `insert mooc-api-lgfd`, **73 s después** | → 3 |
| ~16:11 | La instancia nueva ya reparte | 3 |
| 16:27:01 | Fase baja | 3 |
| 16:39:19 | `delete mooc-api-hzhr`, **12 min después** | → 2 |
| 16:59:44 | Fin | 2 |

Los dos tiempos que el enunciado pide separar:

| Tramo | Medido |
| --- | --- |
| Subida de la demanda → decisión del autoscaler | **73 s** |
| Decisión → la instancia atiende peticiones | **~60 s** |
| Bajada de la demanda → primera retirada | **~12 min** |

Por minuto, con las peticiones de la aplicación y las conexiones a la base:

| UTC | Instancias | Peticiones/s | Conexiones |
| --- | --- | --- | --- |
| 16:02 | 1 | 38 | 22 |
| 16:04 | 1 | 81 | 22 |
| 16:05 | **2** | 80 | 25 |
| 16:10 | 2 | 204 | 26 |
| 16:11 | **3** | 309 | 26 |
| 16:13 | 3 | 317 | 36 |
| 16:26 | 3 | 266 | 31 |
| 16:28 | 3 | 101 | 25 |
| 16:40 | **2** | 85 | 15 |
| 16:59 | 2 | 39 | 16 |

### El grupo no bajó a una instancia, y la previsión estaba mal

La sección 9 decía que la fase baja final «reduce una instancia cada 10 min
**hasta 1**». No ocurrió: el grupo se quedó en **2** durante los 32 minutos de
carga baja.

**No es un fallo del autoscaler, es un error de la previsión.** La señal es la
utilización del balanceador con `maxRatePerInstance` 40 y objetivo 0,8, o sea
32 peticiones/s por instancia. La fase baja genera unas 40 peticiones/s, que
necesitan dos instancias. El autoscaler mantuvo exactamente lo que su política
dice que hay que mantener.

Se corrige la expectativa, no el resultado. Para ver la bajada completa hasta
una instancia haría falta una fase baja por debajo de 32 peticiones/s.

### 3B. Los workers

| Hora | Evento | Grupo |
| --- | --- | --- |
| 17:04:19 | Cinco videos del perfil C, a la vez | 1 |
| 17:05:29 | `insert mooc-worker-mrgs` | → 2 |
| 17:06:11 | `insert mooc-worker-z207`, **~110 s** tras las cargas | → 3 |
| 17:08 | Las tres transcodificando | 3 |
| 17:12 | Cola vacía | 3 |
| 17:21:02 | `delete mooc-worker-z207` | → 2 |
| 17:26 | Dos videos más del perfil C | 2 |
| 17:32:10 | `delete mooc-worker-mrgs` | → 1 |

El grupo llega al máximo de 3 en **menos de dos minutos** desde la carga, y cada
instancia nueva toma su primer trabajo en cuanto arranca. Los siete videos
terminaron en `ready`, todos en el intento 1, sin trabajos muertos.

### La prueba del retiro seguro no llegó a ejercitarse

Se diseñó a propósito: dos videos del perfil C subidos a las 17:26, para que el
autoscaler retirara una instancia **ocupada** y se pusiera a prueba el plazo de
280 s de `WORKER_SHUTDOWN_TIMEOUT`.

No salió. Las dos transcodificaciones terminaron a las **17:31:14** y
**17:31:19**, y la retirada fue a las **17:32:10**: cincuenta segundos tarde.

Y la razón es la propia defensa del sistema: **la señal del autoscaler cuenta
los trabajos en curso además de los pendientes**, así que mientras una instancia
transcodifica el grupo no la considera sobrante. El caso peligroso —retirar una
instancia ocupada— es estructuralmente raro por diseño, y por eso cuesta
provocarlo.

Lo que queda demostrado y lo que no:

| | |
| --- | --- |
| **Demostrado** | Los dos grupos crecen y decrecen solos, con sus tiempos medidos. Ningún trabajo se perdió, ninguno se reintentó, cada video tiene un solo juego de derivados |
| **Demostrado en la Sesión C** | El criterio de DT-02: se retiró un worker con una transcodificación activa y el trabajo terminó en el intento 1 |
| **Sin demostrar** | Que GCE respete los 280 s configurados. Ver abajo |

### Segundo intento: la instancia ocupada sí se retiró

El primer intento falló por el cronometraje —la segunda tanda se subió veinte
minutos después del escalado en vez de doce— y se repitió a las 18:45
contando desde el momento real en que el grupo llegó a tres.

```text
delete mooc-worker-1bs0            19:02:49
media.transcode_hls  18:59:34 → 19:04:17   283 s, intento 1, succeeded
media.transcode_hls  18:59:40 → 19:04:13   273 s, intento 1, succeeded
```

**La instancia se retiró con las dos transcodificaciones en vuelo y las dos
terminaron bien, en el primer intento y sin reintento.** El trabajo necesitó
**88 segundos** desde que empezó el `delete`, y los tuvo. Los siete videos
quedaron en `ready`.

Con esto el criterio de DT-02 queda cumplido por segunda vez, de forma
deliberada y no por casualidad.

**Lo que sigue sin verificarse, y conviene decirlo:** el plazo configurado es de
280 s y en las dos observaciones el trabajo necesitó unos 88 segundos. El margen
que GCE concedió fue de unos 107 s en la Sesión C. Es decir, **`WORKER_SHUTDOWN_TIMEOUT=280`
promete más de lo que la plataforma parece conceder**: una transcodificación que
necesitara más de unos 107 s tras la retirada se cortaría igual, pese a la
configuración. Medir el límite real exige un trabajo deliberadamente más largo
que ese margen; mientras tanto, lo prudente es bajar el valor configurado para
que no prometa lo que no puede cumplir y confiar en el reclamo por latido
vencido para los trabajos largos.

## 12. Comparación con la Entrega 2

Los valores de la Entrega 2 salen de su informe. Los de la Entrega 3 se
completan con la sesión de medición.

| Escenario y operación | Nivel de carga | Métrica y unidad | Entrega 2 | Entrega 3 | Cambio observado y explicación |
| --- | --- | --- | --- | --- | --- |
| E1, recorrido completo | L1, 30 peticiones/s | p95 (ms) | 23 | **28** | Prácticamente igual. A esta carga el grupo está en una instancia casi todo el nivel: no hay nada que escalar |
| E1, recorrido completo | L2, 61 peticiones/s | p95 (ms) | 25 | **27** | Igual dentro del ruido. La Entrega 2 ya servía este nivel con holgura |
| E1, recorrido completo | L2b, 91 peticiones/s | p95 (ms) | 272 | **28** | **Diez veces mejor.** Aquí es donde el autoscaling cambia el resultado: el grupo llega a tres instancias y el nivel deja de ser el codo de la curva |
| E1, recorrido completo | L3, 110 / 117 peticiones/s | p95 (ms) | 8.067 | **5.500** | Mejor, pero sigue saturado. Con la API sobrada de CPU, el límite se ha desplazado del todo a Cloud SQL |
| E1, recorrido completo | L3 | Iteraciones perdidas | 531 | **161 / 183** | Un tercio. El generador mantiene mejor la tasa porque la API responde, aunque la base no siga |
| E1, recorrido completo | Todos | Errores `5xx` y *timeouts* | 0 | **0** | Se mantiene. Ni siquiera saturada devuelve errores: se vuelve lenta, no incorrecta |
| E1, integridad | L2 | Comprobaciones fallidas | 0 de 244 | **0 de 940** | Se mantiene, con casi cuatro veces más aserciones y el grupo escalando durante la corrida. Las cinco comprobaciones —snapshot sin clave, intento ajeno, envío simultáneo, reenvío idempotente y porcentaje del cliente— pasan en las tres corridas |
| E2, autorizar carga | M2, 6 cargas/min | p95 (ms) | 308 | **302 / 304** | Igual. Autorizar y firmar no depende de los workers, y es la comprobación de que la mejora del procesamiento no es ruido |
| E2, procesamiento | M1, 3 cargas/min | Confirmación a `ready`, mediana A / B (s) | 10 / 35 | **20–26 / 53–55** | **Peor.** Con una sola instancia en los dos casos, la elástica tarda el doble: el worker comparte máquina con ClamAV y cruza la red para la cola. El escalado no ayuda donde no escala |
| E2, procesamiento | M2, 6 cargas/min | Confirmación a `ready`, mediana A / B / C (min) | 9,4 / 9,6 / 11,3 | **2,9–3,8 / 3,4–4,4 / 7,3–8,9** | **Dos a tres veces mejor.** Aquí sí escala: el grupo llega a tres instancias |
| E2, procesamiento | M2 | Vídeos a `ready` por minuto de carga | 1,3 | **3,1 / 4,8** | Hasta 8,0 en M3. Parte del salto es el escalado y parte el *reaper* corregido: en la Entrega 2 duplicaba trabajo (2.554 duplicados), así que su cifra está deprimida |
| E2, cola | M2 | Profundidad máxima de `bulk` | 779 | **42 / 64** | Un orden de magnitud menos, pero la cifra de la Entrega 2 está inflada por los duplicados del *reaper*. La comparación honesta es la del tiempo hasta `ready` |
| E2, reproducción | M2 | Segmentos fallidos | 0 de 4.689 | **0 de 4.726 y 0 de 4.741** | Se mantiene. La reproducción sale directa del almacenamiento y no pasa por la plataforma |

La profundidad de la cola de la Entrega 2 está inflada por los duplicados del
*reaper*, 856 en M2. En la Entrega 3 cada tarea lleva su clave como
identificador y la cola no admite copias, así que esa fila compara la espera
real con una medida inflada. Se discute junto al tiempo hasta `ready`, que no
tiene ese sesgo.

## 13. Limitaciones

**Del escenario 1, ya medido:**

- **El estrangulamiento del núcleo compartido no se descartó del todo.** La
  evidencia señala las escrituras de disco, pero `db-g1-small` tiene un núcleo
  compartido y su CPU al 55 % podría estar ya limitada. Separar las dos causas
  exigiría repetir L3 con el disco agrandado: si el p95 cae, era el disco.
- **La propuesta de agrandar el disco no se verificó.** Se deriva de la relación
  documentada entre tamaño y operaciones por segundo, no de una medición propia
  con el disco ya grande.
- **El desglose de `pg_stat_activity` por aplicación no se capturó** durante L3.
  La campaña corrió sin nadie delante y esa consulta es interactiva. El dato
  que sí hay, el máximo de conexiones de Cloud Monitoring, basta para descartar
  el agotamiento del pool: 31 de 47.
- **El grupo de workers estuvo en una instancia todo el escenario 1**, porque el
  recorrido no sube multimedia. La reserva de conexiones con **los dos** grupos
  en tres sigue sin comprobarse bajo carga; se cierra en el escenario 3.
- **La columna «Req/s» incluye el calentamiento en el denominador**, así que
  subestima la tasa real de la fase medida y no permite comparar corridas de
  distinta duración. Se mantiene por coherencia con la Entrega 2.
- **No se midió el arranque en frío por separado.** El p95 de 37 ms de L1-1
  frente a los 28 de L1-2 lo sugiere, pero con dos corridas no se distingue del
  ruido.

## 14. Reproducir estas pruebas

| Qué | Dónde |
| --- | --- |
| Guiones de k6 | [`k6/`](k6/): `preparar.js`, `escenario1.js`, `preparar-medios.js`, `escenario2.js`, `escalado-api.js`, `escalado-medios.js`, `comun.js` y `medios.js` |
| Videos | [`medios/generar.sh`](medios/generar.sh) |
| Resultados originales | [`resultados/`](resultados/), con el prefijo `e3-` |
| Tablas y gráficas | [`metricas/tabla.py`](metricas/tabla.py) y [`metricas/graficas.py`](metricas/graficas.py) |

```bash
make semilla-nube CARGA=1
make carga-sincronizar && make carga-medios
make carga K6=preparar.js ETIQUETA=e3-preparar
make carga-nivel K6=escenario1.js ETIQUETA=e3-L2 ARGS="-e TASA=8 -e INTEGRIDAD=1"
make carga K6=preparar-medios.js ETIQUETA=e3-preparar-medios
make carga-nivel K6=escenario2.js ETIQUETA=e3-M2 \
  ARGS="-e CARGAS=6 -e MEZCLA=3/2/1 -e ESPECTADORES=45 -e DURACION=10m -e ESPERA_MAX=20"
make carga-tabla ARGS="e1 e3-L1 e3-L2 e3-L2b e3-L3"
```
