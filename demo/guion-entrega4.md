# Guion del video. Entrega 4

Cloud Run y Pub/Sub. **Máximo 20 minutos.** Los bloques suman 19.

Este documento basta para grabar. Trae las órdenes exactas, lo que debe verse
en pantalla y qué hacer si algo falla. Todas las órdenes se lanzan desde la
raíz del repositorio, con Docker encendido.

## Qué pide el enunciado y dónde se cubre

El video debe demostrar cuatro cosas. Si un bloque se cae por tiempo, esta
tabla dice qué queda sin cubrir.

| Lo que se pide | Bloque | En vivo o evidencia |
| --- | --- | --- |
| La aplicación desplegada | 1 y 2 | En vivo, por el frontend |
| El recorrido completo de una tarea por Pub/Sub y los workers | 2 y 3 | En vivo: la subida de un video y sus registros |
| Confirmación y reentregas sin efectos duplicados | 3 | En vivo: una copia publicada a mano |
| La configuración y la evidencia de escalamiento | 4 | En vivo, y la medición del 8 de noviembre como respaldo |
| Las decisiones de acceso y permisos | 5 | En vivo, con `gcloud` |

## La idea del guion

**La carga se lanza al principio y el escalado se recoge después.** Cloud
Monitoring publica las métricas con unos tres minutos de retraso y el backend
tarda unos seis minutos en llegar a tres instancias. Por eso la carga del
backend arranca al empezar el bloque 1 y la de los workers al empezar el bloque
3. Cuando llega el bloque 4, los dos servicios ya crecieron delante de la
cámara.

## Tiempos y reparto

| # | Bloque | Min | Quién |
| --- | --- | --- | --- |
| 0 | Apertura | 0,5 | David |
| 1 | Arquitectura y qué cambió | 2,5 | David |
| 2 | La aplicación desplegada: profesor y estudiante | 5 | Daniel |
| 3 | Una tarea por Pub/Sub, confirmación y reentrega | 3 | Santiago |
| 4 | Configuración y escalamiento | 4 | Michael |
| 5 | Acceso y permisos | 3 | David |
| 6 | Cierre | 1 | Santiago |
| | **Total** | **19** | |

Minutos por persona: David 6, Daniel 5, Michael 4 y Santiago 4. Santiago
maneja la terminal 2 durante todo el video, porque es quien lanza las cargas.

> En la sustentación síncrona cualquiera puede recibir cualquier pregunta. Antes
> de grabar, todos leen el `arquitectura/adr/0020-cloud-run-y-pubsub.md` y la
> sección 6 de `docs/entrega4/arquitectura.md`, que explica las respuestas 204,
> 409 y 500 del worker.

---

## Encender el entorno

El entorno queda apagado entre sesiones: Cloud SQL detenida, la máquina de la
cola apagada y Cloud Scheduler en pausa. Los datos y las cuentas sintéticas se
conservan, así que no hace falta migrar ni sembrar.

1. Credenciales de GCP. Si alguna de estas dos órdenes falla, iniciar sesión con
   `make gcloud ARGS="auth login"` y
   `make gcloud ARGS="auth application-default login"`.

   ```bash
   make gcloud ARGS="auth list"
   make tf ENTORNO=entrega4 ARGS="init"
   ```

2. En `infra/environments/entrega4/terraform.tfvars`, poner
   `cloudsql_encendida = true` y `entorno_encendido = true`.

3. Plan y aplicación. **El plan se lee entero antes de aplicar.** Debe decir
   `0 to add, 4 to change, 0 to destroy`: Cloud SQL a `ALWAYS`, la cola a
   `RUNNING` y los dos trabajos de Scheduler sin pausa. Si dice otra cosa, no
   aplicar y avisar al equipo.

   ```bash
   make tf ENTORNO=entrega4 ARGS="plan -out=p.tfplan"
   make tf ENTORNO=entrega4 ARGS="apply p.tfplan"
   ```

4. Esperar a que la API responda. Cloud SQL tarda de cinco a diez minutos en
   arrancar.

   ```bash
   curl https://mooc-backend-568765476410.us-central1.run.app/readyz
   ```

   Debe responder 200 con `postgres`, `redis` y `objectstore` en `ok`.

## La hora previa, minuto a minuto

Encendido a las 16:00 para grabar a las 17:00.

| Hora | Qué | Cuánto |
| --- | --- | --- |
| 16:00 | Encender el entorno, pasos 1 a 3 | 5 min |
| 16:05 | Esperar `/readyz` en 200, paso 4 | 10 min |
| 16:15 | `make frontend-e2e-nube`, como prueba de humo de todo el bloque 2 | 6 min |
| 16:21 | `make medios-local`, si `capacity-planning/medios/b.mp4` no existe | 2 min |
| 16:25 | Contraseña de las cuentas sintéticas, abrir ventanas y documentos | 10 min |
| 16:35 | Repasar el reparto. **Ninguna carga desde aquí** | — |
| **17:00** | Grabar | |

> **No lanzar ninguna carga ni la prueba de humo después de las 16:35.** Cloud
> Run deja las instancias ociosas unos quince minutos antes de retirarlas. Si a
> las 17:00 el backend ya está en tres, el bloque 4 no enseña nada creciendo.

La contraseña de `profesor@mooc.local` y de `estudiante2@mooc.local` es la
misma y sale de Secret Manager. Se lee en la terminal, fuera de cámara, y nunca
se pega en el chat ni en un documento:

```bash
make gcloud ARGS="secrets versions access latest --secret=seed-password"
```

## Qué tener abierto antes de grabar

### Ventanas

1. Navegador, pestaña 1, en `https://mooc-frontend-568765476410.us-central1.run.app`
   con la **sesión cerrada**.
2. Navegador, pestaña 2, en la consola de GCP, **Cloud Run → mooc-backend →
   Métricas**, con el gráfico «Recuento de instancias de contenedor» en la
   última hora.
3. Navegador, pestaña 3, en **Pub/Sub → Suscripciones → mooc-workers →
   Métricas**, con «Mensajes no confirmados».
4. Terminal 1, para las consultas de los bloques 3, 4 y 5.
5. Terminal 2, para las cargas. La maneja Santiago.
6. Editor con los documentos de abajo.
7. En el escritorio, los dos archivos del bloque 2:
   `arquitectura/recursos/diagrama-enunciado.png` (114 KB) y
   `capacity-planning/medios/b.mp4` (25 MB, 1280×720, 1 minuto). Si el
   navegador corre en Windows y el repositorio en WSL, copiarlos al escritorio
   de Windows, cambiando `<usuario>` por el de Windows:

   ```bash
   cp arquitectura/recursos/diagrama-enunciado.png capacity-planning/medios/b.mp4 /mnt/c/Users/<usuario>/Desktop/
   ```

### Documentos

| Bloque | Archivo | Qué enseñar |
| --- | --- | --- |
| 1 | `docs/entrega4/arquitectura.md` | Sección 5, el diagrama de bloques, y la 7, el de despliegue |
| 3 | `docs/entrega4/arquitectura.md` | Sección 6, la tabla «Respuestas del worker» |
| 4 | `docs/entrega4/evidencias/README.md` | Sección 5, las dos tablas de escalamiento |
| 5 | `docs/entrega4/evidencias/pubsub-e-iam.yaml` | Respaldo si `gcloud` falla en vivo |
| 6 | `docs/entrega4/despliegue.md` | La guía, de arriba abajo |

---

## Bloque 0. Apertura (0,5 min)

Quiénes somos y qué entregamos: la plataforma MOOC en tres servicios de Cloud
Run, con Pub/Sub entre el backend y los workers.

Decir de entrada: **«todo lo que van a ver corre en la nube. El escalado sucede
durante el video. Donde enseñemos una medición anterior, lo decimos».**

## Bloque 1. Arquitectura y qué cambió (2,5 min)

**Al empezar este bloque, Santiago lanza la carga del backend en la terminal 2.**
Dura seis minutos:

```bash
make carga-api-nube TASA=600 DURACION=6m
```

Imprime la hora de inicio. Anotarla: el bloque 4 la necesita.

Con `docs/entrega4/arquitectura.md` abierto en la sección 5:

- Las máquinas virtuales y el balanceador de la Entrega 3 se sustituyen por
  **tres servicios de Cloud Run**: frontend, backend y workers.
- La cola de Redis se sustituye por **un topic de Pub/Sub con una suscripción
  push autenticada** hacia los workers. El backend publica y responde sin
  esperar al trabajo.
- Backend y workers llegan a Cloud SQL por IP privada y a Redis por la VPC, con
  salida directa. Los archivos van a Cloud Storage con URL firmadas.
- El dominio no cambió. Cambiaron dos adaptadores: el que publica trabajos y el
  que los recibe. En local y en el CI sigue corriendo la cola anterior, elegida
  con la variable `QUEUE_BACKEND`.

Bajar a la sección 7 y enseñar la tabla de configuración: región, mínimo 0,
máximo 3, CPU, memoria, concurrencia y tiempo máximo de cada servicio.

## Bloque 2. La aplicación desplegada (5 min)

Todo por el frontend, en la pestaña 1.

### Profesor (3 min)

1. **Iniciar sesión** como `profesor@mooc.local`.
2. **Estudio → Crear un curso.** Título con la fecha, para que sea único, una
   categoría y una descripción. **Crear curso ↗**.
3. **Editar contenido ↗** en la versión 1. **Nuevo módulo** «Fundamentos» y
   **Nueva unidad** «Primeros pasos».
4. **Agregar recurso** de tipo **Lectura**, con un par de líneas en Markdown.
5. **Agregar recurso** de tipo **Video**. Arrastrar `b.mp4` desde el escritorio
   a la zona punteada. Decir mientras sube:
   - La barra enseña los megas enviados. El archivo viaja **directo del
     navegador a Cloud Storage**, en partes y con URL firmadas. No pasa por el
     backend.
   - **El ID se llena solo** y el título se toma del nombre del archivo.
   - Al terminar aparecen los pasos **Subido → Revisión antivirus →
     Procesamiento → Listo**, que avanzan solos. La verificación, la revisión
     antivirus y la transcodificación son tres trabajos. Cada uno se publica en
     Pub/Sub y un worker lo recibe por push. **El bloque 3 sigue este video en
     los registros.**

   Cambiar el título a «Video de prueba», **desmarcar Obligatorio** y **Agregar
   recurso**. Se puede guardar sin esperar a «Listo». Lo que no se puede es
   publicar la versión con el video en procesamiento.
6. **Agregar recurso** de tipo **Imagen**, arrastrar `diagrama-enunciado.png` y
   **desmarcar Obligatorio**. Pasa la revisión en segundos.
7. **Agregar recurso** de tipo **Evaluación**, «Evaluación inicial». Desplegarlo
   y entrar en **Configurar evaluación ↗**. **Agregar pregunta**, rellenar
   enunciado y dos opciones, marcar la correcta y **Guardar evaluación**. Sin
   este paso la validación rechaza la versión.
8. Volver a la versión. Cuando el resumen del video diga **«Video · Listo»**,
   pulsar **Validar contenido** y después **Publicar versión ↗ → Confirmar
   publicación**. La versión queda inmutable.
9. **Salir.**

> El video tarda unos 50 s en procesarse con un worker ya encendido, medido el
> 13 de noviembre, y un minuto más si el worker arranca en frío. Si al llegar al
> paso 8 sigue en «Procesamiento», hacer el paso 7 con calma y volver. No
> esperar en silencio delante de la cámara.

### Estudiante (2 min)

1. **Iniciar sesión** como `estudiante2@mooc.local`.
2. **Explorar cursos**, buscar el curso por su título, **Explorar curso ↗** e
   **Inscribirme al curso ↗**.
3. **Quedarse unos 20 segundos en la lectura.** El progreso lo calcula el
   servidor con el tiempo de permanencia, no lo envía el navegador.
4. Abrir la imagen y **reproducir el video** unos segundos. El manifiesto y los
   segmentos HLS salen de Cloud Storage con una credencial de reproducción.
5. **Evaluación inicial → Iniciar intento**, responder, **Guardar respuestas →
   Entregar evaluación ↗ → Entregar**. Debe aparecer **«Objetivo alcanzado.»**
6. **Mis insignias**: la insignia emitida. Esa emisión también fue un trabajo
   de Pub/Sub, `badge.issue`, y el bloque 3 la usa para la reentrega.

## Bloque 3. Una tarea por Pub/Sub (3 min)

**Al empezar este bloque, Santiago lanza la carga de los workers en la terminal
2.** Son nueve videos a la vez. Cada uno tarda unos 50 s en procesarse y hay
tres workers, así que la cola dura tres turnos, unos cuatro minutos. La orden
espera a que todos terminen:

```bash
make carga-medios-nube VIDEOS=9 PERFIL=b
```

### El recorrido de la tarea

Copiar el ID del video del bloque 2. Está en **Estudio → Biblioteca de medios →
Tus cargas recientes**, con el botón **Copiar ID**. En la terminal 1, pegarlo en
`<id>`:

```bash
make gcloud ARGS="logging read 'resource.type=\"cloud_run_revision\" AND jsonPayload.job_key:\"<id>\"' --freshness 1h --order asc --format='value(timestamp,resource.labels.service_name,jsonPayload.message,jsonPayload.job_key)'"
```

Debe verse la cadena completa, en orden:

| Servicio | Registro | Trabajo |
| --- | --- | --- |
| `mooc-backend` | trabajo publicado | `media.probe` |
| `mooc-workers` | trabajo iniciado, trabajo completado | `media.probe` |
| `mooc-workers` | trabajo publicado | `media.scan` |
| `mooc-workers` | trabajo iniciado, trabajo completado | `media.scan` |
| `mooc-workers` | trabajo publicado | `media.transcode_hls` |
| `mooc-workers` | trabajo iniciado, trabajo completado | `media.transcode_hls` |

Decir: **el worker responde 2xx a Pub/Sub solo después de marcar el trabajo
terminado en la base.** Si muere a mitad, no hay respuesta y Pub/Sub lo vuelve a
entregar. Enseñar la tabla «Respuestas del worker» de la sección 6 de
`docs/entrega4/arquitectura.md`: 204 si terminó, 409 si otro worker lo tiene en
curso, 500 si falló, y en la quinta entrega fallida el mensaje pasa al topic de
mensajes muertos.

La finalización también se ve en la aplicación: el estado del archivo es
«Listo» y el aula reproduce el video.

### La reentrega

Buscar la insignia del bloque 2:

```bash
make gcloud ARGS="logging read 'resource.type=\"cloud_run_revision\" AND jsonPayload.type=\"badge.issue\" AND jsonPayload.message=\"trabajo completado\"' --freshness 1h --limit 1 --format='value(timestamp,jsonPayload.job_key)'"
```

Copiar la `job_key`, que tiene la forma `badge.issue:<id>`, y publicar una copia
del mismo trabajo en el topic:

```bash
K=badge.issue:<id>
make gcloud ARGS="pubsub topics publish mooc-trabajos --message='{\"job_key\":\"$K\",\"trace_id\":\"reentrega-en-vivo\"}' --attribute=tipo=badge.issue,cola=critical,job_key=$K"
```

Esperar unos diez segundos y leer lo que hizo el worker:

```bash
make gcloud ARGS="logging read 'resource.type=\"cloud_run_revision\" AND jsonPayload.job_key=\"$K\"' --freshness 1h --order asc --format='value(timestamp,resource.labels.service_name,jsonPayload.message)'"
```

Debe aparecer **«entrega repetida de un trabajo terminado; se confirma sin
repetirlo»**, sin un segundo «trabajo iniciado». En **Mis insignias** el
estudiante sigue teniendo **una** insignia.

> `$K` solo existe en esa terminal. Si se cierra, volver a escribir la línea
> `K=...`.

## Bloque 4. Configuración y escalamiento (4 min)

### Los límites

```bash
make gcloud ARGS="run services list --region us-central1 --format='table(metadata.name,spec.template.metadata.annotations.\"autoscaling.knative.dev/maxScale\",spec.template.spec.containerConcurrency,spec.template.spec.containers[0].resources.limits.cpu,spec.template.spec.containers[0].resources.limits.memory)'"
```

Debe mostrar máximo 3 en los tres servicios, con su concurrencia, CPU y
memoria. Decir por qué difieren:

- **Backend, concurrencia 20 y 2 GiB.** Cada inicio de sesión reserva 64 MiB
  para el hash de la contraseña, y en la Entrega 2 una ráfaga de inicios agotó
  la memoria de una máquina de 2 GiB.
- **Workers, concurrencia 1 y 4 vCPU.** FFmpeg ocupa todos los núcleos. Con 2
  vCPU, un video de 2 minutos en 1080p tardaba hasta 600 s, que es el plazo
  máximo de confirmación de Pub/Sub. Con 4 vCPU tarda de 286 a 437 s.

### El backend creciendo

La carga del bloque 1 ya terminó. En la terminal 1, con la hora de inicio que
imprimió en `DESDE` y la hora actual en `HASTA`, ambas en UTC. Las fechas de
abajo son de ejemplo:

```bash
make escalado-cloudrun DESDE=2026-11-12T22:00:00Z HASTA=2026-11-12T22:12:00Z
```

Leer la columna de instancias del backend: de 1 a 2 y a 3, con la CPU p90 por
encima del 60 % justo antes de cada salto. **Cloud Run escala el backend por
CPU**, porque cada lectura del catálogo tarda unos 3 ms y la concurrencia nunca
llega al límite. Enseñar el mismo crecimiento en la pestaña 2.

### El respaldo

Abrir la sección 5 de `docs/entrega4/evidencias/README.md`. Es la medición
completa del 8 de noviembre: el backend de 1 a 3 instancias a 600 peticiones
por segundo, y los workers en 3 con hasta 7 mensajes pendientes. **Si el
directo no enseñó algo, se dice y se usa esta tabla.**

### Los workers y los mensajes pendientes

Va al final del bloque para que la carga del bloque 3 lleve unos cinco minutos.
Si la cola ya se vació, el gráfico de la pestaña 3 conserva el pico de mensajes
pendientes de los últimos minutos.
Cloud Monitoring publica con unos tres minutos de retraso. Repetir
`make escalado-cloudrun` con `HASTA` en la hora actual, o enseñar la pestaña 3.
Deben verse los workers en 3 y **mensajes pendientes** en la suscripción: tres
trabajos corren, uno por instancia, y el resto espera en Pub/Sub.

## Bloque 5. Acceso y permisos (3 min)

En la terminal 1.

**Los workers no se pueden invocar desde Internet:**

```bash
curl -s -o /dev/null -w "%{http_code}\n" -X POST https://mooc-workers-568765476410.us-central1.run.app/pubsub/push
```

Responde 404, porque el ingreso es interno.

**Solo dos identidades pueden invocarlos:**

```bash
make gcloud ARGS="run services get-iam-policy mooc-workers --region us-central1"
```

Solo `mooc-pubsub-push` y `mooc-programador` tienen `roles/run.invoker`.

**La suscripción se autentica con esa identidad:**

```bash
make gcloud ARGS="pubsub subscriptions describe mooc-workers --format='yaml(pushConfig,ackDeadlineSeconds,retryPolicy,deadLetterPolicy)'"
```

Debe verse el token OIDC de `mooc-pubsub-push`, con la URL de los workers como
audiencia, el plazo de 600 s, los reintentos de 10 a 600 s y cinco entregas
antes del topic de muertos.

**Solo el backend y los workers publican:**

```bash
make gcloud ARGS="pubsub topics get-iam-policy mooc-trabajos"
```

**Los secretos están en Secret Manager:**

```bash
make gcloud ARGS="secrets list --format='value(name)'"
```

Decir:

- Cada servicio tiene su cuenta, con permisos solo sobre lo que usa. El
  frontend no tiene ninguno sobre datos. La tabla está en la sección 6 de
  `docs/entrega4/despliegue.md`.
- Terraform genera las contraseñas y las escribe directo en Secret Manager.
  Ninguna está en el repositorio, en las imágenes ni en el plan.
- Nadie del equipo creó claves JSON de cuentas de servicio. Las credenciales
  son las de cada persona.

> Si `gcloud` falla en vivo, enseñar `docs/entrega4/evidencias/pubsub-e-iam.yaml`,
> que tiene las mismas políticas leídas del proyecto.

## Bloque 6. Cierre (1 min)

Con `docs/entrega4/despliegue.md` abierto:

- La guía despliega todo desde un proyecto vacío, con las imágenes, los
  servicios, el topic, la suscripción y los permisos.
- Las evidencias están en `docs/entrega4/evidencias/`.
- El hallazgo de la entrega: la prueba de carga demostró que con 2 vCPU un video
  largo no cabía en el plazo de Pub/Sub. Se corrigió con 4 vCPU, y el riesgo que
  queda, un video más largo, está en la sección 11 del informe.

---

## Plan B

| Si falla | Qué hacer |
| --- | --- |
| `/readyz` no responde 200 a las 16:15 | Esperar cinco minutos más: Cloud SQL puede tardar. Si sigue, revisar que el `apply` terminó sin errores |
| El backend no llegó a 3 en el bloque 4 | Decirlo y enseñar la tabla del respaldo. La carga desde una red doméstica puede quedarse corta |
| `make carga-medios-nube` no termina antes del cierre | Dejarla corriendo. Ya enseñó lo que importa: los workers en 3 y los mensajes pendientes |
| El video del bloque 2 no llega a «Listo» | Seguir con la evaluación y volver. Si tampoco, publicar sin el video: quitar el recurso y validar de nuevo |
| La consulta de registros no devuelve nada | Los registros tardan unos segundos. Repetirla. Si sigue vacía, usar `docs/entrega4/evidencias/reentrega-insignia.tsv` |
| `make gcloud` pide iniciar sesión | `make gcloud ARGS="auth login"`, fuera de cámara |
| Se acaba el tiempo | Recortar el bloque 1 a la mitad. Nunca recortar el 3 ni el 4 |

## Después de grabar

1. **Apagar el entorno.** En `terraform.tfvars`, `cloudsql_encendida = false` y
   `entorno_encendido = false`. El plan debe decir `0 to add, 4 to change, 0 to
   destroy`:

   ```bash
   make tf ENTORNO=entrega4 ARGS="plan -out=p.tfplan"
   make tf ENTORNO=entrega4 ARGS="apply p.tfplan"
   ```

2. Subir el video con enlace accesible para el evaluador y ponerlo en la fila 8
   de la tabla del principio del `README.md`, donde hoy dice «Pendiente».
3. Commit en `main` y mover el tag al commit nuevo:

   ```bash
   git tag -f -a entrega-4 -m "Entrega 4: Cloud Run y Pub/Sub"
   git push origin main
   git push -f origin entrega-4
   ```
