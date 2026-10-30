# Guion del video. Entrega 3

Aplicación elástica IaaS. **Máximo 20 minutos** (el PDF de la Entrega 3 no fija
duración; se mantiene el límite del proyecto).

## Qué pide el enunciado y dónde se cubre

El enunciado (§Sustentación → Video) pide cinco cosas y nueve demostraciones
concretas. Esta tabla es la lista de verificación: si un bloque se cae por
tiempo, aquí se ve qué requisito se queda sin cubrir.

| Lo que se pide | Bloques |
| --- | --- |
| La arquitectura desplegada | 1 |
| Recorridos mínimos de estudiante y profesor desde el frontend | 2 |
| Ciclos de escalamiento de API y workers, con sus evidencias | 3, 4, 5 |
| Resultados comparativos de desempeño | 6 |
| Diagnóstico de calidad y deuda técnica, con línea base y plan | 7 |

| Demostración exigida | Bloque | Cómo |
| --- | --- | --- |
| Recorrido conectado profesor–estudiante | 2 | En vivo, por el frontend |
| Tráfico distribuido entre réplicas de API | 3 | En vivo, cabecera `X-Instancia` |
| Incorporación automática de capacidad, las dos capas | 3, 5 | **En vivo**: la API en ~84 s, los workers en 3–5 min |
| Retiro automático de capacidad, las dos capas | 4, 5 | Grabado: tarda 10–12 min |
| Procesamiento por nuevos workers | 5 | Grabado, con el diario de las instancias nuevas |
| Recuperación segura de trabajos | 5 | Grabado: el retiro con trabajo en vuelo |
| Operaciones sobre PostgreSQL administrado | 3 | En vivo, `make psql-nube` |
| Acceso al almacenamiento de objetos | 2, 3 | En vivo: el video reproduce y los derivados están en el bucket |
| **Continuidad funcional durante los cambios de capacidad** | 2 + 3 | El recorrido del bloque 2 ocurre **mientras** el grupo crece |

> El enunciado permite enseñar los procesos largos con ejecuciones previas
> «siempre que sus registros completos estén disponibles». Los nuestros están en
> `docs/entrega3/evidencias/elastica/`. Lo que sí se hace en vivo es el
> **crecimiento** de los dos grupos, que tarda menos de dos minutos.

---

## La idea del guion

**El escalado no se cuenta, se provoca al principio y se recoge después.**

En el bloque 1, mientras se explica la arquitectura, se lanza la carga. Para
cuando el bloque 3 llega, el grupo ya está en tres instancias y lo hemos visto
crecer sin esperar a nada. Y el recorrido funcional del bloque 2 habrá ocurrido
justo durante ese crecimiento, que es la prueba de continuidad que el enunciado
pide y la más difícil de fingir.

---

## Tiempos y reparto

| # | Bloque | Min | Quién |
| --- | --- | --- | --- |
| 0 | Apertura | 0,5 | |
| 1 | La arquitectura elástica y qué cambió | 2,5 | |
| 2 | Recorrido conectado profesor → estudiante | 4,5 | |
| 3 | Réplicas, continuidad, base y objetos | 3 | |
| 4 | Ciclo de escalado de la API | 2 | |
| 5 | Workers: escalado y recuperación segura | 3 | |
| 6 | Resultados comparativos | 2,5 | |
| 7 | Calidad, deuda técnica y cierre | 1,5 | |
| | **Total** | **19,5** | |

---

## Qué tener abierto antes de grabar

### Ventanas

1. Navegador en `https://34-160-11-22.sslip.io`, **sesión cerrada**.
2. Terminal 1, en la raíz del repositorio, para los `make`.
3. Terminal 2, para la carga que provoca el escalado.
4. Consola de GCP en Compute Engine → Grupos de instancias (opcional, se ve
   mejor con `make instancias`).
5. Editor con los documentos de abajo, cada uno en su sección.

### Documentos

| # | Archivo | Sección | Bloque |
| --- | --- | --- | --- |
| 1 | `docs/entrega3/vista-de-despliegue.md` | El diagrama | 1 |
| 2 | `arquitectura/adr/0019-arquitectura-elastica-iaas.md` | Decisiones D2 y D9 | 1 |
| 3 | `capacity-planning/pruebas_de_carga_entrega3.md` | 11, línea de tiempo de 3A | 4 |
| 4 | `capacity-planning/pruebas_de_carga_entrega3.md` | 11, el retiro con trabajo en vuelo | 5 |
| 5 | `capacity-planning/pruebas_de_carga_entrega3.md` | 12, tabla comparativa | 6 |
| 6 | `capacity-planning/pruebas_de_carga_entrega3.md` | 6, «qué recurso de la base se agota» | 6 |
| 7 | `docs/entrega3/calidad/resumen.md` | Medidas de la línea base | 7 |
| 8 | `docs/entrega3/deuda-tecnica.md` | DT-22 y el plan de tratamiento | 7 |
| 9 | `docs/entrega3/arquitectura.md` | 10, Costo y consumo observado | 7 |

### Evidencias de las sesiones

| # | Archivo | Qué enseña | Bloque |
| --- | --- | --- | --- |
| 10 | `evidencias/elastica/2026-10-28-sesion-d.md` | Línea de tiempo de 3A y 3B | 4, 5 |
| 11 | `evidencias/elastica/2026-10-24-sesion-c.md` | Exclusión del balanceador y autorreparación | 4 |
| 12 | `evidencias/frontend/escritorio.png` y `movil.png` | La interfaz verificada | 2 |

---

## Bloque 0 — Apertura (0,5 min)

Quiénes somos, qué entregamos: la plataforma MOOC sobre una arquitectura
elástica en GCP, con dos grupos de instancias que escalan solos.

Decir de entrada: **«todo lo que vais a ver corre en la nube, y el escalado que
enseñamos sucede durante el video, no está pregrabado salvo donde lo digamos»**.

---

## Bloque 1 — La arquitectura elástica y qué cambió (2,5 min)

**Lo primero, antes de hablar: lanzar la carga.** En la terminal 2:

```bash
make carga K6=escalado-api.js ETIQUETA=video-api ARGS="-e TASA=90 -e DURACION=14m"
```

> **`TASA=90`, no 50.** Con 50 el grupo se queda en **dos** instancias: el
> objetivo del autoscaler son 32 peticiones/s por instancia y 50 solo justifica
> dos. Comprobado en el ensayo. Con 90 llega a **tres en 84 segundos**.

Decir en voz alta que se acaba de lanzar y que volveremos a ella.

Con el diagrama de `vista-de-despliegue.md` delante:

- **Balanceador HTTPS** con certificado gestionado sobre `sslip.io`, en modo
  `RATE`: reparte por peticiones por segundo, no por conexiones.
- **Dos grupos de 1 a 3 instancias**, cada uno con su autoscaler y **su propia
  señal**. El de la API mira la utilización del balanceador; el de los workers,
  la profundidad de la cola `bulk`. Es la exigencia del enunciado: no vale
  escalar la API y dejar fija la capa worker.
- **`mooc-cola`, una máquina dedicada** con Redis y Mailpit. Fuera de los dos
  grupos, porque un grupo que escala no puede llevar dentro el estado
  compartido. Se documenta como punto único de falla (ADR-0019, D2).
- **Cloud SQL y Cloud Storage** sin cambios respecto a la Entrega 2, a
  propósito: así la comparación de rendimiento mide el escalado y no otra cosa.

Qué cambió frente a la Entrega 2, en una frase: **las dos máquinas fijas se
convirtieron en dos grupos elásticos, y lo que era estado dentro de la máquina
salió a una máquina propia.**

---

## Bloque 2 — Recorrido conectado profesor → estudiante (4,5 min)

Todo por el frontend, sin tocar la API a mano. Las rutas están en
`docs/entrega3/verificacion-frontend.md` si alguien se pierde.

### Profesor (2,5 min)

1. `#/login` como `profesor@mooc.local`.
2. `#/studio` → **Crear un curso**. Título con la fecha para que sea único.
3. Dentro de la versión: **Nuevo módulo** → **Nueva unidad**.
4. Añadir una **lectura** en Markdown.
5. Añadir una **imagen** y un **video** desde la biblioteca de medios.
   **Aquí está el momento clave:** mientras el video se procesa, la interfaz lo
   muestra como `Recibido` → `En revisión` → `Procesando`, y **no deja
   publicarlo**. Decirlo: *un archivo en procesamiento no se ofrece como
   disponible*, que es un requisito explícito del enunciado.
6. Crear un **quiz** con una pregunta.
7. **Validar** la versión y **publicar**. Enseñar que la versión queda
   inmutable.
8. **Salir** con el botón de la cabecera.

### Estudiante (2 min)

1. `#/login` como `estudiante2@mooc.local`.
2. `#/catalog` → buscar el curso → **Explorar curso** → **Inscribirme**.
3. Abrir la lección, ver la imagen, **reproducir el video**. Dejar que avance
   unos segundos: eso demuestra el acceso autorizado al almacenamiento de
   objetos, porque el manifiesto y los segmentos salen de Cloud Storage con
   credencial de reproducción.
4. Presentar el **quiz**, entregarlo y ver el resultado.
5. `#/learning`: el **porcentaje de progreso**, calculado por el servidor.
6. **Salir.**

> Si el video tarda en procesarse, seguir con el quiz y volver. No esperar en
> silencio delante de la cámara.

---

## Bloque 3 — Réplicas, continuidad, base y objetos (3 min)

Volver a la terminal 1. **La carga del bloque 1 lleva ya unos siete minutos
corriendo.**

```bash
make instancias
```

Debe haber **tres** instancias de API. Decir: *cuando empezamos el video había
una; nadie ha tocado nada*.

**Tráfico repartido entre réplicas:**

```bash
for i in 1 2 3 4 5 6; do curl -sI https://34-160-11-22.sslip.io/healthz | grep -i x-instancia; done
```

Salen nombres distintos. Es el balanceador repartiendo entre las réplicas.

**Continuidad funcional durante el cambio de capacidad.** Este es el punto que
más vale y se cuenta, no se ejecuta: *todo el recorrido del bloque anterior
ocurrió mientras el grupo pasaba de una instancia a tres. No hubo un corte, ni
una sesión perdida, ni un error.*

**PostgreSQL administrado:**

```bash
make psql-nube
```

```sql
select count(*) from identity.users;
select title, status from authoring.course_versions order by created_at desc limit 3;
select application_name, count(*) from pg_stat_activity group by 1;
```

El curso recién publicado está ahí, y las conexiones son las de tres instancias
con su pool repartido.

**Almacenamiento de objetos:**

```bash
make gcloud ARGS="storage ls gs://mooc-509602-derived"
```

Los derivados HLS del video que acabamos de subir, con su master y sus tres
calidades.

---

## Bloque 4 — Ciclo de escalado de la API (2 min)

Lo que se acaba de ver en vivo es el **crecimiento**. El **retiro** tarda diez
minutos por diseño, así que se enseña grabado, con la línea de tiempo de la
sesión del 28 de octubre (documento 3).

| Tramo | Medido |
| --- | --- |
| Subida de carga → decisión del autoscaler | **73 s** |
| Decisión → la instancia reparte tráfico | **~60 s** |
| Bajada de carga → primera retirada | **~12 min** |

Y el detalle que conviene contar porque demuestra que lo entendimos:

> El informe predecía que la fase baja reduciría «hasta una instancia». Se quedó
> en dos durante treinta y dos minutos. No es un fallo del autoscaler: su
> objetivo son 32 peticiones/s por instancia y esa fase genera unas 40. **La
> previsión estaba mal, no el sistema**, y corregimos la previsión.

Si sobra tiempo, la Sesión C (documento 11): cuando una instancia deja de pasar
`/readyz`, el balanceador la excluye en 14 s y el grupo la recrea a los 111 s.
Dos cosas distintas con dos tiempos distintos.

---

## Bloque 5 — Workers: escalado y recuperación segura (3 min)

**En vivo**, en la terminal 2:

```bash
make carga K6=escalado-medios.js ETIQUETA=video-workers ARGS="-e VIDEOS=9 -e EMAIL=profesor@mooc.local"
```

Mientras sube, explicar la señal: `cola_bulk_trabajos` cuenta los pendientes
**y los que están en curso**, a dos trabajos por instancia.

> **Nueve videos, no cinco.** Con cinco la señal se queda en `total=4` —alguno
> termina antes de que los otros entren a la cola— y eso solo pide **dos**
> instancias. Con nueve la señal llega a 7 y pide tres. Comprobado en el
> ensayo. Y tarda **de tres a cinco minutos**, no 110 segundos: hay que lanzarlo
> y seguir hablando, no esperar mirando.

**Antes de lanzarlo, dejar que el grupo de API haya bajado a una instancia.**
Ver la nota de la cuota, abajo.

**Grabado** (documento 4), la parte que no se puede provocar a demanda:

```text
19:02:49  delete mooc-worker-1bs0        ← el autoscaler retira una instancia OCUPADA
19:04:13  transcode_hls 18:59:40 → 19:04:13   273 s, intento 1, succeeded
19:04:17  transcode_hls 18:59:34 → 19:04:17   283 s, intento 1, succeeded
```

Los dos trabajos **terminaron en el primer intento**, sin reintento. El trabajo
necesitó 88 segundos desde que empezó el borrado y los tuvo.

Y la honestidad que da credibilidad al resto: **el plazo que configuramos son
280 segundos y el margen que GCE parece conceder son unos 107**. En las dos
observaciones el trabajo necesitó 88, así que nunca lo hemos agotado. Lo
prudente es bajar el valor configurado para que no prometa lo que no puede
cumplir. Está en el documento de deuda.

---

## Bloque 6 — Resultados comparativos (2,5 min)

Tabla comparativa (documento 5). Tres cifras y una conclusión.

| Escenario 1 | Entrega 2 | Entrega 3 |
| --- | --- | --- |
| L2b, 91 peticiones/s, p95 | 272 ms | **28 ms** |
| L3, 117 peticiones/s, p95 | 8.067 ms | 5.500 ms |
| Errores `5xx` y *timeouts*, todos los niveles | 0 | **0** |

**A 91 peticiones/s somos diez veces más rápidos con la misma base de datos.**
Ahí es donde el autoscaling cambia el resultado.

**Pero el límite no desapareció, se mudó** (documento 6). A 117 peticiones/s la
API va al 19–21 % de CPU con tres instancias, las conexiones a 31 de 47 y Redis
ocioso. Lo que se agota es el disco de la base:

| Nivel | Peticiones/s | Escrituras de disco/s |
| --- | --- | --- |
| L2 | 61 | 144 |
| L2b | 91 | 218 |
| **L3** | **118** | **206**, con picos de 339 |

Las escrituras **dejan de crecer** cuando la carga sube un 30 % más. Un PD-SSD
de 10 GB da 300 escrituras por segundo, y ahí estamos. **La propuesta de
evolución es agrandar el disco de 10 a 100 GB**: multiplica por diez el techo
por unos pocos dólares al mes, sin tocar el *tier* ni una línea de código. El
disco está al 4 % de ocupación — se agranda por rendimiento, no por espacio.

Escenario 2, en dos frases: el tiempo hasta `ready` baja de **9–11 minutos a
3–9** cuando el grupo de workers llega a tres. **Y donde no escala, no mejora**:
con una sola instancia en las dos entregas, la elástica tarda el doble, porque
el worker cruza la red para la cola. Lo decimos porque enseñar solo los niveles
favorables sería hacer trampa.

---

## Bloque 7 — Calidad, deuda técnica y cierre (1,5 min)

**Línea base de calidad** (documento 7): SonarQube Community en el CI y en
local, perfil «Sonar way comprehensive», todo el repositorio. Mantenibilidad A,
duplicación 1,6 %, deuda estimada 2,4 días.

**El inventario de deuda** (documento 8): 22 hallazgos priorizados, cada uno con
impacto, tratamiento, criterio de verificación, responsable y plazo. Los cinco
de prioridad 1 eran condiciones de esta entrega y están atendidos.

**Uno vale la pena contarlo: DT-22, y no lo encontró el analizador.** Midiendo,
dos niveles de carga devolvieron `401` y no subieron nada: las sesiones habían
caducado. El código promete un TTL deslizante en un comentario y nunca extiende
la expiración, y `SessionMaxTTL` se declara, se documenta en `.env.example` y no
se usa en ninguna parte. Un usuario activo doce horas se queda fuera sin aviso.
**Hay deuda que solo aparece ejecutando.**

**Costo** (documento 9): el entorno completo cuesta unos 0,42 USD/h encendido y
se apaga entre sesiones con un `apply`. La sesión de medición fueron 18 horas y
unos 7,5 USD. Dos ajustes que salieron de medir: apagar el generador entre
corridas y vaciar el bucket de derivados, que creció 12,8 GB en una campaña.

**Cierre.** El repositorio tiene el tag `entrega-3`, la URL está en el README y
el entorno se reconstruye con el procedimiento documentado en unos quince
minutos si hace falta para la sustentación síncrona.

---

## Plan B

| Si falla | Qué hacer |
| --- | --- |
| El grupo no creció al llegar al bloque 3 | Seguir con los bloques 4 y 5 y volver. La carga sigue corriendo |
| El video del bloque 2 no acaba de procesarse | Pasar al quiz y volver. Si tampoco, usar un curso ya publicado de la semilla |
| El balanceador devuelve error de TLS | Esperar: el proxy HTTPS tarda un par de minutos tras un encendido. **Comprobarlo antes de grabar** |
| `make psql-nube` no conecta | Enseñar las métricas de Cloud SQL en la consola |
| Se agota el tiempo | Recortar el bloque 4 al mínimo: la tabla de tres filas. Nunca recortar el 2 ni el 5 |
| El tercer worker no aparece y el grupo se queda en 2 | Mirar `make gcloud ARGS="compute instance-groups managed list-instances mooc-worker --zone us-central1-a"`. Si dice `QUOTA_EXCEEDED`, es la cuota de la cuenta: ver abajo |

## La cuota de la cuenta limita el directo

El proyecto tiene un tope de **16 vCPU globales**, y la configuración completa
pide diecisiete:

| | vCPU |
| --- | --- |
| 3 instancias de API | 6 |
| 3 instancias de worker | 6 |
| `mooc-cola` | 1 |
| Generador de carga | 4 |
| **Total** | **17** |

**Los dos grupos no pueden estar en tres a la vez mientras el generador esté
encendido.** No afecta a los resultados del informe —el pico real de las
corridas medidas fue de 15 vCPU, comprobado serie a serie—, pero sí al guion:
los bloques 3 y 5 **tienen que ir en ese orden y no solaparse**. Al llegar al
bloque 5, la carga de la API ya habrá terminado y el grupo estará bajando.

Si el grupo de API sigue en tres al empezar el bloque 5, el tercer worker falla
con `QUOTA_EXCEEDED` y en el video se ve un grupo que no crece. Comprobar con
`make instancias` antes de lanzar los videos.
