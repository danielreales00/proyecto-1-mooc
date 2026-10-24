# [Entrega 3] Prueba del despliegue elástico en GCP

Plan para encender por primera vez el entorno `infra/environments/entrega3` y
dejar demostrado el componente **arquitectura elástica**: plantillas e
inicialización, dos grupos de 1 a 3 con su autoscaler, balanceador HTTPS con
verificaciones de salud y la cola fuera de los grupos. Gasta lo mínimo del
presupuesto de 50 USD.

Fuera de este plan: frontend y su matriz de verificación, pruebas de
integridad entre réplicas, corridas medidas de capacidad, SonarQube y deuda
técnica.

El procedimiento de referencia es `arquitectura/despliegue-gcp-entrega3.md`
(fases 0 a 4). Los requisitos salen de `arquitectura/enunciado-entrega-3.md`,
sección «Modelo de despliegue elástico en la nube pública». Si este plan no
coincide con el runbook, manda el runbook y se corrige este archivo.

## Punto de partida

- El entorno `entrega3` **nunca se ha encendido**. Las fases 1 a 4 del runbook
  están sin ejecutar.
- `terraform.tfvars` está todo apagado: `entorno_encendido`,
  `cloudsql_encendida`, `autoescalado` y `generador_encendido` en `false`.
- `version_imagenes = "86b31a8f89a0"` (las de `main`, con el frontend) e `imagen_grupos = "debian-cloud/debian-12"`
  (sin imagen horneada).
- `inicializacion_api_s = 180` e `inicializacion_workers_s = 240` son
  provisionales hasta medir el arranque con la imagen horneada.
- La cabecera `X-Instancia` ya está en `deploy/Caddyfile.produccion`: es el
  identificador de instancia para las evidencias del balanceador.

## Lo que hay que demostrar

Cada fila sale del enunciado y dice en qué paso se obtiene la evidencia.

| # | Requisito del enunciado | Evidencia | Paso |
| --- | --- | --- | --- |
| E1 | Una instancia nueva queda lista sin acceso manual | Diario de arranque y `/readyz` en 200 sin haber entrado a la máquina | A2, B3 |
| E2 | Un reinicio no duplica procesos ni recursos | `docker ps` y temporizadores tras reiniciar la máquina | A4 |
| E3 | Migraciones controladas, sin que cada réplica las aplique a la vez | Diario de `migrate` en cada API nueva | B3, C3 |
| E4 | Tiempo de arranque registrado y criterio de «lista» explicado | Tiempos de `creationTimestamp` a `/readyz` o primer trabajo | B3 |
| E5 | Misma versión de código y configuración en un grupo y corrida | `make instancias` con la misma versión en todas | C3, C4 |
| E6 | Dos grupos de 1 a 3, cada uno con su política documentada | `describe` de los dos autoscalers | C2 |
| E7 | Cada grupo crece solo al menos una vez | Línea de tiempo del tablero y `describe` con hora | C3, C4 |
| E8 | Cada grupo decrece solo al bajar la demanda | Ídem | C3, C4 |
| E9 | La API nueva se incorpora al balanceador | Backend sano en el servicio de backend y `X-Instancia` nuevo | C3 |
| E10 | El worker nuevo empieza a consumir la cola compartida | Diario del `worker-media` nuevo con un trabajo tomado | C4 |
| E11 | Comportamiento al llegar al máximo | Grupo de workers en 3 con cinco videos | C4 |
| E12 | HTTPS en el balanceador, 80 redirige | `curl` contra 80 y 443 | A3 |
| E13 | El balanceador reparte entre al menos dos instancias sanas | Conteo de `X-Instancia` sobre muchas peticiones | C3 |
| E14 | Deja de enviar a una instancia que no pasa la verificación | Instancia fuera del reparto tras romper su `/readyz` | C3 |
| E15 | Exclusión por el balanceador distinta de la reparación por el grupo | Hora de la exclusión frente a hora de la recreación | C3 |
| E16 | La cola no decrece con trabajos en curso | El grupo no encoge mientras `worker-media` transcodifica | C4 |

## Prerrequisitos de otros frentes

Este plan no los resuelve. Solo dice qué sesión espera a cada uno.

| # | Prerrequisito | Dueño | Por qué lo necesita este plan | Bloquea | Estado |
| --- | --- | --- | --- | --- | --- |
| P1 | Pools de conexiones repartidos para 3 + 3 (alcance, punto 3) | Backend | Con los valores de la Entrega 2, 3 + 3 piden 135 conexiones contra unas 47 de `db-g1-small`: la tercera instancia falla | Sesión C | Cerrado: `db_max_conns_api = 8`, `db_max_conns = 3`, 3 × 8 + 3 × 2 × 3 = 42 |
| P2 | Reaper corregido (alcance, punto 4) | Backend | Republicaba trabajos que ya esperan e inflaba `cola_bulk_trabajos`: el grupo de workers crecería por trabajos que no existen y no encogería | Ciclo de workers, C4 | Cerrado: `job_key` como ID de la tarea en asynq |
| P3 | Retiro seguro de workers (alcance, punto 5) | Backend | Al encoger el grupo, la transcodificación en curso se cortaba a los 8 s | Ciclo de workers, C4 | Cerrado y comprobado en C4: el grupo retiró un worker ocupado y la transcodificación terminó en el intento 1, 89 s después, antes del apagado. El puerto serie no se pudo leer; la evidencia es `platform.job_runs` |

Las imágenes con P2 y P3 salen de un commit posterior a `86b31a8f89a0`: antes
de la Sesión C, `make publicar`, `version_imagenes` y `make hornear-imagen`
(Sesión B, paso 1).

## Reglas de operación

1. **Avisar al equipo antes de cada `apply`.** El estado está en
   `gs://mooc-tfstate-mooc-509602` con bloqueo compartido.
2. **El `terraform.tfvars` encendido no se versiona.** Al terminar,
   `git diff infra/environments/entrega3/terraform.tfvars` solo puede mostrar
   cambios acordados (imagen, tiempos de arranque, `autoescalado`), nunca los
   interruptores de encendido.
3. **Toda sesión termina con el apagado y sus comprobaciones**, aunque falle a
   mitad. Todo prendido un día cuesta de 6 a 10 USD.
4. **Revisar el gasto antes de encender.** Si el presupuesto supera el 80 %, no
   se enciende y se avisa al equipo.
5. **Nada de claves JSON de cuenta de servicio**, ni para probar
   (ADR-0015, D3).
6. Docker Desktop encendido, y `MSYS_NO_PATHCONV=1` delante de las órdenes que
   pasan rutas a contenedores desde Git Bash.
7. El generador de carga (`generador_encendido`) queda apagado: estas sesiones
   no son corridas medidas. La carga sale de k6 en la laptop.
8. Evidencia en `docs/entrega3/evidencias/elastica/`, con fecha y hora en el
   nombre, sin credenciales ni cookies de sesión.

## Costo estimado

Precios de lista aproximados en `us-central1`.

| Recurso | USD/h |
| --- | --- |
| `e2-highcpu-2`, cada instancia de API o worker | ~0,05 |
| `e2-small`, `mooc-cola` | ~0,017 |
| Cloud SQL `db-g1-small` | ~0,035 |
| Reglas de reenvío del balanceador (80 y 443) | ~0,025 |
| Cloud NAT y descargas de imágenes | ~0,01 + ~0,045/GB |

| Sesión | Configuración | Horas | USD |
| --- | --- | --- | --- |
| A, puesta en marcha | 1 + 1 | ~3 (la espera del certificado es la mayor parte) | ~1 |
| B, imagen y arranque | 1 + 1 | ~1,5 | ~0,5 |
| C, autoscaling | Hasta 3 + 3 | ~2,5 | ~2 |
| **Total** | | | **~3,5** |

Apagado quedan la IP reservada (~7 USD al mes), el disco de la cola y la imagen
horneada. Se anotan las horas reales de cada sesión: la estimación de costos
del alcance las pide.

---

## Sesión A — Puesta en marcha con 1 + 1 (fases 0, 1 y 2)

Se puede hacer ya: no depende de P1 a P3.

### A0. Verificaciones previas (local, sin costo)

1. Revisar el gasto del presupuesto en la consola de facturación (regla 4).
2. Confirmar que no hay nada encendido:
   - `make gcloud ARGS="compute instances list"`
   - `make gcloud ARGS="sql instances list"`
   - `make gcloud ARGS="compute forwarding-rules list"`
3. Estado de `entrega3`:
   `make gcloud ARGS="storage ls gs://mooc-tfstate-mooc-509602/entrega3/"`.
   - Si no existe, fase 1 del runbook, una sola vez, con `entrega2` detenido y
     nadie más aplicando:
     `make gcloud ARGS="storage cp gs://mooc-tfstate-mooc-509602/entrega2/default.tfstate gs://mooc-tfstate-mooc-509602/entrega3/default.tfstate"`.
4. `make tf ENTORNO=entrega3 ARGS=init` y `ARGS=plan`. El plan se lee entero.
   - Esperado en el primer `plan` tras la copia: destruye `module.maquinas`
     (mooc-web, mooc-worker), sus direcciones y la regla `mooc-web-publico`;
     crea la cola, los grupos, el balanceador y el tablero; cambia en sitio el
     CORS de los buckets.
   - **No puede** tocar `module.cloudsql`, `module.secretos`,
     `module.registry` ni el presupuesto, ni recrear ningún bucket. Si aparece,
     se para.
5. Confirmar que las imágenes `86b31a8f89a0` existen en Artifact Registry. Si se
   despliega otro commit, `make publicar` y actualizar `version_imagenes`.

### A1. Encendido

1. En `terraform.tfvars`: `cloudsql_encendida = true`,
   `entorno_encendido = true`. `autoescalado` sigue en `false`.
2. Avisar al equipo (regla 1). `make tf ENTORNO=entrega3 ARGS=apply`.
3. `make tf ENTORNO=entrega3 ARGS="output url_publica"` tiene que coincidir con
   `NUBE_HOST` del `Makefile` (`34-160-11-22.sslip.io`, versionado en la
   Sesión A junto con el `base_url` de Postman). La IP del balanceador está
   reservada y se conserva entre sesiones; si cambiara, se actualizan los dos.
4. Anotar la hora de encendido.

### A2. Mientras se aprovisiona el certificado (15 a 60 minutos)

El 443 responde con error de TLS hasta que el certificado está `ACTIVE`. No es
un fallo de las instancias.

1. `make instancias`: `mooc-cola`, una `mooc-api-*` y una `mooc-worker-*`.
2. **E1.** Sin entrar a configurar nada, leer el diario de arranque de cada
   instancia:
   `make ssh MAQUINA=<instancia> CMD="sudo journalctl -u google-startup-scripts | grep mooc:"`.
   Anotar `mooc: web levantado en N s` (o el equivalente del worker) y el
   `creationTimestamp`. Es la primera medida, con el arranque por script.
3. Publicador de la cola:
   `make ssh MAQUINA=mooc-cola CMD="sudo journalctl -u mooc-publicar-cola -n 5"`.
4. **Fase 0, comprobación 1:** la serie
   `custom.googleapis.com/mooc/cola_bulk_trabajos` aparece en el explorador de
   métricas. Sin ella, el autoscaler de workers no tiene señal.
5. Repetir `make gcloud ARGS="compute ssl-certificates list"` hasta `ACTIVE`
   (fase 0, comprobación 3).

### A3. Balanceador y grupos con una instancia

1. **E12.** `curl -sI http://<host>/healthz` devuelve 301 hacia `https://`, y
   `curl -sI https://<host>/healthz` devuelve 200 con `X-Instancia: mooc-api-…`.
2. `curl -s https://<host>/readyz`: 200. Comprobar que la respuesta no expone
   secretos ni direcciones internas.
3. El servicio de backend tiene un backend sano:
   `make gcloud ARGS="compute backend-services get-health <servicio> --global"`.
4. La API funciona detrás del balanceador: `make semilla-nube` y
   `make postman-nube`. **El código de salida tiene que ser 0.**
5. El grupo de workers consume la cola compartida: subir un video corto y ver
   `uploaded → scanning → clean → processing → ready`. **Uno a la vez**
   mientras P2 siga abierto.

### A4. Reinicio sin duplicados (E2)

1. Reiniciar la instancia de API y la de worker:
   `make gcloud ARGS="compute instances reset <instancia> --zone us-central1-a"`.
2. Cuando vuelvan, comprobar:
   - `make ssh MAQUINA=<instancia> CMD="sudo docker ps"`: un solo contenedor por
     servicio;
   - en `mooc-cola`, tras reiniciarla también, un solo temporizador
     `mooc-publicar-cola` y Redis con sus datos (AOF);
   - el balanceador vuelve a tener el backend sano sin intervención.

### A5. Si algo falla al encender

| Síntoma | Primer lugar donde mirar |
| --- | --- |
| La instancia de API se recrea en bucle | `retraso_autorreparacion_s` y el diario de arranque: `/healthz` no responde antes de 300 s |
| `apply` termina con `Failed to save state` | La subida del estado a GCS falló (DNS de Docker en WSL): queda `errored.tfstate`. **No repetir el `apply`** con el estado viejo; subir ese archivo con `state push`, o importar lo creado, antes de seguir |
| Un reinicio acaba en `repair.recreateInstance` | La verificación de vida del grupo (`vida`, 12 s × 10 fallidas): si se endurece, un reinicio normal cuenta como muerte |
| `postman-nube` falla con 422 en el registro | La cuenta no lee el secreto `seed-password` y la contraseña queda vacía: falta el permiso `integrantes_semilla` |
| `/readyz` falla y el balanceador no tiene backends sanos | Conexión a Cloud SQL (secreto `database-url`), a Redis en 10.10.0.30:6379 o a los buckets |
| El certificado sigue en `PROVISIONING` pasada una hora | Que el nombre `<ip>.sslip.io` resuelva a la IP del balanceador y que la regla del 80 exista |
| La cola no publica la métrica | Cuenta `mooc-cola` sin `monitoring.metricWriter`, o el temporizador de systemd |

Si en 30 minutos no se encuentra la causa, se apaga (A6) y se investiga sin
gastar.

### A6. Apagado (obligatorio)

1. `entorno_encendido = false` y `cloudsql_encendida = false`, y `apply`.
2. Repetir las tres listas de A0.2: ninguna instancia, Cloud SQL `STOPPED`,
   ninguna regla de reenvío.
3. `git diff infra/environments/entrega3/terraform.tfvars` sin los
   interruptores (regla 2).
4. Anotar las horas de uso.

**La Sesión A termina cuando** E1, E2 y E12 tienen evidencia, `postman-nube`
sale con 0 por el balanceador, un video llega a `ready` y las comprobaciones
del apagado salen vacías.

---

## Sesión B — Imagen horneada y tiempos de arranque (fase 3)

Antes de la Sesión C, y otra vez cada vez que cambie `version_imagenes`.

1. `make hornear-imagen` con todo apagado: solo enciende la máquina temporal
   del horno (~8 min) y la borra al terminar.
2. Poner el valor impreso en `imagen_grupos` y encender como en A1: las
   instancias nacen ya con la imagen horneada. Es la primera medida, desde el
   `creationTimestamp`.
3. **E1, E3 y E4.** Recrear la instancia de cada grupo tres veces seguidas con
   `make gcloud ARGS="compute instance-groups managed recreate-instances <grupo> --instances=<instancia> --zone us-central1-a"`.
   **La recreación es en sitio:** la instancia conserva nombre, `id` y
   `creationTimestamp`. El inicio se toma del fin de la operación `update` de la
   instancia en `compute operations list`. Para cada una:
   - tiempo desde ese inicio hasta `mooc: <rol> levantado en N s` en el diario
     de arranque (todos los contenedores sanos);
   - API: el diario de `migrate` dice `esquema al día` y no aplicó nada.
   - En la API, la operación `update` empieza ~90 s después de pedirla: el
     balanceador drena las conexiones antes. No cuenta en el arranque.
4. Llevar los tiempos a `inicializacion_api_s`, `inicializacion_workers_s` y
   `retraso_autorreparacion_s`, con margen sobre el peor medido. Anotar fecha,
   versión y los tres valores de cada grupo.
5. Apagar como en A6. Los tres valores y `imagen_grupos` sí se versionan.

**Termina cuando** E4 tiene, por grupo, la medida con arranque por script de
A2 y las tres con imagen horneada de B3, y los periodos de inicialización
están en `terraform.tfvars`.

---

## Sesión C — Ciclo de autoscaling de las dos capas (fase 4)

**Puerta:** P1 cerrado y la Sesión B terminada. P2 y P3 solo bloquean el ciclo
de workers (C4): si siguen abiertos, se hace C3 y C4 queda para otra sesión.

### C1. Encendido con autoscaling

1. Encender como en A1, con `autoescalado = true`, y `apply`.
2. Si el `apply` falla porque el autoscaler no acepta el filtro
   `resource.type = "global"`, es la fase 0, comprobación 1: parar y revisar el
   ADR-0019 (D9).

### C2. Políticas efectivas (E6)

Guardar la salida de:

- `make gcloud ARGS="compute instance-groups managed describe mooc-api --zone us-central1-a"`
- `make gcloud ARGS="compute instance-groups managed describe mooc-worker --zone us-central1-a"`

`gcloud` no tiene un grupo `compute autoscalers`: la política efectiva sale en
la sección `autoscaler` del `describe` de cada grupo.
- `make gcloud ARGS="compute backend-services describe <servicio> --global"`
- `make gcloud ARGS="compute health-checks list"`

Con eso se llena la tabla de políticas que pide el enunciado: señal, objetivo,
inicialización, estabilización, límites, y para el balanceador, puertos, modo
de reparto y parámetros de salud (`/readyz`, cada 5 s, timeout 5 s, 2 sanas, 3
fallidas).

### C3. Ciclo de la API y balanceador

1. Abrir el tablero `MOOC — ciclo de escalado (Entrega 3)` y anotar la hora de
   inicio.
2. **Crecimiento (E7, E9).** k6 desde la laptop (`docker run grafana/k6`), con
   un guion basado en `capacity-planning/k6/escenario1.js`, a unas 50
   peticiones por segundo contra la URL pública. Supera las ~32 por instancia
   que disparan el crecimiento (40 × 0,8). Esperado: una segunda instancia
   aparece sana en `get-health` y llegan respuestas con un `X-Instancia` nuevo.
   Anotar la hora de cada transición: decisión del autoscaler, instancia
   creada, backend sano.
3. **Migraciones y versión (E3, E5).** Diario de `migrate` de la API nueva, y
   `make instancias` con la misma versión en las dos.
4. **Reparto (E13).** Con la carga aún encendida, 200 peticiones a `/healthz`
   contando `X-Instancia`. Las dos instancias tienen que aparecer.
5. **Exclusión y reparación (E14, E15).** En una de las dos, detener la API:
   `make ssh MAQUINA=<api> CMD="cd /opt/mooc && sudo docker compose stop api"`.
   - Anotar cuándo `get-health` la marca `UNHEALTHY` y cuándo deja de aparecer
     en el conteo de `X-Instancia`: ~15 s según el ADR-0019.
   - Anotar cuándo el grupo la recrea por `/healthz` (autorreparación): ocurre
     después y por otra verificación. La diferencia entre las dos horas es la
     evidencia de E15.
   - Mientras tanto, el autoscaler puede crear una tercera instancia. Si lo
     hace, se registra.
6. **Reducción (E8).** Detener k6. Esperado: el grupo retira instancias pasados
   los 10 minutos de `ventana_reduccion_s`, hasta quedar en 1. Anotar la hora.
7. Documentar las solicitudes en curso al retirar una instancia: k6 con carga
   baja durante la reducción, y registrar si hubo errores.

### C4. Ciclo de los workers

Solo con P2 y P3 cerrados.

1. Generar videos del perfil C con `capacity-planning/medios/generar.sh`, como
   hace `make carga-medios`, y subir cinco seguidos.
2. **Crecimiento y máximo (E7, E10, E11).** Esperado: `cola_bulk_trabajos` pasa
   de 2, el grupo crece hasta 3 (cinco trabajos piden ceil(5 / 2) = 3) y se
   queda ahí. El diario del `worker-media` de cada instancia nueva muestra un
   trabajo tomado. Anotar la hora de cada transición.
3. **Versión (E5).** `make instancias` con la misma versión en las tres.
4. **Reducción sin cortes (E8, E16).** Mientras quedan transcodificaciones en
   curso, el grupo no encoge aunque no haya trabajos pendientes. Cuando la
   señal baja a cero, encoge hasta 1. Ninguna transcodificación se corta, y
   cada video tiene un solo juego de derivados.

### C5. Vigilancia durante C3 y C4

- Conexiones de Cloud SQL en el tablero. Si se acercan a 47, P1 está mal
  calculado: apagar y avisar al frente de backend.
- Gasto: la sesión no pasa de 3 horas encendida.

### C6. Evidencia y apagado

1. Capturas del tablero que cubran los dos ciclos, y las salidas con hora de
   C2 a C4. Es la línea de tiempo que piden los escenarios 3A y 3B.
2. Apagar como en A6. `autoescalado = true` sí se versiona.

**La Sesión C termina cuando** E3 y E5 a E16 tienen evidencia, sin conexiones
agotadas ni derivados duplicados, y las comprobaciones del apagado salen
vacías.

---

## Lo que queda listo para después

- **Documento de arquitectura:** la tabla de políticas de C2 y las horas de C3
  y C4 alimentan `docs/entrega3/arquitectura.md`. Contrastar
  `docs/entrega3/vista-de-despliegue.md` con lo observado.
- **Corridas medidas** (escenarios 1, 2, 3A y 3B): empiezan cuando este plan
  está terminado. Usan el generador en la nube, no la laptop.
- **Reconstrucción para la sustentación:** el orden del runbook (init y `apply`
  sin autoscaling, certificado, semilla, imagen, autoscaling) es el de estas
  sesiones; ensayarlo una vez antes del video.

## Registro de sesiones

| Fecha | Sesión | Quién | Horas encendido | Resultado | Evidencia |
| --- | --- | --- | --- | --- | --- |
| 2026-10-23 | A, tramo 1 | Michael | 04:30–06:03 UTC (~1,5) | Encendido, certificado `ACTIVE`, E1 y E12. Apagado al cerrar el día | [`sesion-a`](../docs/entrega3/evidencias/elastica/2026-10-23-sesion-a.md) |
| 2026-10-23 | A, tramo 2 | Michael | 13:55–15:34 UTC (~1,7) | E2 tras corregir la verificación de vida, `postman-nube` en 0, video en `ready`. Sesión A cerrada, ~0,6 USD | [`sesion-a`](../docs/entrega3/evidencias/elastica/2026-10-23-sesion-a.md), [`postman-nube`](../docs/entrega3/evidencias/elastica/2026-10-23-postman-nube.log) |
| 2026-10-23 | B | Michael | 17:27–17:35 horno, 17:35–18:41 UTC (~1,2) | Imagen `mooc-base-86b31a8f89a0`; arranque horneado 53–80 s (API) y 70–100 s (workers); periodos 120 / 150 s y autorreparación a 300 s. Un `apply` interrumpido obligó a liberar el bloqueo e importar las reglas. ~0,4 USD | [`sesion-b`](../docs/entrega3/evidencias/elastica/2026-10-23-sesion-b.md) |
| 2026-10-24 | C | Michael | 20:59–21:05 horno, 21:14–22:28 UTC (~1,2) | Imágenes `5d24b4bc72e4` de `feat/entrega3-integracion` e imagen `mooc-base-5d24b4bc72e4`. E3 y E5 a E16 con evidencia: API 1 → 3 → 1 y workers 1 → 3 → 1, exclusión a los 14 s frente a reparación a los 111 s, retiro de un worker ocupado sin corte. Máximo 27 conexiones a Cloud SQL. ~0,4 USD. Sesión C cerrada | [`sesion-c`](../docs/entrega3/evidencias/elastica/2026-10-24-sesion-c.md) |
