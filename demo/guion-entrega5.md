# Guion del video. Entrega 5

Funciones serverless e infraestructura como código. **Máximo 20 minutos.** Los
bloques suman 18,5.

Este documento basta para grabar. Trae las órdenes exactas, lo que debe verse
en pantalla y qué hacer si algo falla. Todas las órdenes se lanzan desde la
raíz del repositorio, en la rama `main` actualizada (`git pull`), con Docker
encendido.

## Qué pide el enunciado y dónde se cubre

| Lo que pide el video | Bloque | Cómo |
| --- | --- | --- |
| Explicar la migración | 1 y 2 | El análisis, la tarea elegida y el código |
| Demostrar el despliegue con Terraform | 3 | En vivo: un plan sin cambios y una actualización con `plan` y `apply` |
| La ejecución de una tarea mediante Pub/Sub y la función | 4 | En vivo: un estudiante aprueba un curso y la función emite su insignia |
| Reintentos sin duplicar efectos | 4 | En vivo: una copia del mismo trabajo, confirmada sin repetirlo |
| Permisos mínimos, secretos y estado protegidos | 5 | En vivo, con `gcloud` |

## Tiempos y reparto

| # | Bloque | Min | Quién |
| --- | --- | --- | --- |
| 0 | Apertura | 0,5 | David |
| 1 | Análisis serverless y la tarea elegida | 3 | David |
| 2 | La migración en el código | 3 | Santiago |
| 3 | Terraform: estructura, despliegue y actualización | 5 | Michael |
| 4 | Una insignia de Pub/Sub a la función, y su reentrega | 4 | Daniel |
| 5 | Acceso, secretos y estado | 2 | David |
| 6 | Cierre | 1 | Santiago |
| | **Total** | **18,5** | |

> En la sustentación cualquiera puede recibir cualquier pregunta. Antes de
> grabar, todos leen el `arquitectura/adr/0021-insignias-en-una-cloud-run-function.md`
> y la sección 6 de `docs/entrega5/arquitectura.md`.

---

## Encender el entorno

El entorno queda apagado entre sesiones. Los datos y las cuentas sintéticas se
conservan.

1. Credenciales. Si alguna orden falla, iniciar sesión con
   `make gcloud ARGS="auth login"` y
   `make gcloud ARGS="auth application-default login"`.

   ```bash
   make gcloud ARGS="auth list"
   make tf ENTORNO=entrega5 ARGS="init"
   ```

2. En `infra/environments/entrega5/terraform.tfvars`, poner
   `cloudsql_encendida = true` y `entorno_encendido = true`.

3. Plan y aplicación. **El plan se lee entero antes de aplicar.** Debe decir
   `2 to add, 5 to change, 0 to destroy`:
   - crea la NAT y su router;
   - enciende Cloud SQL y la cola;
   - quita la pausa de los dos trabajos de Scheduler;
   - devuelve el token de las migraciones.

   Si dice otra cosa, no aplicar y avisar al equipo.

   ```bash
   make tf ENTORNO=entrega5 ARGS="plan -out=p.tfplan"
   make tf ENTORNO=entrega5 ARGS="apply p.tfplan"
   ```

   El `apply` tarda de cinco a diez minutos, porque espera a que Cloud SQL
   arranque y a que terminen las migraciones.

4. Comprobar la API:

   ```bash
   curl https://mooc-backend-568765476410.us-central1.run.app/readyz
   ```

   Debe responder `postgres`, `redis` y `objectstore` en `ok`.

## La hora previa, minuto a minuto

| Hora | Qué | Cuánto |
| --- | --- | --- |
| 16:00 | Encender el entorno, pasos 1 a 4 | 15 min |
| 16:15 | Preparar el curso del bloque 4, ver abajo | 5 min |
| 16:20 | Comprobar que el plan está limpio, bloque 3 | 2 min |
| 16:25 | Abrir ventanas y documentos, leer la contraseña | 10 min |
| **17:00** | Grabar | |

### El curso del bloque 4

Para que el estudiante apruebe en cámara en menos de dos minutos, el curso
tiene solo una lectura y una evaluación. Se prepara fuera de cámara, por el
frontend, como `profesor@mooc.local`:

1. **Estudio → Crear un curso**, con el título «Curso del video» y la fecha.
2. **Editar contenido ↗** → **Nuevo módulo** → **Nueva unidad**.
3. **Agregar recurso** de tipo **Lectura**, con dos líneas de Markdown.
4. **Agregar recurso** de tipo **Evaluación**. **Configurar evaluación ↗**,
   una pregunta con dos opciones, marcar la correcta y **Guardar evaluación**.
5. **Validar contenido → Publicar versión ↗ → Confirmar publicación**.

### La contraseña

La de `profesor@mooc.local` y `estudiante2@mooc.local` es la misma y sale de
Secret Manager. Se lee fuera de cámara y nunca se pega en un documento:

```bash
make gcloud ARGS="secrets versions access latest --secret=seed-password"
```

## Qué tener abierto antes de grabar

1. Navegador, pestaña 1, en `https://mooc-frontend-568765476410.us-central1.run.app`,
   con la sesión cerrada.
2. Navegador, pestaña 2, en la consola de GCP, abrir la función
   `mooc-insignias` y la pestaña de sus activadores. Enseña el disparador de
   Eventarc sobre el topic `mooc-insignias`. Comprobar antes de grabar dónde
   está la pestaña, porque la consola cambia de nombre las secciones.
3. Terminal 1, para Terraform y las consultas.
4. Editor con estos archivos abiertos:

| Bloque | Archivo | Qué enseñar |
| --- | --- | --- |
| 1 | `docs/entrega5/analisis-serverless.md` | Secciones 3 y 5 |
| 2 | `backend/funcion.go` | Todo, son unas 90 líneas |
| 2 | `backend/internal/adapters/queue/evento.go` | El comentario de `Evento` y `Atender` |
| 2 | `backend/internal/adapters/pubsub/pubsub.go` | `topicDe` |
| 3 | `infra/environments/entrega5/funcion.tf` | El zip, la función y el disparador |
| 3 | `infra/environments/entrega5/terraform.tfvars` | El bloque `funcion` |
| 3 | `docs/entrega5/despliegue.md` | Sección 3, «Recursos que no gestiona Terraform» |
| 6 | `docs/entrega5/evidencias/README.md` | Las cuatro secciones |

---

## Bloque 0. Apertura (0,5 min)

Quiénes somos y qué entregamos: una tarea del worker migrada a una Cloud Run
function activada por Pub/Sub, y toda la arquitectura desplegada con
Terraform.

## Bloque 1. Análisis serverless y la tarea elegida (3 min)

Con `docs/entrega5/analisis-serverless.md`:

- **Sección 1.** Ya eran serverless el frontend, el backend y los workers en
  Cloud Run, Pub/Sub, Cloud Scheduler y Cloud Storage. Escalan a cero o cobran
  por uso.
- **Sección 2.** No lo son Cloud SQL, que cobra encendida, y la máquina de la
  cola con Redis y Mailpit.
- **Sección 3.** Las once oportunidades. Comentar tres:
  - **O1**, la implementada.
  - **O4**, Transcoder API para el video: quitaría FFmpeg y el riesgo del plazo
    de 600 s de Pub/Sub.
  - **O7 frente a O8**, Memorystore frente a Firestore para las sesiones.
    Memorystore es gestionado pero no serverless, porque cobra por capacidad
    reservada.

  Las demás solo se documentan, como pide el enunciado.
- **Sección 5.** Por qué `badge.issue`:
  - usa Cloud SQL y Cloud Storage;
  - dura menos de un segundo;
  - no necesita FFmpeg ni ClamAV;
  - un duplicado se vería como una segunda insignia.

  Las de medios necesitan binarios que el runtime de la función no trae.

## Bloque 2. La migración en el código (3 min)

1. **`backend/funcion.go`**. Es el punto de entrada de la función. Registra
   `EmitirInsignia`, abre la base y el almacén en la primera entrega y le pasa
   el evento a `queue.Evento`. Decir: **la función no reimplementa la tarea.
   Usa el mismo `badges.Service` y el mismo ejecutor que el worker**, así que
   el resultado es el mismo.
2. **`backend/internal/adapters/queue/evento.go`**. El comentario de `Evento`
   resume las respuestas:
   - devuelve `nil` y Eventarc confirma;
   - devuelve un error y Eventarc reintenta;
   - al quinto reclamo fallido marca el trabajo `dead` y corta. Los intentos
     se cuentan en la base porque el evento no trae el número de entrega.
3. **`backend/internal/adapters/pubsub/pubsub.go`**, `topicDe`. El backend
   publica cada tipo en su topic según `PUBSUB_TOPICOS`. `badge.issue` va a
   `mooc-insignias`. El worker, al arrancar, retira de sus manejadores los
   tipos con otro topic, así que **deja de ejecutar la tarea**.

## Bloque 3. Terraform (5 min)

### La estructura (1,5 min)

Con `infra/environments/entrega5/funcion.tf`:

- `archive_file` empaqueta el código del backend. El objeto se sube con su hash
  en el nombre: **si el código cambia, el plan redespliega la función**.
- `google_cloudfunctions2_function`:
  - `build_config`, con `go126` y una cuenta de compilación propia;
  - `service_config`, con 0 a 3 instancias, la salida directa a la VPC para
    llegar a Cloud SQL y el secreto por referencia;
  - `event_trigger`, el disparador de Eventarc sobre `mooc-insignias`, con
    reintentos.
- Arriba, el topic `mooc-insignias` y quién puede publicar en él.

Decir también que `cloudrun.tf` gestiona los tres servicios con sus imágenes,
su configuración y su escalamiento. Las migraciones corren dentro del `apply`.

### Todo coincide con la nube (1 min)

```bash
make tf ENTORNO=entrega5 ARGS="plan -detailed-exitcode"
```

Debe decir **«No changes»**: nada se configuró a mano.

### Una actualización en vivo (2,5 min)

En `infra/environments/entrega5/terraform.tfvars`, dentro de `funcion`, cambiar
`tiempo_maximo_s = 60` por `tiempo_maximo_s = 90`.

```bash
make tf ENTORNO=entrega5 ARGS="plan -out=p.tfplan"
```

El plan debe decir `0 to add, 1 to change, 0 to destroy`, con
`timeout_seconds = 60 -> 90` en `google_cloudfunctions2_function.insignias`.
Leerlo en voz alta y aplicar:

```bash
make tf ENTORNO=entrega5 ARGS="apply p.tfplan"
make gcloud ARGS="functions describe mooc-insignias --region us-central1 --format='value(serviceConfig.timeoutSeconds)'"
```

La última orden debe imprimir `90`. Mientras aplica, enseñar la sección 3 de
`docs/entrega5/despliegue.md`, la de los recursos que no gestiona Terraform:
- el proyecto;
- el bucket del estado;
- el bucket interno de Cloud Run functions;
- la suscripción que crea Eventarc;
- las imágenes, que se preparan con `make publicar`.

## Bloque 4. Una insignia de Pub/Sub a la función (4 min)

### La tarea (2,5 min)

En la pestaña 1:

1. **Iniciar sesión** como `estudiante2@mooc.local`.
2. **Explorar cursos**, buscar «Curso del video», **Explorar curso ↗** e
   **Inscribirme al curso ↗**.
3. **Quedarse unos 20 segundos en la lectura.** El servidor calcula el progreso
   con el tiempo de permanencia.
4. **Evaluación → Iniciar intento**, responder bien, **Guardar respuestas →
   Entregar evaluación ↗ → Entregar**. Debe aparecer «Objetivo alcanzado.»
5. **Mis insignias**: la insignia nueva.

En la terminal 1:

```bash
make gcloud ARGS="logging read 'log_id(\"run.googleapis.com/stdout\") AND jsonPayload.type=\"badge.issue\"' --freshness 30m --order asc --format='value(timestamp,resource.labels.service_name,jsonPayload.message,jsonPayload.job_key,jsonPayload.topic)'"
```

Deben verse tres líneas con la misma `job_key`:

| Servicio | Registro |
| --- | --- |
| `mooc-backend` | trabajo publicado, con el topic `mooc-insignias` |
| `mooc-insignias` | trabajo iniciado |
| `mooc-insignias` | trabajo completado |

Decir: **el backend publicó en Pub/Sub, Eventarc entregó el evento y la función
escribió la insignia en Cloud SQL y su imagen en Cloud Storage. `mooc-workers`
no aparece.** Enseñar el disparador en la pestaña 2.

### La reentrega (1,5 min)

Copiar la `job_key` **completa** de la salida anterior. Empieza por
`badge.issue:`. No se le añade nada delante, porque con el prefijo duplicado la
clave no existe y la prueba no demuestra nada.

```bash
K=<la job_key completa, tal cual>
make gcloud ARGS="pubsub topics publish mooc-insignias --message='{\"job_key\":\"$K\",\"trace_id\":\"reentrega-en-vivo\"}' --attribute=tipo=badge.issue,cola=critical,job_key=$K"
```

Unos diez segundos después:

```bash
make gcloud ARGS="logging read 'jsonPayload.job_key=\"$K\"' --freshness 30m --order asc --format='value(timestamp,resource.labels.service_name,jsonPayload.message)'"
```

Debe aparecer al final **«entrega repetida de un trabajo terminado; se confirma
sin repetirlo»**, sin un segundo «trabajo iniciado». En **Mis insignias** sigue
habiendo **una** insignia de este curso.

## Bloque 5. Acceso, secretos y estado (2 min)

```bash
make gcloud ARGS="run services get-iam-policy mooc-insignias --region us-central1"
```

Solo `mooc-eventarc` puede invocar la función. Decir:

- **La función tiene su propia cuenta** (`mooc-insignias`). Lee `database-url`
  y escribe en el bucket `badges`. Nada más.
- **La compilación usa una cuenta propia**, no la de Compute Engine por
  omisión, que tiene permisos de editor.
- **Los secretos los genera Terraform** y los escribe directo en Secret
  Manager. No aparecen en el plan, en el estado ni en el repositorio.
- **El estado vive en un bucket con versionado**, no en el repositorio.
- Nadie creó claves JSON de cuentas de servicio.

```bash
make gcloud ARGS="secrets list --format='value(name)'"
make gcloud ARGS="storage ls gs://mooc-tfstate-mooc-509602/"
```

## Bloque 6. Cierre (1 min)

Con `docs/entrega5/evidencias/README.md`:

- El despliegue del 21 de noviembre, con su plan final sin cambios.
- La insignia y la reentrega en la nube, y la misma prueba en local con
  `make prueba-funcion`.
- Lo que encontró el primer despliegue y se corrigió, por ejemplo que Cloud Run
  functions exige un punto en la ruta del módulo de Go. Está en la sección
  «Hallazgos del primer despliegue» de `arquitectura/alcance-entrega-5.md`.

---

## Plan B

| Si falla | Qué hacer |
| --- | --- |
| El plan del encendido no dice `2 to add, 5 to change, 0 to destroy` | No aplicar. Avisar al equipo |
| El plan del bloque 3 muestra cambios antes de tocar nada | Decirlo, y enseñar `docs/entrega5/evidencias/terraform-plan-final.txt` |
| El `apply` del bloque 3 tarda | Seguir con la sección 3 de la guía mientras termina |
| El estudiante no llega a «Objetivo alcanzado» | Revisar que se quedó 20 s en la lectura. Si no, usar la traza de `docs/entrega5/evidencias/tarea-y-reentrega.tsv` |
| La consulta de registros no devuelve nada | Los registros tardan unos segundos. Repetirla |
| `make gcloud` pide iniciar sesión | `make gcloud ARGS="auth login"`, fuera de cámara |
| Se acaba el tiempo | Recortar el bloque 1 a la sección 5. Nunca recortar el 3 ni el 4 |

## Después de grabar

1. **Devolver el tiempo máximo de la función a 60 y apagar el entorno.** En
   `terraform.tfvars`:
   - `tiempo_maximo_s = 60`;
   - `cloudsql_encendida = false`;
   - `entorno_encendido = false`.

   El plan debe decir `0 to add, 6 to change, 2 to destroy`. Lo que destruye
   son la NAT y su router.

   ```bash
   make tf ENTORNO=entrega5 ARGS="plan -out=p.tfplan"
   make tf ENTORNO=entrega5 ARGS="apply p.tfplan"
   ```

2. Subir el video con enlace accesible para el evaluador y ponerlo en la fila 8
   de la tabla de la Entrega 5 del `README.md`, donde dice «Pendiente».
3. Commit en `main` y el tag:

   ```bash
   git tag -a entrega-5 -m "Entrega 5: funciones serverless e infraestructura como código"
   git push origin main
   git push origin entrega-5
   ```
