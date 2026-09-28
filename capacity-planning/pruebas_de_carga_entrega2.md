# Pruebas de carga — Entrega 2

Informe de capacidad de la plataforma MOOC sobre el despliegue básico en la nube
pública. La ruta de este archivo la fija el enunciado de la entrega.

**Estado: en ejecución (27-09-2026).** La definición de abajo se fijó antes de
tocar el generador; los ajustes que exigió la plataforma real están en §1.1 y
§5.2.1, y los resultados en §9, con lo medido y no con lo esperado.

El objetivo no es alcanzar una escala de producción ni demostrar los objetivos
finales de concurrencia del producto. Es **determinar la capacidad sostenible de
la configuración básica, reconocer en qué condiciones empieza a degradarse y
explicar con mediciones qué componente limita primero el flujo.**

---

## 1. Infraestructura bajo prueba

Fija durante todas las corridas. Cualquier cambio obliga a reportar los
resultados como una configuración distinta.

| Elemento | Valor | Estado |
| --- | --- | --- |
| Proveedor y región | Google Cloud, `us-central1` | Fijado |
| Web Server | `e2-highcpu-2`: 2 vCPU no compartidas, 2048 MiB, 30 GiB `pd-balanced` | Fijado |
| Worker Server | `e2-highcpu-2`: 2 vCPU no compartidas, 2048 MiB, 30 GiB `pd-balanced` | Fijado |
| Base administrada | Cloud SQL PostgreSQL 17, `db-g1-small` (núcleo compartido, 1,7 GiB), 10 GiB SSD, zonal | Fijado |
| Almacenamiento de objetos | Cloud Storage, cuatro buckets en `US-CENTRAL1` | Fijado |
| Cola | Redis 7.4 en contenedor, Worker Server, AOF | Fijado |
| Generador de carga | `mooc-generador`, `e2-standard-4` (4 vCPU, 16 GiB), `us-central1-a`, sin IP externa, sale por Cloud NAT | Fijado |
| Versión de la aplicación | Imágenes `ef646a192a82` | Fijado |
| Concurrencia de workers | `worker` 20 (`critical=6,default=3`); `worker-media` 1 (`bulk=1`) | Fijado |
| Pool de conexiones | `DB_MAX_CONNS=10` por proceso en el escenario 1. La API pasó a 25 tras medirlo (§9.1.1), y el escenario 2 se mide así | Fijado por escenario |
| Límites por IP | `RATE_LIMIT_IP_FACTOR=100` (§3.1); por cuenta y sesión sin tocar | Fijado |
| Caché de la aplicación | Redis, base 2, sin cambios respecto a la Entrega 1 | Fijado |

### 1.1 Lo que cambió respecto a esta definición

- **El tipo se llama `e2-highcpu-2`.** Es la combinación exacta de 2 vCPU no
  compartidas y 2048 MiB. Pedido como `e2-custom-2-2048`, GCP lo normaliza a
  este nombre del catálogo.
- **La base es `db-g1-small`, de núcleo compartido.** El presupuesto real es de
  50 USD, y un vCPU dedicado costaba el doble. Se registra como limitación: un
  núcleo compartido puede ralentizarse sin aviso, y si la base resulta ser el
  límite, parte de la variación entre repeticiones puede venir de ahí.

### El tipo de máquina no es un detalle

El enunciado pide **2 vCPU y 2 GiB**. En el catálogo de GCP la combinación que
más se le parece de nombre, `e2-small`, tiene **vCPU compartidas**: dos hilos
con una fracción garantizada y capacidad de ráfaga. Medir capacidad sobre vCPU
compartidas produce números que no se repiten, porque dependen de lo que hagan
los vecinos y del crédito de ráfaga acumulado.

Se usa un **tipo personalizado de 2 vCPU dedicadas y 2048 MiB**
(`e2-custom-2-2048` o su equivalente vigente), que da la combinación exacta que
pide el enunciado sin compartir CPU. Se confirma con
`gcloud compute machine-types list` antes de la fase de aprovisionamiento y se
registra el nombre efectivo aquí.

---

## 2. Herramienta de generación de carga

**k6**, en contenedor, con la versión fijada y registrada en cada corrida.

| Criterio del enunciado | Cómo lo cumple k6 |
| --- | --- |
| Modelar los flujos del escenario | Guiones en JavaScript: un recorrido completo por iteración, con estado por usuario virtual |
| Automatizar validaciones | `check()` sobre el cuerpo de la respuesta, no solo sobre el código HTTP |
| Exportar resultados reproducibles | Resumen JSON y `--out csv` con todas las muestras |
| Patrón de inyección controlado | Ejecutores `constant-arrival-rate` y `ramping-arrival-rate` |
| No exigir herramienta específica | El enunciado deja la elección libre; se justifica y se registra |

Corre en contenedor como el resto del proyecto: ninguna herramienta se instala
en ninguna máquina de nadie.

### Modelo abierto, no cerrado

Los guiones usan **tasa de llegada** (`arrival-rate`), no número de usuarios
virtuales concurrentes.

La diferencia decide si la prueba sirve. Con un modelo cerrado —N usuarios en
bucle— cuando el servidor se pone lento cada usuario espera más, la tasa de
peticiones **baja sola** y el sistema nunca llega a acumular trabajo: la gráfica
enseña latencia creciente y rendimiento plano, y el punto de saturación no
aparece. Con un modelo abierto la carga ofrecida es independiente de lo que
tarde el servidor, que es como se comporta el tráfico real y lo que hace visible
la cola.

Consecuencia operativa: **`dropped_iterations` tiene que ser 0**. Si k6 no
consigue mantener la tasa pedida, la corrida mide al generador y se descarta.

### Verificar que el generador no limita

Una corrida es válida solo si, además de lo anterior:

- CPU del generador por debajo del 60 % durante toda la medición.
- Sin errores de red del lado del generador (`dial`, `connection reset`).
- La prueba de humo a tasa baja da la misma latencia que desde otra máquina.

Los tres se registran junto a los resultados de cada nivel.

---

## 3. Dos cosas que hay que ajustar antes de la primera corrida

Se descubren midiendo mal una vez. Aquí están antes.

### 3.1 El límite de tasa por IP convierte la prueba en una prueba del limitador

El generador es **una sola máquina, una sola IP**. La plataforma limita por IP:

| Regla | Límite | Sujeto |
| --- | --- | --- |
| `login_ip` | 60 / min | IP |
| `register` | 60 / hora | IP |
| `verify_email` | 20 / min | IP |
| `login_cuenta` | 5 / min | Cuenta |
| `progress` | 120 / min | Sesión |

Por encima de un inicio de sesión por segundo, **todo lo demás es `429`**. Sin
tocar nada, el escenario 1 mediría el limitador de tasa y no la plataforma.

**Qué se hace:** los límites por IP se suben, solo para la corrida, a un valor
por encima de la carga ofrecida máxima; los límites **por cuenta y por sesión se
dejan como están**, porque son los que protegen de verdad y cada usuario virtual
tiene cuenta propia. El valor efectivo se registra como condición fija de la
corrida, y si se ajusta a mitad de la campaña se presentan el antes y el después
por separado, como pide el enunciado.

Esto exige un cambio pequeño de código: hoy las reglas son constantes en
`identity/limites.go` y `learning/http.go`, no configuración. Está anotado en
[`alcance-entrega-2.md`](../arquitectura/alcance-entrega-2.md).

**Los `429` se reportan aparte** de los errores. Un rechazo por regla de negocio
o por límite de tasa no es un fallo del sistema, y mezclarlos infla la tasa de
error y esconde los fallos reales.

### 3.2 Las cuentas sintéticas no se crean por la API

`register` admite 60 por hora y por IP. Crear 500 estudiantes por la API son
más de ocho horas. El generador de datos sintéticos **escribe en la base
directamente**, con el mismo hash de contraseña que usa la aplicación, y deja
las cuentas ya verificadas.

Y hay una razón mejor que la velocidad: registrar por la API mete
`argon2id` de 64 MiB por cuenta en la preparación, lo que ensucia la máquina
antes de empezar a medir.

---

## 4. El hallazgo que condiciona el escenario 1

El hash de contraseñas es **argon2id con m=64 MiB, t=3, p=2** (ADR-0006). Cada
verificación de contraseña reserva **64 MiB** y ocupa dos hilos durante su
cómputo.

El Web Server tiene **2 vCPU y 2 GiB**. Diez inicios de sesión simultáneos piden
640 MiB solo de memoria de trabajo del hash, en una máquina que además sostiene
el proxy, la API y sus pools. **El inicio de sesión no escala en esta
configuración, y no es un defecto: es el parámetro haciendo su trabajo.**

Por eso el enunciado pide exactamente esta separación, y se respeta:

- **Las sesiones se preparan antes de la corrida.** El generador arranca con una
  bolsa de tokens válidos y el recorrido medido no incluye `login`.
- **La ráfaga de inicios de sesión se ejecuta y se reporta como variante
  separada**, con su propio nivel de carga, para no atribuir su costo a toda la
  actividad académica.

La variante de ráfaga es, además, la medición más interesante de la entrega:
da el número exacto a partir del cual el registro de entrada tumba la máquina, y
la propuesta de evolución que se deriva de él —bajar `m` y subir `t`, mover el
hash fuera del Web Server, o ambas— queda respaldada por una medición.

---

## 5. Escenario 1 — Actividad académica concurrente

Estudiantes que consultan el catálogo, entran a cursos, se inscriben, consumen
contenido, registran progreso válido y presentan quizzes.

### 5.1 Conjunto de datos sintéticos

Declarado y reproducible. Lo genera el sembrador y se restaura idéntico antes de
cada nivel de carga.

| Magnitud | Cantidad |
| --- | --- |
| Estudiantes | 600, verificados, con sesión preparada |
| Profesores | 10 |
| Cursos publicados | 20 |
| Módulos por curso | 3 |
| Unidades por módulo | 4 |
| Recursos por unidad | 5 (texto, PDF y video) |
| Quizzes | 1 por módulo, 10 preguntas de opción múltiple |
| Inscripciones previas | 400, repartidas sobre los 20 cursos |
| Intentos previos | 200 |

**Cada usuario virtual usa cuenta propia, curso propio e intento propio.** Dos
usuarios virtuales no compiten por la misma fila salvo cuando la prueba quiere
que compitan (§5.5): si compiten siempre, se mide contención inventada por el
guion, no por la plataforma.

### 5.2 El recorrido

Una iteración es una sesión de estudio plausible, no un endpoint repetido:

| # | Acción | Endpoint | Peso |
| --- | --- | --- | --- |
| 1 | Ver el catálogo | `GET /catalog/courses` (paginado por cursor) | 1 |
| 2 | Abrir la ficha de un curso | `GET /catalog/courses/{slug}` | 1 |
| 3 | Inscribirse, **solo la primera vez** | `POST /enrollments` | 0,15 |
| 4 | Abrir el contenido | `GET /enrollments/{id}/content` | 1 |
| 5 | Abrir un recurso | `GET /enrollments/{id}/resources/{rid}` | 2 |
| 6 | Latidos de progreso | `POST /enrollments/{id}/progress` | 3 |
| 7 | Consultar su progreso | `GET /enrollments/{id}/progress` | 1 |
| 8 | Presentar un quiz | `POST …/attempts` + `POST …:submit` | 0,3 |

≈ **10 peticiones HTTP por iteración**, con relación lectura/escritura cercana
al 70/30 en número de llamadas. Es más escritura que el 90/10 de referencia del
producto (`disenos/objetivos-de-servicio.md`) porque el latido de progreso es el
endpoint de mayor volumen real y hay que cargarlo de verdad.

**Pausas entre acciones:** 1 a 3 segundos, uniforme. Representan lectura. Una
iteración dura del orden de 20 a 25 segundos.

**La distribución de operaciones permanece constante entre niveles.** Lo único
que cambia de un nivel a otro es la tasa de llegada.

### 5.2.1 Ajustes del recorrido al medirlo

La plataforma real obligó a cuatro ajustes. Se declaran aquí porque cambian lo
que se mide.

- **«Abrir un recurso» es la evidencia `open` de progreso.** El detalle de
  recurso `GET /enrollments/{id}/resources/{rid}` está en el contrato como
  planificado y responde 404. El contenido ya llega en `GET /content`.
- **Los latidos respetan la cadencia de 10 s por recurso** (ADR-0012). El guion
  lleva la cuenta por recurso y espera lo que falte. Por eso una iteración dura
  unos 30 s y no 20 a 25.
- **Las inscripciones.** La preparación inscribe al 85 % de las cuentas. El 15 %
  restante se inscribe en cada iteración, y como la inscripción es un *upsert*,
  repetirla devuelve la misma. Es el peso 0,15 de la tabla, con escrituras
  reales.
- **Los recursos son de texto.** Veinte cursos con 60 lecturas y 3 quizzes de 10
  preguntas cada uno. El video se mide en el escenario 2.

La mezcla efectiva por iteración: catálogo 1, ficha 1, inscripción 0,15,
contenido 1, evidencias de progreso 5 (2 aperturas y 3 latidos), consulta de
progreso 1 y quiz 0,3 (inicio, respuestas y envío). Son unas 11 peticiones.

### 5.3 Niveles de carga

Ejecutor `constant-arrival-rate`, una tasa por nivel. La unidad es la
**iteración completa por segundo**, no la petición.

| Nivel | Iteraciones/s | ≈ Peticiones/s | ≈ Estudiantes concurrentes | Duración medida |
| --- | --- | --- | --- | --- |
| **L0** línea base | 1 | 10 | ~25 | 8 min |
| **L1** | 4 | 40 | ~100 | 8 min |
| **L2** | 8 | 80 | ~200 | 8 min |
| **L3** | 16 | 160 | ~400 | 8 min |
| **L4** *(si L3 aguanta)* | 24 | 240 | ~600 | 8 min |

- **Calentamiento:** 2 minutos a la tasa del nivel, descartados de la medición.
- **Pausa entre niveles:** 3 minutos, con la cola drenada y la base en reposo.
- **Repetición cerca del límite:** el último nivel estable se repite **15
  minutos** para comprobar que la estabilidad no era el arranque. Un sistema que
  aguanta ocho minutos y se cae al duodécimo no sostiene esa carga.

El nivel de referencia es L2: 80 peticiones/s se acerca a las ~100 req/s de
régimen que `objetivos-de-servicio.md` deriva de la escala del enunciado. Saber
si la configuración básica llega ahí es la pregunta que el informe contesta.

### 5.4 Variante separada: ráfaga de inicios de sesión

No entra en los niveles de arriba. Se ejecuta aparte y se reporta aparte (§4).

Ejecutor `ramping-arrival-rate` de 1 a 20 inicios de sesión por segundo en 3
minutos, midiendo latencia de `POST /sessions`, CPU y **memoria** del Web
Server. Criterio de parada: p95 por encima de 5 s o memoria disponible por
debajo de 200 MiB.

### 5.5 Comprobación de integridad bajo concurrencia

El enunciado la exige nominalmente: *«Incluir una comprobación de envío
duplicado sin doble calificación.»* Se ejecuta **durante** el nivel L2, no en
reposo, porque en reposo no comprueba nada.

| Comprobación | Cómo | Qué debe pasar |
| --- | --- | --- |
| Envío duplicado con la misma `Idempotency-Key` | Dos `:submit` idénticos seguidos | Una sola calificación; la segunda respuesta repite la primera |
| Envío duplicado **simultáneo** | Dos `:submit` en paralelo sobre el mismo intento | Una gana, la otra recibe conflicto o la misma respuesta. **Nunca dos notas** |
| Progreso enviado por el cliente | `POST /progress` con un porcentaje inventado | Rechazado y auditado (ADR-0012) |
| Clave del quiz | Leer el detalle del recurso y el snapshot del intento | La respuesta correcta **no aparece** (ADR-0013) |
| Intento ajeno | Un usuario virtual intenta enviar el intento de otro | `403` |

Verificación final: `SELECT count(*)` de calificaciones por intento es siempre
**1**. Un `200` no demuestra que el estado quedara bien; se comprueba el estado.

### 5.6 Qué debe contestar el informe

1. ¿Qué volumen de actividad sostiene la plataforma dentro de los umbrales y en
   qué nivel empieza a degradarse?
2. ¿Qué operaciones concentran la latencia o los errores, y cómo se relacionan
   con la API, Redis y su cola, el pool de conexiones y PostgreSQL?
3. ¿Se conservan la integridad de intentos, la calificación y el progreso bajo
   concurrencia?
4. ¿Qué cambio aumentaría la capacidad y qué medición respalda esa propuesta?

**Hipótesis previa, para contrastarla con lo medido:** el primer límite será el
salto de red a Redis, que está en el Worker Server (ADR-0016, D2) y se cruza
varias veces por petición autenticada —sesión, límite de tasa y, al escribir,
encolado—. La medición que la confirma o la descarta es la latencia de la
llamada a Redis frente a la latencia total del endpoint. Si se confirma, la
propuesta de evolución es un Redis local a la API para sesión y caché,
conservando la cola en el Worker Server.

---

## 6. Escenario 2 — Carga, procesamiento y consumo multimedia

Profesores que suben video y audio **directamente al almacenamiento de
objetos** mientras estudiantes consumen HLS ya disponible desde ese mismo
servicio.

### 6.1 Perfiles de archivo

Tres, con duración, tamaño y resolución distintos, como exige el enunciado. Las
rendiciones esperadas salen de la escalera de `media.VariantesPara`, que **no
sube de resolución**: un original de 360p produce una sola variante.

| Perfil | Duración | Resolución | Tamaño aprox. | Rendiciones esperadas | Segmentos de 6 s |
| --- | --- | --- | --- | --- | --- |
| **A** corto | 20 s | 640×360 | 1,4 MiB | 360p | 4 por variante |
| **B** medio | 1 min | 1280×720 | 8,3 MiB | 360p + 720p | 10 |
| **C** largo | 2 min | 1920×1080 | 36 MiB | 360p + 720p + 1080p | 20 |

**Ajuste del 27-09, antes de la primera corrida.** La definición original
usaba 30 s, 3 min y 10 min (hasta 180 MiB). Se acortó porque un solo archivo
de 10 min en 1080p tarda decenas de minutos en transcodificarse con 2 vCPU, y
el drenaje entre niveles no cabía en el tiempo de la entrega. Se conservan
los tres perfiles distintos en duración, tamaño y resolución, y la escalera
completa de rendiciones en C.

Los archivos los genera `capacity-planning/medios/generar.sh` con el mismo
FFmpeg de `worker-media`: imagen de prueba en movimiento (`testsrc2`) y un
tono de 440 Hz, en H.264 y AAC. No son grabaciones reales ni ruido
incompresible; se declaran así.

Son archivos reales, no ruido: un archivo sintético incompresible falsea el
tiempo de FFmpeg. **No se aumenta artificialmente la resolución** del original
para forzar más rendiciones.

### 6.2 Los dos flujos

**Carga (profesores).** Por iteración:

1. `POST /assets` — autorizar y obtener las URL firmadas por parte.
2. `PUT` de cada parte **directo a Cloud Storage**, en paralelo. Este tráfico
   no pasa por la API y se mide aparte.
3. `POST /assets/{id}:complete` — confirmación, responde `202`.
4. Sondeo de `GET /assets/{id}` hasta `ready` o estado terminal de fallo.

**Consumo (estudiantes).** Por espectador:

1. `POST /enrollments/{id}/media-sessions` — credencial de reproducción.
2. `GET` del master y del playlist de la variante.
3. `GET` de segmentos **a cadencia de reproducción: uno cada 6 segundos**.

La cadencia no es un detalle. Descargar todos los segmentos tan rápido como se
pueda es otro patrón de carga —una descarga masiva, no una reproducción— y si se
hace, se identifica como tal y se reporta por separado. Con 6 segundos por
segmento, 60 espectadores son 10 peticiones/s al almacenamiento, no 600.

### 6.3 Niveles de carga

**La concurrencia de los workers permanece fija** en todos los niveles:
`WORKER_CONCURRENCY=1` y `WORKER_QUEUES=bulk=1` en `worker-media`. Es lo que
convierte el escenario en una medición de capacidad de procesamiento y no en una
medición de cuántos FFmpeg caben.

| Nivel | Cargas/min | Mezcla A/B/C | Espectadores | Segmentos/s | Duración |
| --- | --- | --- | --- | --- | --- |
| **M0** línea base | 1 | 1/0/0 | 5 | 0,8 | 10 min |
| **M1** | 3 | 2/1/0 | 20 | 3,3 | 10 min |
| **M2** | 6 | 3/2/1 | 45 | 7,5 | 10 min |
| **M3** | 12 | 6/4/2 | 90 | 15 | 10 min |

**Ajuste del 27-09, antes de la primera corrida.** Niveles de 10 minutos, y
cada carga sondea su estado hasta 20 minutos después de confirmarse. Lo que
no termine en ese plazo se reporta como aceptado sin terminar, con su estado,
y se observa el drenaje antes del nivel siguiente.

Las corridas de multimedia son más largas porque el trabajo asíncrono tarda:
con M0 no tiene sentido medir diez minutos de cola si la cola se vacía en dos.

**Criterio de parada:** profundidad de la cola creciendo de forma monótona
durante 5 minutos sin señal de drenaje, o disco del Worker Server por encima del
85 %, o memoria disponible por debajo de 150 MiB, o trabajos muertos > 0.

### 6.4 Qué se mide, separado como pide el enunciado

**En la API:**
- Latencia de autorizar la carga y emitir las URL firmadas (objetivo: p95 ≤ 150 ms).
- Latencia de la confirmación de carga completa.
- CPU de la API mientras firma. Firmar es criptografía, y con firma sin clave
  descargada es además una llamada de red a IAM Credentials.

**En la transferencia directa:**
- Tiempo de transferencia al almacenamiento, tasa efectiva y errores, medidos
  **en el generador**. Este tráfico no toca la API y hay que distinguirlo del
  tráfico de control.

**En la cola y el procesamiento:**
- Tiempo de espera en cola.
- Duración del procesamiento, por perfil de archivo.
- **Tiempo desde la carga completa hasta `ready`.** Es la métrica que el usuario
  percibe.
- Trabajos completados por unidad de tiempo, reintentos y fallos.
- **Evolución de la profundidad y la antigüedad de la cola.** Hoy no se exportan;
  el trabajo está anotado en `alcance-entrega-2.md` y **bloquea este escenario**.

**En el almacenamiento:**
- Latencia, tasa de transferencia y errores de las operaciones sobre objetos.

**En las máquinas:**
- CPU, memoria, red y **disco** del Worker Server. El disco importa aquí más que
  en ningún otro sitio: el original y las tres calidades conviven en 30 GiB
  mientras dura el trabajo.

**En la reproducción:**
- Latencia y errores al pedir manifiestos y segmentos.
- Si se reporta tiempo hasta el primer cuadro o interrupciones de reproducción,
  **se miden con un reproductor**. Peticiones HTTP por sí solas no demuestran
  eso, y el enunciado lo dice expresamente. Si no se mide con reproductor, no se
  afirma.

### 6.5 Al terminar la generación de carga

Observar el drenaje de la cola y verificar que **todos los trabajos aceptados
terminan en `ready` o en un estado de fallo diagnosticable**. Un `202` de
aceptación no equivale a una transcodificación exitosa, y contar `202` como
éxito es la forma más común de reportar una capacidad que no existe.

Además, dentro de este escenario, la evidencia que el video necesita:

| Prueba | Qué demuestra |
| --- | --- |
| EICAR subido como video | Termina en `infected`, **sin derivados**. El orden escaneo → transcodificación se respeta también en la nube |
| Archivo ilegible | Termina en fallo diagnosticable, no colgado |
| Fallo con reintento | Un trabajo falla, reintenta con *backoff* y acaba bien |
| `:complete` duplicado | No produce dos transcodificaciones (invariante 6) |

### 6.6 Hipótesis previa

FFmpeg en 2 vCPU será el límite del procesamiento, y la cola crecerá antes de
que se degrade nada más. Pero hay dos candidatos que pueden llegar primero y por
eso se vigilan desde M0:

1. **La memoria del Worker Server.** `clamd` mantiene la base de firmas
   residente, del orden de 1 GiB, y al lado tienen que caber Redis, dos procesos
   Go y FFmpeg en 2 GiB. Si no cabe, se documenta el fallo, el ajuste mínimo
   aplicado y su efecto en costo y capacidad, que es lo que el enunciado manda
   hacer en ese caso.
2. **El disco de 30 GiB**, por la convivencia de original y derivados.

Si el límite es FFmpeg, la propuesta de evolución es cuantificable con lo
medido: `job_duration_seconds{type="media.transcode_hls"}` dividido por la
duración del original da el costo por minuto de transcodificación, y de ahí sale
cuántas máquinas o qué tamaño harían falta para una tasa de carga dada.

---

## 7. Métricas que se recogen en toda corrida

| Origen | Métricas |
| --- | --- |
| Generador (k6) | p50, p95, p99, rendimiento, errores, *timeouts*, `dropped_iterations`, comprobaciones funcionales fallidas |
| Web Server | CPU, memoria, red, disco |
| Worker Server | CPU, memoria, red, disco |
| Cloud SQL | CPU, memoria, conexiones activas, operaciones de E/S, consultas lentas |
| Redis / cola | Profundidad, antigüedad del trabajo más viejo, tasa de procesamiento, reintentos, trabajos muertos |
| Almacenamiento | Latencia, tasa de transferencia y errores por operación |
| Aplicación | `http_request_duration_seconds`, `jobs_processed_total`, `job_duration_seconds`, `jobs_dead_letter_total`, `rate_limited_total`, `progress_rejected_total` |

Memoria y disco **no están** en las métricas por defecto de Compute Engine. Se
recogen con un agente instalado en las dos máquinas desde el aprovisionamiento;
si se deja para después, el primer nivel de carga se ejecuta sin ellas y hay que
repetirlo.

## 8. Definición de éxito y de saturación

**Éxito de un nivel** — los cuatro a la vez:

| Criterio | Umbral |
| --- | --- |
| Latencia | p95 dentro de la clase de `objetivos-de-servicio.md`; p99 ≤ 2× el p95 de su clase |
| Errores | `5xx` < 0,5 %; *timeouts* < 0,1 % |
| Recursos | Ninguna máquina con CPU al 100 % sostenido, ni memoria disponible < 200 MiB |
| Cola | Profundidad estable o decreciente al terminar el nivel; trabajos muertos = 0 |
| Función | Todas las comprobaciones funcionales pasan |

**Saturación** — cualquiera de estos:

- p95 por encima de 3× el objetivo de su clase.
- `5xx` por encima del 1 %, o *timeouts* por encima del 0,5 %.
- Rendimiento que deja de crecer aunque suba la tasa de llegada.
- Cola creciendo de forma monótona sin drenar.
- Memoria agotada o proceso reiniciado por el sistema.

**Lo que no cuenta como fallo:** los `429` por límite de tasa y los rechazos por
regla de negocio —inscripción duplicada, intento agotado, progreso inválido—.
Se cuentan aparte y se reportan aparte. Una petición rechazada por una regla que
funciona no es una petición fallida.

**Lo que no cuenta como éxito:** un `200` sin validación del resultado. Cada
operación con efecto comprueba su estado: la inscripción existe, el progreso
subió, el intento tiene una nota y solo una.

---

## 9. Resultados

*Pendiente de ejecución.*

### 9.1 Escenario 1

Corridas del 27-09-2026, imágenes `ef646a192a82`, generador en
`mooc-generador`. Cada fila es la fase de medición (8 min; 15 en la
repetición), sin el calentamiento. Las cifras salen de
`make carga-tabla ARGS="e1 …"`, que lee los archivos de
[`resultados/`](resultados/): el resumen de k6 (`<nivel>.json`) y las métricas
de Cloud Monitoring de la misma ventana (`<nivel>-metricas.json`).

| Nivel | Iter/s | Req/s | p50 / p95 / p99 (ms) | Errores (5xx + timeouts) | CPU web / worker / SQL |
| --- | --- | --- | --- | --- | --- |
| L0 | 1 | 7,7 | 18 / 24 / 32 | 0,00 % | 8 / 6 / 10 % |
| L1 | 4 | 30,2 | 17 / 23 / 30 | 0,00 % | 19 / 7 / 17 % |
| L2 | 8 | 61,2 | 17 / 25 / 32 | 0,00 % | 35 / 9 / 27 % |
| L2, repetición 15 min | 8 | 68,3 | 17 / 24 / 31 | 0,00 % | 36 / 10 / 29 % |
| L2b | 12 | 91,1 | 19 / 272 / 959 | 0,00 % | 44 / 13 / 35 % |
| L3 | 16 | 110,4 | 2893 / 8067 / 14695 | 0,00 % | 44 / 11 / 40 % |

| Nivel | Memoria web / worker | Conexiones BD máx. | 429 | Comprobaciones | Iteraciones perdidas |
| --- | --- | --- | --- | --- | --- |
| L0 | 35 / 74 % | 9 | 0 | 100,00 % | 0 |
| L1 | 38 / 74 % | 12 | 0 | 100,00 % | 0 |
| L2 | 44 / 74 % | 14 | 0 | 100,00 % | 0 |
| L2, repetición | 53 / 75 % | 16 | 0 | 99,84 % | 0 |
| L2b | 55 / 75 % | 16 | 0 | 99,83 % | 0 |
| L3 | 55 / 74 % | 16 | 0 | 100,00 % | 531 |

La CPU del generador no pasó del 13 % en ningún nivel y no hubo errores de
red del lado del generador: las corridas son válidas (§2). La memoria del
Worker Server es casi constante porque la ocupa `clamd` (966 MiB en reposo).

![Latencia por nivel](graficas/e1-latencia-por-nivel.png)

![Recursos por nivel](graficas/e1-recursos-por-nivel.png)

**Las comprobaciones que fallan no son fallos del sistema.** Aparecen solo en
las dos corridas posteriores a L3 (113 y 96 rechazos `4xx` en la medición) y
todas son rechazos de negocio: un inicio de intento recibe `409` porque la
cuenta tiene otro intento abierto en ese quiz. Esos intentos quedaron abiertos
en L3, cuando la saturación cortó iteraciones a mitad del quiz (105 en la base
al terminar). La regla que los rechaza es el índice `attempts_one_in_progress`
y funciona como debe. El guion ya cuenta ese `409` como rechazo de negocio.

**Integridad bajo concurrencia (§5.5), durante L2:**

| Comprobación | Correctas | Fallidas |
| --- | --- | --- |
| Dos envíos simultáneos del mismo intento: una sola calificación | 49 | 0 |
| Reenvío con la misma `Idempotency-Key`: la misma respuesta | 49 | 0 |
| Enviar el intento de otro estudiante: rechazado | 48 | 0 |
| La clave del quiz no aparece en el snapshot del intento | 49 | 0 |
| Un porcentaje enviado por el cliente: rechazado | 49 | 0 |

### 9.1.1 El cuello de botella, y la medición que lo sostiene

**La capacidad sostenible es de unas 60 a 70 peticiones por segundo**: L2
aguanta 15 minutos con p95 de 24 ms. A 91 (L2b) empieza la degradación y a
110 (L3) el sistema está saturado. Nunca hay errores: todo lo que responde,
responde bien, pero tarde.

**Ninguna máquina llega al 100 % de CPU.** La serie de L3 lo explica: durante
cuatro minutos aguanta, y cuando la latencia se dispara la CPU del Web Server y
la de Cloud SQL **bajan** (de 60 a 43 % y de 54 a 41 %). Menos CPU con más
latencia es trabajo que espera.

![L3 minuto a minuto](graficas/e1-L3-serie.png)

Había dos candidatos y se midieron por separado, cambiando una cosa cada vez
(se presentan antes y después, como exige la entrega):

| L3, 16 iter/s | Req/s | p50 / p95 (ms) | Iteraciones perdidas | Conexiones BD |
| --- | --- | --- | --- | --- |
| `db-g1-small`, pool de la API en 10 (configuración base) | 110,4 | 2893 / 8067 | 531 | 16 |
| `db-g1-small`, pool de la API en 25 | 114,3 | 1654 / 5594 | 317 | 32 |
| `db-custom-1-3840` (1 vCPU dedicado), pool en 25 | 121,2 | 17 / 27 | 0 | 19 |

1. **El pool de conexiones de la API** (`DB_MAX_CONNS=10`). Se llenó en L3, y
   ampliarlo a 25 mejoró la latencia un 30 a 40 %. Contribuye, pero no es la
   causa.
2. **El núcleo compartido de `db-g1-small`.** La CPU consumida por la base
   (`cloudsql …/database/cpu/usage_time`) llega a 0,54 núcleos y, a los dos o
   tres minutos, cae a 0,41-0,44 aunque la carga ofrecida no cambie: es la
   cuota sostenida de un núcleo compartido cuando se agota la ráfaga. **Con un
   vCPU dedicado, la misma carga da p95 de 27 ms y ninguna iteración perdida**,
   el mismo comportamiento que L2.

**El primer límite es Cloud SQL `db-g1-small`.** El segundo es el pool de la
API. Tras las pruebas la base volvió a `db-g1-small` y el pool quedó en 25,
que es la configuración con la que se midió el escenario 2.

**La hipótesis previa (§5.6) se descarta.** El salto a Redis cuesta, pero no
satura: bajo L2, la ida y vuelta desde el Web Server mide 0,68 ms de media y
3 ms de máximo (`redis-cli --latency`, 2.774 muestras), con unos 5,2 comandos
por petición. En serie son unos 3,5 ms, cerca del 20 % de la mediana. El
Worker Server, donde vive Redis, no pasa del 13 % de CPU.

### 9.2 Ráfaga de inicios de sesión

| Momento (UTC) | Demanda ofrecida | Inicios atendidos | Memoria del Web Server |
| --- | --- | --- | --- |
| 23:14:30 | 1 por segundo | — | 50 % |
| 23:15:00 | ~4 por segundo | ~0,5 por segundo | 59 % |
| 23:15:30 | ~7 por segundo | ~1,5 por segundo | 59 %, último dato del agente |
| 23:16:33 | ~11 por segundo | ninguno: *timeouts* de 60 s | sin datos: máquina congelada |

**El Web Server se congela a partir de unos dos inicios de sesión por
segundo.** Cada uno es un argon2id de 64 MiB y dos hilos. La máquina despacha
unos 1,5 por segundo; lo que llega por encima se acumula, cada uno con su
memoria reservada, y en 2 GiB sin swap el núcleo se queda sin memoria antes
de que el recolector de procesos actúe. El agente de métricas dejó de informar
a la vez que la API. Hizo falta reiniciar la máquina (`gcloud compute
instances reset`, 23:23 UTC); volvió en un minuto y sin pérdida de datos.

La corrida la paró el propio criterio de parada (p95 > 5 s), pero tarde: el
criterio de memoria (< 200 MiB) no lo vigila k6, y cuando la latencia lo
delató la máquina ya no respondía. Queda como limitación del experimento.

**Qué lo resolvería, con esta medición detrás:** limitar cuántos hashes corren
a la vez en la API (un semáforo del tamaño de los núcleos), para que una
ráfaga espere en cola en vez de reservar memoria sin límite. Con 2 vCPU, dos
hashes simultáneos ocupan 128 MiB fijos y el resto espera. El costo de cada
login no cambia; lo que cambia es que la máquina no se cae.

### 9.3 Escenario 2

| Nivel | Cargas/min | Espectadores | p95 autorizar | Transferencia | Espera en cola | Duración A/B/C | Completa→`ready` | Prof. cola | Disco | Veredicto |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| M0 | 1 | 5 | | | | | | | | |
| M1 | 3 | 20 | | | | | | | | |
| M2 | 6 | 45 | | | | | | | | |
| M3 | 12 | 90 | | | | | | | | |

### 9.4 Punto de degradación y cuello de botella

*Pendiente. Con la evidencia que lo sustenta y las limitaciones del experimento.*

### 9.5 Propuesta de evolución

*Pendiente. Cada propuesta con la medición que permite esperar que ese cambio
aumente la capacidad.*

---

## 10. Reproducir estas pruebas

*Pendiente: guiones, datos, órdenes y resultados originales.*

Las gráficas y las capturas sirven como evidencia, no sustituyen la
interpretación. El informe relaciona las métricas del generador con las del Web
Server, el Worker Server, PostgreSQL, Redis y el almacenamiento, para explicar
**qué ocurrió y por qué**.
