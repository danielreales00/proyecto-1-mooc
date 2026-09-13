# Guion de la demostración

Video de la Entrega 1. La interfaz es Postman y la línea de órdenes, como
autorizó el profesor.

**Duración máxima: 20 minutos**, confirmada por el profesor. Los bloques suman
**19,25**, así que hay 45 segundos de margen para tropiezos. Un bloque por
segmento de la §10.2 del enunciado, más apertura y cierre.

**Si un bloque se va de tiempo, se recorta ahí mismo, no se le roba al
siguiente.**

**Índice:** [Tiempos](#tiempos) · [Antes de grabar](#antes-de-grabar) ·
[Cómo llevar la demo](#cómo-llevar-la-demo) · [El guion](#el-guion) ·
[Si algo falla](#si-algo-falla) · [Qué no prometer](#qué-no-prometer)

**Reparto entre las cuatro personas del equipo:** [`reparto.md`](reparto.md).

---

## Tiempos

| Bloque | Min |
| --- | --- |
| 0 · Apertura | 0,5 |
| 1 · Levantar el stack | 1 |
| 2 · Identidad | 2 |
| 2b · Administración | 1 |
| 3 · Autoría y publicación | 2 |
| 3b · Carga multimedia | 1,5 |
| 4 · Catálogo, inscripción y consumo | 1 |
| 5 · Quiz | 1,5 |
| 6 · Progreso y aprobación | 2,5 |
| 7 · Insignia | 1 |
| 8 · Tolerancia a fallos | 1,5 |
| 8b · Observabilidad y la alerta | 2 |
| 9 · Operación | 1 |
| 10 · Cierre | 0,75 |
| **Total** | **19,25** |

**Las esperas del worker en los bloques 2, 6 y 7 no se tocan.** Son la prueba de
que el procesamiento es asíncrono, que es justo lo que el profesor pidió
evidenciar.

---

## Antes de grabar

### Media hora antes

```bash
cd proyecto-1-mooc
make clean          # borra volúmenes: la demo empieza de cero
make up             # construye e inicia; la primera vez tarda
make seed           # 8 cuentas sintéticas
make obs            # Prometheus y Grafana
make demo           # ENSAYO COMPLETO: comprueba que todo pasa
```

`make demo` debe terminar en verde. Si falla, **no grabes**: arregla primero.
Ese guion recorre exactamente lo mismo que vas a enseñar, así que si pasa, la
grabación va a salir.

Después del ensayo, vuelve a dejar la base limpia para que el video empiece sin
datos previos:

```bash
make clean && make up && make seed
```

### Comprobaciones

- [ ] `docker compose ps` — los 9 servicios en `healthy`
- [ ] http://localhost:8090/readyz — los tres en `ok`
- [ ] http://localhost:8026 — Mailpit abierto y **vacío**
- [ ] http://localhost:8090/docs — Swagger UI carga el contrato
- [ ] http://localhost:3002 — Grafana, panel «MOOC · Operación»
- [ ] http://localhost:9091/alerts — Prometheus, las 4 reglas en verde
- [ ] Postman con la colección y el entorno **MOOC local** importados
- [ ] `make build` ya ejecutado: que ninguna orden se ponga a construir en cámara
- [ ] Terminal con fuente grande (≥ 16 pt) y ventana ancha
- [ ] Silenciar notificaciones

### Ventanas en pantalla

Tres, y solo tres. Cambiar entre más de tres marea a quien mira.

1. **Postman** — la interfaz principal
2. **Terminal** — para `make`, `psql` y los logs
3. **Navegador** — Mailpit, Grafana y la URL pública de la insignia

---

## Cómo llevar la demo

**Enseña el resultado, no el camino.** Nadie quiere ver cómo escribes un UUID.
Usa las variables de la colección: ya encadenan las peticiones.

**Cada bloque responde a una pregunta.** Dila en voz alta antes de ejecutar:
«¿qué pasa si el cliente miente sobre su progreso?». Luego ejecuta y deja que la
respuesta se vea en pantalla. Sin eso, es una sucesión de JSON.

**Lee de la pantalla, no de la memoria.** Cuando salga un `422`, señala el
`trace_id` y el arreglo `errors`. Lo que convence es el cuerpo de la respuesta,
no tu afirmación.

**Los tiempos muertos, cúbrelos.** El worker tarda unos segundos en emitir la
insignia. Ese hueco es el mejor momento para explicar el patrón outbox. No lo
edites: es la prueba de que la API no esperó al worker.

**Sé explícito con lo que falta.** En el cierre, no lo escondas. Un tribunal
detecta antes una omisión que un hueco reconocido.

---

## El guion

**Cada paso dice dónde se ejecuta.** `Postman` indica la carpeta y el nombre
exacto de la petición, entre comillas angulares; `Terminal` y `Navegador` son
lo que parecen. Las peticiones de una carpeta que el guion no nombra se pueden
saltar, salvo donde se advierte lo contrario.

### 0 · Apertura — 0,5 min

**No se ejecuta nada.** Solo el repositorio abierto, con `arquitectura/` a la
vista.

> «Plataforma MOOC. Backend en Go como monolito modular con workers
> independientes, todo sobre Docker Compose. Sin frontend, como se autorizó
> para esta entrega: la interfaz es Postman.
>
> El recorrido va a ser el flujo completo: un profesor crea y publica un curso,
> un estudiante se inscribe, lo consume, presenta un quiz, el servidor calcula
> su progreso y, al aprobar, un worker emite una insignia verificable.
>
> Todas las decisiones están en `arquitectura/adr`, quince documentos, cada uno
> con las alternativas que descartamos y por qué. Y el contrato lo sirve la
> propia API en `/docs`: ochenta y ocho operaciones, OpenAPI 3.1.»

Enseña la pestaña de `/docs` diez segundos al pasar, sin recorrerla.

---

### 1 · Levantar el stack — 1 min · *SEG-9, CE-01*

**Terminal.** Nada de Postman en este bloque.

```bash
docker compose ps
curl -s localhost:8090/readyz | jq
```

**Qué señalar:** nueve servicios. La API **no publica puerto propio**: entra por
el proxy. Eso es lo que permite escalarla.

```bash
docker compose up -d --scale api=3 --scale worker=3
docker compose ps | grep -c mooc-api
```

> «Tres instancias de API y tres de worker. Ninguna guarda estado: las sesiones
> viven en Redis y los datos en PostgreSQL. Al final volveré sobre esto.»

**Acredita:** arquitectura y despliegue (CE-01).

---

### 2 · Identidad — 2 min · *SEG-1, CE-02*

**Postman → carpeta `SEG-1 · Identidad`**

1. **«Registro inválido — lista exhaustiva de errores»**. Correo mal formado,
   contraseña corta, nombre vacío.

   > «Fíjense en que devuelve **los tres errores a la vez**, no el primero. Esa
   > mecánica es la misma que valida la publicación de un curso, y es una
   > condición de aceptación del enunciado.»

   Señala: `application/problem+json`, el `trace_id`, el arreglo `errors`.

2. **«Registro de estudiante»** → `202 Accepted`.

   > «Doscientos dos, no doscientos uno. La API **no espera al worker**. El
   > trabajo del correo se registró en PostgreSQL en la misma transacción que
   > el usuario, y se publicó en la cola después del commit.»

3. **Navegador → Mailpit** (http://localhost:8026). El correo ya está. Vuelve a
   Postman y ejecuta **«Leer token del correo (Mailpit)»**, que lo extrae sola.

   > «Ese correo lo envió el worker, no la API. Es el primer componente
   > asíncrono funcionando.»

4. **«Verificar el correo»** → `200`. Después **«Reutilizar el token — debe
   fallar»** → `422`.

   *No te saltes la verificación:* sin ella la cuenta queda sin verificar y el
   login del paso siguiente falla.

5. **«Login»** → **«GET /me»** → **«Cerrar sesión»** → **«GET /me tras cerrar
   sesión — revocación inmediata»** → `401`.

   > «La sesión se resuelve contra Redis en cada petición, así que revocarla
   > tiene efecto **inmediato**. No hay ventana de gracia.»

6. **Límite de tasa. Terminal**, seis intentos contra la misma cuenta:

   ```bash
   for i in $(seq 1 6); do
     curl -s -o /dev/null -w "%{http_code} " -X POST localhost:8090/api/v1/auth/login \
       -H 'Content-Type: application/json' \
       -d '{"email":"profesor@mooc.local","password":"mala"}'
   done; echo
   ```

   > «Cinco cuatrocientos uno y un **cuatrocientos veintinueve**.
   >
   > Y hay un detalle de diseño: el límite es por **cuenta**, no solo por IP.
   > Limitar únicamente por IP castigaría a una universidad entera detrás de un
   > NAT. Así que van dos niveles: uno estrecho por cuenta, cinco por minuto,
   > y otro ancho por IP, sesenta, contra el rociado de contraseñas.»

   Dilo mientras corre el bucle; no hace falta esperar a que termine para
   empezar a explicarlo.

**Acredita:** identidad, autorización y seguridad (CE-02).

---

### 2b · Administración — 1 min · *SEG-1b, RF-01*

**Postman → carpeta `SEG-1b · Administración`.** Ejecuta primero **«Login como
administradora»** —fija `admin_token` y `admin_id`, que las dos siguientes
necesitan— y salta al resto.

1. **«7 · ÚLTIMO ADMINISTRADOR: degradarse a sí misma»** → `409`.

   > «Es la única administradora activa. Degradarse dejaría el sistema sin nadie
   > que pueda administrarlo, así que se rechaza.
   >
   > Y el detalle que importa: esa comprobación va **dentro de la transacción**,
   > con un `FOR UPDATE`. Hacerla antes dejaría una ventana en la que dos
   > peticiones simultáneas degradan a los dos últimos administradores y el
   > sistema se queda cerrado para siempre.»

   *(La función que lo decide es pura y tiene una tabla de ocho casos en
   `admin/domain_test.go`.)*

2. **«12 · Un profesor NO puede administrar»** y después **«13 · …y al
   intentarlo recibe 403»**.

   > «Cuatrocientos tres, no cuatrocientos uno. Cuatrocientos uno sería «no sé
   > quién eres»; esto es «sé quién eres y no te toca».»

**Acredita:** RF-01, gestión de usuarios y roles.

---

### 3 · Autoría y publicación — 2 min · *SEG-2, CE-03*

**Postman → carpeta `SEG-2 · Autoría y publicación`**

1. **«Login como profesor»** y **«Crear curso»** → versión 1 en borrador.

2. **«Publicar un curso vacío — lista exhaustiva de errores»** → `422` con la
   lista de incumplimientos.

   > «Aquí está la condición de aceptación: la validación **no para en el primer
   > error**. Recorre todas las reglas y devuelve la lista completa, para que el
   > profesor arregle todo de una vez.»

3. **«Añadir módulo»** → **«Añadir unidad»** → **«Añadir recurso de texto»**,
   de corrido y sin comentar.

4. **«Añadir recurso de quiz»** y **«Configurar el quiz»**.

   > «Fíjense en que aquí **sí** viaja `is_correct`. Es la ruta del profesor
   > dueño del curso, y la única por la que la clave sale del servidor.»

5. **«Publicar la versión»** → `200`, `published`.

   *(«Previsualizar (HTML saneado)» queda en la carpeta pero no se enseña, por
   tiempo.)*

6. **«INMUTABILIDAD: mutar la versión publicada»** → `409`.

   > «Y esto no lo rechaza Go: lo rechaza **PostgreSQL**, con un trigger. La
   > regla vive donde están los datos, porque es la que más fácil se salta un
   > UPDATE de mantenimiento.»

**Acredita:** autoría y publicación (CE-03), CA-01.

---

### 3b · Carga multimedia — 1,5 min · *SEG-3, CE-04*

Va en la terminal: el guion `subir-video.sh` recorre la carga entera y deja los
seis pasos en pantalla.

1. **La carga completa**, narrando por encima mientras corre:

   ```bash
   make subir ARCHIVO=demo.mp4
   ```

   > «Declaración, URLs firmadas por parte, subida, estado de la carga,
   > completado y verificación. Fíjense en que **los bytes no pasan por la
   > API**: el cliente sube directamente al almacén y la API solo firma. Y esa
   > consulta de estado es lo que hace la carga **reanudable**: hay
   > veinticuatro horas para terminarla.»

2. **La verificación del worker**, en el último paso que imprime el guion:

   > «El estado pasa a `clean`. Y aquí está lo importante: el servidor
   > **recalculó el SHA-256 leyendo del bucket** y **detectó el MIME por los
   > bytes**. Lo que el cliente declaró es informativo; lo que manda es lo que
   > el servidor comprueba.»

3. **Un negativo**, que vale más que los positivos: un PDF renombrado a `.mp4`
   y declarado como vídeo.

   > «El tipo real es `application/pdf`: un ejecutable renombrado no entra. Y va
   > a **cuarentena**, no a la papelera: si hay que revisarlo, está.»

**Lo que falta aquí**, y conviene decirlo en el momento: el escaneo antimalware
con ClamAV y la transcodificación a HLS. Son los pasos 2 y 3 del plan que está
en `arquitectura/media-plan.md`.

**Acredita:** parte de multimedia y distribución (CE-04), RF-05.

---

### 4 · Catálogo, inscripción y consumo — 1 min · *SEG-5*

**Postman → carpeta `SEG-5 · Catálogo, inscripción y consumo`.** Las cuatro
peticiones de corrido, comentando solo la última.

1. **«Catálogo público (sin sesión)»** → aparece el curso recién publicado.
2. **«Login como estudiante»** e **«Inscribirse»**.
3. **«Contenido con progreso»** → el árbol, con el texto ya renderizado.

   > «El contenido llega como HTML ya saneado, y el recurso de quiz **no trae
   > ninguna clave**.»

---

### 5 · Quiz — 1,5 min · *SEG-6, CE-05*

**Postman → carpeta `SEG-6 · Quiz`**

1. **«Iniciar intento (congela el snapshot)»**. Deja el JSON en pantalla unos
   segundos.

   > «Este es el snapshot congelado. Cada opción trae **dos campos**:
   > `stable_id` y `text_md`. No hay `is_correct`, ni siquiera como campo nulo.
   >
   > No es un `omitempty`: son **dos tipos distintos en Go**. El del estudiante
   > no tiene el campo, así que no hay forma de que se filtre por descuido.»

2. **«Guardado parcial»** → `200`. Sin comentarla: es la que deja las respuestas
   guardadas, y el envío califica eso. **No se puede saltar** — el envío va con
   cuerpo vacío y sin ella la nota sale en cero.

3. **«Enviar (calificación en servidor)»** → nota calculada.

   > «La nota la calcula el servidor contra las claves de la base, no contra el
   > snapshot. Y con la política `correctness` le decimos qué acertó, pero **no
   > cuáles eran las correctas**, porque le queda otro intento.»

4. **«Reenviar: idempotente»** → misma nota.

   > «Idempotente. Reenviar no recalifica ni crea un segundo intento.»

**Acredita:** evaluación académica (CE-05), CA-04.

---

### 6 · Progreso y aprobación — 2,5 min · *SEG-7, CE-06*

Este es el bloque que más impresiona. No lo corras.

**Antes de empezar, sin comentarlo**, lanza en la terminal el trabajo que va a
morir para el bloque 8b. Tarda 3,5 minutos en llegar a la DLQ, justo lo que
duran los bloques 6, 7 y 8:

```bash
docker compose exec postgres psql -U mooc -d mooc -c \
  "INSERT INTO platform.job_runs (job_key, type, queue, status, payload)
   VALUES ('email.send:demo-dlq','email.send','critical','queued',
           '{\"template\":\"inexistente\",\"to\":\"x@mooc.local\"}'::jsonb)
   ON CONFLICT (job_key) DO UPDATE SET status='queued', attempt=0,
           created_at = now() - interval '1 hour';"
```

**Postman → carpeta `SEG-7 · Progreso y aprobación`**

1. **«SEÑAL FRAUDULENTA: enviar el porcentaje»** → `422`.

   > «El cliente reporta **evidencias**, nunca conclusiones. Un porcentaje
   > enviado por el cliente se rechaza… y además se **audita**.»

   **Terminal**, para ver que además quedó auditado:

   ```bash
   docker compose exec postgres psql -U mooc -d mooc -x -c \
     "SELECT action, metadata FROM audit.events
       WHERE action='progress.tampering_attempt' ORDER BY id DESC LIMIT 1;"
   ```

   > «Ahí está el intento, con el cuerpo que envió y quién lo envió.»

2. **«Evidencia legítima: apertura»** → aceptada.

3. **«Heartbeat inmediato: rechazado por cadencia»** → `reject_reason:
   "cadence"`.

   > «Inundar de heartbeats no simula permanencia. El servidor exige diez
   > segundos entre uno y otro, con **su** reloj: el `client_ts` se registra
   > pero no se usa para calcular.»

4. **«Heartbeat espaciado (esperar >10 s)»**, dos veces, **esperando ~11 s
   antes de cada una**. Cubre las esperas explicando el punto 3.

5. **«Progreso calculado por el servidor»** → `approved`, 100 %.

   > «El porcentaje sale de los recursos **obligatorios y visibles**. Los
   > opcionales no cuentan. Y al aprobar se registró el trabajo de la insignia.»

**Acredita:** progreso e insignias (CE-06), CA-05.

---

### 7 · Insignia — 1 min · *SEG-8, CA-07*

**Postman → carpeta `SEG-8 · Insignia`**

1. **«Mis insignias»**. Si aún no está, repite en unos segundos y aprovecha:

   > «La emite un worker, no la petición. Por eso puede tardar un momento.»

2. **«Verificación PÚBLICA (sin sesión)»**.

   *No te saltes «Mis insignias»:* es la única petición que fija `badge_code`,
   así que sin ella esta otra no tiene qué verificar.

   > «Pública, sin autenticación. Nombre, curso, versión, fecha y estado.
   > **No aparece el correo del estudiante** por ningún lado, y el código es
   > aleatorio: no deriva de su identidad.»

3. **«La imagen, descargada SIN credenciales»**, y después **abre esa misma
   `image_url` en el navegador**. Es el momento más visual del video: aparece la
   insignia dibujada, con el nombre, el curso y el código.

   > «La imagen la generó el worker y vive en el almacén de objetos, no en la
   > base: el enunciado lo pide así en la sección siete. Y la URL **no está
   > firmada** a propósito, porque una insignia que caduca a los quince minutos
   > no sirve para acreditar nada. El bucket tiene lectura anónima y solo ese.»

**Acredita:** CA-07.

---

### 8 · Tolerancia a fallos — 1,5 min · *SEG-4, CA-03*

El bloque técnicamente más importante. **Terminal**, salvo el último paso del
8b.

> «Todos los trabajos están en PostgreSQL, no solo en Redis. Se registran en la
> misma transacción que el cambio de dominio y se publican en la cola **después**
> del commit. Eso permite recuperarlos.»

**Devolver a la cola el trabajo de la insignia** que se acaba de emitir:

```bash
docker compose exec postgres psql -U mooc -d mooc -c \
  "UPDATE platform.job_runs SET status='queued', created_at = now() - interval '1 hour'
    WHERE id = (SELECT max(id) FROM platform.job_runs WHERE job_key LIKE 'badge.issue:%');"
```

> «Ahora hay un trabajo huérfano: registrado, pero que nadie va a ejecutar
> porque no está en la cola de Redis. Un proceso periódico, el *reaper*, lo
> republica.»

Espera ~30 s y comprueba:

```bash
docker compose exec postgres psql -U mooc -d mooc -c \
  "SELECT job_key, status, attempt FROM platform.job_runs
    WHERE job_key LIKE 'badge.issue:%' ORDER BY id DESC LIMIT 1;"
docker compose exec postgres psql -U mooc -d mooc -c \
  "SELECT count(*) FROM badges.badges
    WHERE enrollment_id = (SELECT split_part(job_key,':',2)::uuid
                           FROM platform.job_runs
                           WHERE job_key LIKE 'badge.issue:%' ORDER BY id DESC LIMIT 1);"
```

> «**Intento 2**: el worker lo ejecutó de verdad, por segunda vez. Y sigue
> habiendo **una sola insignia** para esa inscripción. Esa es la idempotencia:
> no es que no se haya ejecutado, es que ejecutarlo dos veces no produce dos
> salidas.»

*La consulta se acota a la inscripción a propósito. Contar toda la tabla
devolvería el número de ensayos que llevas, y en pantalla contradiría lo que
acabas de decir.*

**Acredita:** CA-03, tolerancia a fallos.

---

### 8b · Observabilidad y la alerta — 2 min · *SEG-4, CE-07*

**Antes de grabar este bloque:** `make obs` (Prometheus y Grafana tardan ~20 s).

1. **Navegador → Grafana** (http://localhost:3002), panel «MOOC · Operación».
   Enseña las filas.

   > «Ocho gráficas, y ninguna está de adorno: cada una sostiene un objetivo
   > concreto del enunciado. Latencia p95, entregas duplicadas descartadas,
   > evidencias de progreso rechazadas por motivo, y esta de aquí arriba, la
   > dead-letter queue.»

2. **Terminal.** Los reintentos del trabajo que lanzaste al empezar el bloque 6:

   ```bash
   docker compose logs worker | grep -E "trabajo fallido|ALERTA"
   ```

   > «Una plantilla de correo que no existe, así que el worker falló las tres
   > veces. Ahí está el backoff: los reintentos se van separando, porque
   > reintentar en ráfaga castiga a un servicio que ya está caído. Y al
   > tercero, la alerta.»

   Debería estar ya el `ALERTA: trabajo en la dead-letter queue`. Si no,
   sigue el log con `-f` unos segundos.

3. **Navegador → Prometheus** — http://localhost:9091/alerts

   > «`TrabajoEnDeadLetterQueue`, en estado *firing*. Esta es literalmente la
   > condición del enunciado: tras tres reintentos fallidos, el trabajo llega a
   > la DLQ y **emite una alerta**.»

   **Y el detalle que hace que esto funcione**, que además se puede enseñar en
   lugar de solo contarlo. **Postman → carpeta `SEG-4 · Procesamiento y
   fallos`**, petición **«3 · Los contadores nacen en cero — /metrics»**:

   > «Fíjense en que `jobs_dead_letter_total` ya existía, en cero, **antes de
   > que fallara nada**. Los contadores se declaran al arrancar el worker. Sin
   > eso, `increase()` no ve el salto de una serie que no existía a su primer
   > valor, y la alerta se quedaría muda justo el día que hace falta.»

   Y **«4 · Las cuatro reglas de alerta»** para enseñar que las reglas están
   versionadas con el código, no configuradas a mano en una interfaz.

4. **Recuperarlo por la API de administración.** Vuelve a Postman, carpeta
   `SEG-1b · Administración`, y ejecuta dos peticiones seguidas:

   **«9 · Trabajos en la cola de muertos»** → ahí está el que acaba de morir, y
   la petición se queda con su `id` sola.

   **«10 · Reencolar: 202 si está en la DLQ, 409 si no»** → `202`.

   > «Y no hace falta entrar a la base a mano: hay una ruta de administración
   > para esto. Se reencola **con la misma `job_key`**, que es lo que hace que
   > volver a ejecutarlo no produzca una segunda salida.
   >
   > Y la de al lado, «11 · RECHAZO», hace lo contrario sobre un trabajo ya
   > completado y recibe `409`: solo se recupera lo que está muerto o falló.»

**Acredita:** CA-03 y calidad operativa (CE-07).

---

### 9 · Operación — 1 min · *SEG-9, CE-07*

1. **Postman → carpeta `SEG-4`**, peticiones **«1 · Vivo — /healthz»** y
   **«2 · Listo — /readyz»**.

   > «Dos sondas distintas a propósito: la primera dice que el proceso está en
   > pie, la segunda que PostgreSQL, Redis y el almacén responden. Es la
   > diferencia entre reiniciar un contenedor y no mandarle tráfico todavía.»

2. **Terminal.** Auditoría inmutable:

   ```bash
   docker compose exec postgres psql -U mooc -d mooc -c \
     "UPDATE audit.events SET action='manipulado' WHERE id=1;"
   ```

   > «PostgreSQL lo rechaza. La auditoría es de solo inserción.»

   Y consultarla no exige `psql`: `GET /api/v1/admin/audit?action=...` la
   expone filtrada por acción, actor o entidad.

   > «Cada evento trae quién, desde qué IP y con qué `trace_id`, así que una
   > acción de la interfaz se puede seguir hasta su registro.»

3. **El recorrido entero, automático:**

   ```bash
   make demo
   ```

   > «Treinta y tres aserciones, todas contra el sistema en ejecución. Este
   > guion es reproducible: cualquiera puede clonar el repositorio y correrlo.»

4. **Navegador → GitHub**, pestaña Actions: el run en verde.

   > «Build, lint, análisis de seguridad con gosec y govulncheck, migraciones,
   > pruebas y un trabajo de extremo a extremo que levanta Compose y corre esto
   > mismo.»

**Acredita:** calidad operativa (CE-07).

---

### 10 · Cierre — 0,75 min

> «Lo que han visto es el flujo de trabajo completo: registro, autoría,
> publicación, inscripción, consumo, evaluación, progreso e insignia, con los
> workers funcionando y las condiciones de aceptación comprobadas.
>
> **Lo que falta**, y lo digo claro: **sesenta de las ochenta y ocho
> operaciones** responden hoy; las veintiocho restantes están marcadas como
> planificadas en el propio contrato.
>
> De multimedia han visto la carga reanudable y la verificación en servidor;
> falta el escaneo antimalware y la transcodificación a HLS. Las decisiones
> están tomadas en los ADR cinco, once y quince, y el plan paso a paso está en
> `arquitectura/media-plan.md`.
>
> La decisión fue anchura antes que profundidad: demostrar el flujo completo
> pesaba más que dos módulos perfectos y siete ausentes.
>
> Todo lo demostrado es reproducible con `make up && make seed && make demo`.»

---

## Si algo falla

| Síntoma | Qué hacer |
| --- | --- |
| Un servicio no arranca | `docker compose logs <servicio> --tail=50`. Casi siempre es un puerto ocupado: los del proyecto se cambian en `.env` |
| El correo no llega a Mailpit | `docker compose logs worker --tail=30`. Comprueba que `worker` esté `healthy` |
| La insignia no aparece | Espera 10 s más. Si no, `docker compose logs worker \| grep badge` |
| Un heartbeat sale rechazado por cadencia | Es lo correcto. Espera 11 s |
| La inscripción no llega a `approved` | Faltan heartbeats espaciados. `GET /progress` muestra cuántos obligatorios faltan |
| Algo se rompe en directo | **No improvises arreglos.** Di «esto lo tengo cubierto por `make demo`», ejecútalo y sigue |

**Regla de oro:** si vas a grabar en una toma, ensaya entero al menos una vez.
Si vas a editar, graba cada bloque por separado y no cortes los tiempos de
espera del worker: son la prueba de que el procesamiento es asíncrono.

---

## Qué no prometer

Frases que **no** hay que decir, porque no son ciertas hoy:

- ~~«el backend está completo»~~ → «el flujo completo funciona; de multimedia
  faltan el antimalware y la transcodificación»
- ~~«soporta 2.000 usuarios concurrentes»~~ → «está diseñado para eso; la prueba
  de carga es trabajo pendiente»
- ~~«transcodifica video»~~ → el módulo de medios sube y verifica, no transcodifica
- ~~«tiene CDN»~~ → el contrato lo contempla; la implementación va en GCP
- ~~«cumple WCAG»~~ → no hay interfaz que auditar

Reconocer un hueco cuesta diez segundos. Que te lo encuentren cuesta el
criterio entero.
