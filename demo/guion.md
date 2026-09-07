# Guion de la demostración

Video de la Entrega 1. La interfaz es Postman y la línea de órdenes, como
autorizó el profesor.

**Duración objetivo: 21 minutos.** Un bloque por segmento de la §10.2 del
enunciado, más apertura y cierre.

**Índice:** [Antes de grabar](#antes-de-grabar) ·
[Cómo llevar la demo](#cómo-llevar-la-demo) · [El guion](#el-guion) ·
[Si algo falla](#si-algo-falla) · [Qué no prometer](#qué-no-prometer)

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
- [ ] http://localhost:3002 — Grafana, panel «MOOC · Operación»
- [ ] http://localhost:9091/alerts — Prometheus, las 4 reglas en verde
- [ ] Postman con la colección y el entorno **MOOC local** importados
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

### 0 · Apertura — 1 min

**Qué se ve:** el repositorio abierto, `arquitectura/` a la vista.

> «Plataforma MOOC. Backend en Go como monolito modular con workers
> independientes, todo sobre Docker Compose. Sin frontend, como se autorizó
> para esta entrega: la interfaz es Postman.
>
> El recorrido va a ser el flujo completo: un profesor crea y publica un curso,
> un estudiante se inscribe, lo consume, presenta un quiz, el servidor calcula
> su progreso y, al aprobar, un worker emite una insignia verificable.
>
> Todas las decisiones están en `arquitectura/adr`, quince documentos, cada uno
> con las alternativas que descartamos y por qué.»

---

### 1 · Levantar el stack — 1,5 min · *SEG-9, CE-01*

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

### 2 · Identidad — 3 min · *SEG-1, CE-02*

**Postman → SEG-1**

1. **Registro inválido.** Correo mal formado, contraseña corta, nombre vacío.

   > «Fíjense en que devuelve **los tres errores a la vez**, no el primero. Esa
   > mecánica es la misma que valida la publicación de un curso, y es una
   > condición de aceptación del enunciado.»

   Señala: `application/problem+json`, el `trace_id`, el arreglo `errors`.

2. **Registro válido** → `202 Accepted`.

   > «Doscientos dos, no doscientos uno. La API **no espera al worker**. El
   > trabajo del correo se registró en PostgreSQL en la misma transacción que
   > el usuario, y se publicó en la cola después del commit.»

3. **Mailpit** (navegador). El correo ya está. Vuelve a Postman: la petición
   «Leer token del correo» lo extrae sola.

   > «Ese correo lo envió el worker, no la API. Es el primer componente
   > asíncrono funcionando.»

4. **Verificar** → `200`. **Reutilizar el token** → `422`.

5. **Login**, **GET /me**, **logout**, **GET /me otra vez** → `401`.

   > «La sesión se resuelve contra Redis en cada petición, así que revocarla
   > tiene efecto **inmediato**. No hay ventana de gracia.»

6. **Límite de tasa.** En la terminal, seis intentos contra la misma cuenta:

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

   Demuéstralo: otra cuenta desde la misma IP entra sin problema.

   ```bash
   curl -s -o /dev/null -w "%{http_code}\n" -X POST localhost:8090/api/v1/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"email":"estudiante2@mooc.local","password":"Contrasena-Demo-2026"}'
   ```

**Acredita:** identidad, autorización y seguridad (CE-02).

---

### 3 · Autoría y publicación — 3,5 min · *SEG-2, CE-03*

**Postman → SEG-2**

1. **Login como profesor** y **crear curso** → versión 1 en borrador.

2. **Publicar el curso vacío** → `422` con la lista de incumplimientos.

   > «Aquí está la condición de aceptación: la validación **no para en el primer
   > error**. Recorre todas las reglas y devuelve la lista completa, para que el
   > profesor arregle todo de una vez.»

3. **Módulo → unidad → recurso de texto.**

   Al crear el recurso, señala el `content_md` de la respuesta:

   > «Envié las viñetas con asterisco y tres líneas en blanco. El servidor las
   > guardó normalizadas: guión y una sola línea en blanco. **No guarda lo que
   > le mandan**: parsea, valida y vuelve a serializar en forma canónica. Por eso
   > guardar dos veces lo mismo da el mismo ETag.»

4. **Recurso de quiz** y **configurar el quiz**.

   > «Fíjense en que aquí **sí** viaja `is_correct`. Es la ruta del profesor
   > dueño del curso, y la única por la que la clave sale del servidor.»

5. **Previsualizar** → HTML saneado.

6. **Publicar** → `200`, `published`.

7. **Intentar añadir un módulo a la versión publicada** → `409`.

   > «Y esto no lo rechaza Go: lo rechaza **PostgreSQL**, con un trigger. La
   > regla vive donde están los datos, porque es la que más fácil se salta un
   > UPDATE de mantenimiento.»

   *(Opcional, si hay tiempo: abrir `psql` e intentar el UPDATE a mano.)*

**Acredita:** autoría y publicación (CE-03), CA-01.

---

### 4 · Catálogo, inscripción y consumo — 1,5 min · *SEG-5*

**Postman → SEG-5**

1. **Catálogo sin sesión** → aparece el curso recién publicado.
2. **Login como estudiante** e **inscribirse**.
3. **Contenido con progreso** → el árbol, con el texto ya renderizado.

   > «El contenido llega como HTML ya saneado, y el recurso de quiz **no trae
   > ninguna clave**.»

---

### 5 · Quiz — 2,5 min · *SEG-6, CE-05*

**Postman → SEG-6**

1. **Iniciar intento.** Deja el JSON en pantalla unos segundos.

   > «Este es el snapshot congelado. Cada opción trae **dos campos**:
   > `stable_id` y `text_md`. No hay `is_correct`, ni siquiera como campo nulo.
   >
   > No es un `omitempty`: son **dos tipos distintos en Go**. El del estudiante
   > no tiene el campo, así que no hay forma de que se filtre por descuido.»

2. **Guardado parcial** → `200`.

3. **Enviar** → nota calculada.

   > «La nota la calcula el servidor contra las claves de la base, no contra el
   > snapshot. Y con la política `correctness` le decimos qué acertó, pero **no
   > cuáles eran las correctas**, porque le queda otro intento.»

4. **Reenviar el mismo intento** → misma nota.

   > «Idempotente. Reenviar no recalifica ni crea un segundo intento.»

**Acredita:** evaluación académica (CE-05), CA-04.

---

### 6 · Progreso y aprobación — 3 min · *SEG-7, CE-06*

Este es el bloque que más impresiona. No lo corras.

**Postman → SEG-7**

1. **Señal fraudulenta**: `{"kind":"heartbeat","progress_percent":100}` → `422`.

   > «El cliente reporta **evidencias**, nunca conclusiones. Un porcentaje
   > enviado por el cliente se rechaza… y además se **audita**.»

   **Terminal:**

   ```bash
   docker compose exec postgres psql -U mooc -d mooc -x -c \
     "SELECT action, metadata FROM audit.events
       WHERE action='progress.tampering_attempt' ORDER BY id DESC LIMIT 1;"
   ```

   > «Ahí está el intento, con el cuerpo que envió y quién lo envió.»

2. **Apertura** → aceptada.

3. **Heartbeat inmediato** → `reject_reason: "cadence"`.

   > «Inundar de heartbeats no simula permanencia. El servidor exige diez
   > segundos entre uno y otro, con **su** reloj: el `client_ts` se registra
   > pero no se usa para calcular.»

4. **Espera ~11 s** y repite el heartbeat dos veces. Cubre la espera explicando
   el punto 3.

5. **Consultar el progreso** → `approved`, 100 %.

   > «El porcentaje sale de los recursos **obligatorios y visibles**. Los
   > opcionales no cuentan. Y al aprobar se registró el trabajo de la insignia.»

**Acredita:** progreso e insignias (CE-06), CA-05.

---

### 7 · Insignia — 1,5 min · *SEG-8, CA-07*

**Postman → SEG-8**

1. **Mis insignias.** Si aún no está, repite en unos segundos y aprovecha:

   > «La emite un worker, no la petición. Por eso puede tardar un momento.»

2. **Verificación pública** — pégala en el **navegador**, sin sesión.

   > «Pública, sin autenticación. Nombre, curso, versión, fecha y estado.
   > **No aparece el correo del estudiante** por ningún lado, y el código es
   > aleatorio: no deriva de su identidad.»

**Acredita:** CA-07.

---

### 8 · Tolerancia a fallos — 2 min · *SEG-4, CA-03*

El bloque técnicamente más importante. Va en la terminal.

```bash
docker compose exec postgres psql -U mooc -d mooc -c \
  "SELECT job_key, type, status, attempt FROM platform.job_runs ORDER BY id DESC LIMIT 5;"
```

> «Todos los trabajos están en PostgreSQL, no solo en Redis. Se registran en la
> misma transacción que el cambio de dominio y se publican en la cola **después**
> del commit.»

**Devolver el trabajo de la insignia a la cola**, como haría un reencolado
administrativo:

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
  "SELECT count(*) FROM badges.badges;"
```

> «**Intento 2**: el worker lo ejecutó de verdad, por segunda vez. Y sigue
> habiendo **una sola insignia**. Esa es la idempotencia: no es que no se haya
> ejecutado, es que ejecutarlo dos veces no produce dos salidas.»

**Acredita:** CA-03, tolerancia a fallos.

---

### 8b · Observabilidad y la alerta — 2,5 min · *SEG-4, CE-07*

**Antes de grabar este bloque:** `make obs` (Prometheus y Grafana tardan ~20 s).

1. **Grafana** → panel «MOOC · Operación». Enseña las filas.

   > «Ocho gráficas, y ninguna está de adorno: cada una sostiene un objetivo
   > concreto del enunciado. Latencia p95, entregas duplicadas descartadas,
   > evidencias de progreso rechazadas por motivo, y esta de aquí arriba, la
   > dead-letter queue.»

2. **Forzar un trabajo a la DLQ.** En la terminal:

   ```bash
   docker compose exec postgres psql -U mooc -d mooc -c \
     "INSERT INTO platform.job_runs (job_key, type, queue, status, payload)
      VALUES ('email.send:demo-dlq','email.send','critical','queued',
              '{\"template\":\"inexistente\",\"to\":\"x@mooc.local\"}'::jsonb)
      ON CONFLICT (job_key) DO UPDATE SET status='queued', attempt=0,
              created_at = now() - interval '1 hour';"
   ```

   > «Una plantilla de correo que no existe. El worker va a fallar las tres
   > veces.»

3. **Los reintentos, en vivo:**

   ```bash
   docker compose logs -f worker | grep -E "trabajo fallido|ALERTA"
   ```

   > «Ahí está el backoff: los reintentos se van separando. Y al tercero…»

   Espera al `ALERTA: trabajo en la dead-letter queue`. Son unos dos minutos:
   aprovecha para explicar que el backoff exponencial existe para no castigar
   un servicio que ya está caído.

4. **La alerta en Prometheus** — http://localhost:9091/alerts

   > «`TrabajoEnDeadLetterQueue`, en estado *firing*. Esta es literalmente la
   > condición del enunciado: tras tres reintentos fallidos, el trabajo llega a
   > la DLQ y **emite una alerta**.»

   **Detalle que vale la pena contar:** los contadores se declaran en cero al
   arrancar el worker. Sin eso, `increase()` no ve el salto de una serie que no
   existía a su primer valor, y la alerta se quedaría muda justo el día que
   hace falta.

**Acredita:** CA-03 y calidad operativa (CE-07).

---

### 9 · Operación — 1,5 min · *SEG-9, CE-07*

1. **Auditoría inmutable:**

   ```bash
   docker compose exec postgres psql -U mooc -d mooc -c \
     "UPDATE audit.events SET action='manipulado' WHERE id=1;"
   ```

   > «PostgreSQL lo rechaza. La auditoría es de solo inserción.»

2. **El recorrido entero, automático:**

   ```bash
   make demo
   ```

   > «Treinta y tres aserciones, todas contra el sistema en ejecución. Este
   > guion es reproducible: cualquiera puede clonar el repositorio y correrlo.»

3. **CI:** enseña la pestaña verde en GitHub.

   > «Build, lint, análisis de seguridad con gosec y govulncheck, migraciones,
   > pruebas y un trabajo de extremo a extremo que levanta Compose y corre esto
   > mismo.»

**Acredita:** calidad operativa (CE-07).

---

### 10 · Cierre — 1 min

> «Lo que han visto es el flujo de trabajo completo: registro, autoría,
> publicación, inscripción, consumo, evaluación, progreso e insignia, con los
> workers funcionando y las condiciones de aceptación comprobadas.
>
> **Lo que falta**, y lo digo claro: el módulo de multimedia. Cincuenta y
> cuatro de las ochenta y ocho operaciones responden hoy.
>
> Multimedia no es un hueco en blanco: su esquema está aplicado, sus nueve
> operaciones están en el contrato y validadas, y las decisiones —carga
> multipart directa, ClamAV, FFmpeg y la entrega por cookie firmada— están
> tomadas en los ADR cinco, once y quince. En `arquitectura/media-plan.md` está
> el plan de implementación paso a paso, y cada paso deja algo demostrable.
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

- ~~«el backend está completo»~~ → «el flujo completo funciona; faltan
  multimedia y administración»
- ~~«soporta 2.000 usuarios concurrentes»~~ → «está diseñado para eso; la prueba
  de carga es trabajo pendiente»
- ~~«transcodifica video»~~ → el módulo de medios no está
- ~~«tiene CDN»~~ → el contrato lo contempla; la implementación va en GCP
- ~~«cumple WCAG»~~ → no hay interfaz que auditar

Reconocer un hueco cuesta diez segundos. Que te lo encuentren cuesta el
criterio entero.
