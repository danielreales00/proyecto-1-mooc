# Deuda técnica. Línea base de la Entrega 3

Santiago Chica Castano, Josue Briceño Urquijo, Daniel Alfredo Reales Paba y
Michael Javier Patiño Pantoja.

Este documento es el primer inventario formal de la deuda técnica de la
plataforma. Registra los compromisos técnicos presentes en el código y en la
solución desplegada, sus consecuencias y el plan para atenderlos. El reporte
del commit final de la Entrega 3 y este inventario forman la línea base contra
la que se medirán las entregas siguientes.

## 1. Alcance y método del diagnóstico

### Componentes revisados

| Componente | Ubicación | Líneas de código |
| --- | --- | --- |
| API y workers en Go | `backend/` | 11.762 |
| Interfaz web | `frontend/` (sin `vendor/`) | 2.007 |
| Guiones de carga y métricas | `capacity-planning/k6`, `capacity-planning/metricas` | 1.062 |
| Infraestructura como código | `infra/` (Terraform) | 1.079 |
| Contenedores y arranque | `Dockerfile`, `docker-compose.yml`, `deploy/` | 65 de Docker más YAML y shell |
| Scripts de operación | `scripts/` | 208 |

El total analizado es de 16.157 líneas. Go aporta el 73 %.

### Herramientas

| Herramienta | Versión | Para qué |
| --- | --- | --- |
| SonarQube Community Build | 26.10.0.133078 | Duplicación, mantenibilidad, fiabilidad, seguridad y deuda estimada |
| SonarScanner CLI | 8.1.0.6389 | Envía el código y la cobertura al servidor |
| `go test -coverprofile` | Go 1.26.10 | Cobertura de las pruebas unitarias de Go |
| `node --test` con cobertura lcov | Node 24 | Cobertura del cliente HTTP de la interfaz |
| `scripts/calidad.py` | Del repositorio | Exporta el análisis y verifica los umbrales |
| `make arch`, `make invariantes`, `make demo`, `make sec` | Del repositorio | Reglas de arquitectura, invariantes de dominio, recorrido de punta a punta, `govulncheck` y `gosec` |

La decisión de usar SonarQube Community como servicio efímero del CI, y no
SonarQube Cloud, está en el
[ADR-0018](../../arquitectura/adr/0018-analisis-de-calidad-con-sonarqube-en-el-ci.md).

### Configuración del análisis

- **Perfil de reglas.** «Sonar way comprehensive» en todos los lenguajes. El
  perfil de fábrica activa 6 de las 37 reglas de Go y con él el backend no
  tenía hallazgos. El perfil amplio subió el total de 24 a 156 hallazgos y la
  deuda estimada de 27 a 1.162 minutos. Los dos umbrales se cumplen con
  cualquiera de los dos perfiles.
- **Modo de severidades.** Clásico, con Blocker, Critical, Major, Minor e Info.
  No hace falta tabla de equivalencias.
- **Alcance.** Todo el repositorio. Las únicas exclusiones son código de
  terceros (`frontend/vendor`, con hls.js y js-sha256 minificados), las
  dependencias de desarrollo (`node_modules`) y las salidas de las pruebas de
  carga (`capacity-planning/resultados` y `graficas`). La razón de cada una
  está en `sonar-project.properties`.
- **Código de prueba.** Los `_test.go` y `frontend/tests` se declaran como
  pruebas. SonarQube los analiza con las reglas de pruebas y no cuentan en la
  cobertura.
- **Regla desactivada.** `go:S1135` busca la palabra «TODO» sin distinguir
  mayúsculas. Los comentarios están en español y marcaba 16 frases como «todo
  el trabajo va por esta conexión». Una búsqueda de `TODO`, `FIXME`, `XXX` y
  `HACK` en `backend/` y `frontend/` no encontró ninguno real.

### Actividades del equipo

1. Configurar el análisis y su automatización en local (`make calidad`) y en
   el CI (job «Calidad (SonarQube)»).
2. Ejecutar el análisis con el perfil de fábrica y con el amplio, y comparar.
3. Revisar los 117 hallazgos Major o superior contra el código, regla por
   regla y ubicación por ubicación.
   Cada uno se clasificó como deuda real, falso positivo o decisión de diseño.
4. Revisar a mano lo que el analizador no ve. Se contrastó el código con las
   mediciones de la Entrega 2 y con los requisitos de la Entrega 3: reintentos
   del `reaper`, apagado de los workers, presupuesto de conexiones a la base,
   comprobaciones de salud, firma de URL, Redis, trabajos programados, pruebas,
   respaldo y versiones de imágenes.
5. Agrupar los hallazgos en 21 elementos de deuda, priorizarlos y asignarles
   tratamiento, responsable y plazo.

### Cómo se reproduce

```bash
make calidad    # SonarQube en http://localhost:9002, exportación en docs/entrega3/calidad/
```

El CI corre lo mismo en cada push y publica el artefacto `calidad-<sha>`.

## 2. Resumen de la medición inicial

Análisis del 21 de octubre de 2026 sobre el commit `ccf2452` más la
configuración del análisis. **DT-22 no salió del analizador sino de la sesión
de medición del 28 de octubre**: hay deuda que solo aparece ejecutando. Los archivos completos están en
[`calidad/`](calidad/): [`resumen.md`](calidad/resumen.md),
[`medidas.json`](calidad/medidas.json) y
[`hallazgos.json`](calidad/hallazgos.json).

| Indicador | Valor | Umbral del proyecto | Cumple |
| --- | --- | --- | --- |
| Duplicación de código | 1,6 % (325 líneas en 10 bloques) | Máximo 20 % | Sí |
| Mantenibilidad | A (ratio de deuda 0,2 %) | A, B o C | Sí |
| Deuda técnica estimada | 1.162 minutos, unos 2,4 días de 8 horas | Referencia | |

**Interpretación.** La deuda estimada es el tiempo que SonarQube calcula para
corregir los 135 *code smells*. Con un ratio de 0,2 % sobre el costo estimado
de escribir el código, la calificación es A con holgura. La cifra se reparte
de forma desigual. La interfaz acumula 616 minutos con el 12 % de las líneas, y
los guiones de carga 245 minutos con el 7 %. El backend, con el 73 % de las
líneas, suma 301 minutos.

La cifra no mide la deuda de arquitectura ni la de operación. Los elementos de
mayor impacto de este inventario (DT-01 a DT-04) no aparecen en SonarQube.

### Otros indicadores

| Indicador | Valor | Lectura |
| --- | --- | --- |
| Fiabilidad | C, con 1 bug | Una promesa sin esperar en la interfaz (DT-07) |
| Seguridad | E, con 20 vulnerabilidades | 11 son usos de `Math.random` en guiones de carga y 9 son reglas de Terraform. La calificación E viene de los dos Blocker de IP pública (DT-05) |
| Cobertura de pruebas unitarias | 9,5 % | Go 9,9 %, interfaz 18,2 %. No cuenta las pruebas de extremo a extremo (DT-09) |

### Hallazgos abiertos por severidad

| Severidad | Cantidad | Dónde |
| --- | --- | --- |
| Blocker | 2 | Terraform |
| Critical | 32 | 18 en el backend, 8 en guiones de carga, 6 en la interfaz |
| Major | 83 | 58 en la interfaz, 16 en guiones de carga, 5 en Terraform, 4 en el backend |
| Minor | 39 | |
| Info | 0 | |

## 3. Inventario de deuda identificada

Categorías: ARQ (arquitectura), COD (código), INF (infraestructura), OPE
(operación), PRU (pruebas) y SEG (seguridad).

Prioridades:

- **P1.** Pone en riesgo un requisito obligatorio de la Entrega 3 o la
  integridad de los datos. Se resuelve antes del cierre de la entrega.
- **P2.** Afecta la mantenibilidad, la seguridad o el rendimiento sin bloquear
  la entrega. Tiene plan con pasos y fecha.
- **P3.** Se acepta de forma temporal, con la razón registrada.

Los responsables se nombran por frente de trabajo, según el reparto de
[`alcance-entrega-3.md`](../../arquitectura/alcance-entrega-3.md).

| Frente | Integrante |
| --- | --- |
| Infraestructura elástica | Por asignar |
| Backend y calidad | Por asignar |
| Frontend | Por asignar |
| Capacidad | Por asignar |

### Prioridad 1

#### DT-01 · ARQ · El `reaper` duplica trabajos en la cola

- **Descripción y ubicación.** `backend/internal/adapters/queue/reaper.go:60-66`
  vuelve a publicar cada 30 s todo trabajo `queued` con más de 30 s de
  antigüedad. No comprueba si el trabajo sigue en Redis y `created_at` no
  cambia, así que un trabajo que espera su turno se publica otra vez en cada
  ciclo. `Publish` (`queue.go:64-79`) no usa `asynq.TaskID`, y asynq no
  descarta las copias.
- **Evidencia.** 2.554 duplicados descartados por la idempotencia en las
  corridas M2 y M3 de la Entrega 2
  ([informe de capacidad](../../capacity-planning/pruebas_de_carga_entrega2.md)).
- **Impacto.** El reclamo atómico (`jobs.TryClaim`) impide la doble ejecución,
  así que no hay efectos duplicados. El daño es otro. La profundidad de la cola
  `bulk` es la señal de escalado del grupo de workers, y las copias la inflan.
  El grupo crecería por trabajos que no existen y tardaría más en reducirse.
- **Prioridad.** P1. Distorsiona la señal del autoscaling obligatorio.
- **Tratamiento.** Publicar con `asynq.TaskID(job_id)` para que Redis rechace
  la copia, o republicar solo los trabajos que no estén en Redis. Prueba de
  integración que deje un trabajo `queued` tres ciclos y cuente las tareas.
- **Criterio de verificación.** En el escenario 3B la profundidad de `bulk`
  coincide con los `job_runs` en `queued`, y los logs no registran duplicados
  descartados.
- **Responsable y plazo.** Backend y calidad. Cierre de la Entrega 3.
- **Estado.** En tratamiento. Corregido en `692f30d` y `e0b38f2`, integrados en la fusión con `main`: `Publisher.Republicar` encola con `asynq.TaskID(job_key)`, no duplica una tarea que sigue viva en Redis y reactiva con `RunTask` la que está archivada en la DLQ, conservando su payload. El reaper no cuenta como recuperado un trabajo que sigue en Redis. `make prueba-cola` lo prueba contra Redis. La Sesión C aporta evidencia parcial en la nube: los nueve videos de la sesión terminaron con **un solo juego de derivados** (65 objetos y 4 manifiestos cada uno en `gs://mooc-509602-derived`), que es lo que un trabajo duplicado habría roto. Falta la verificación bajo carga del escenario 3B, que es donde el reaper tiene trabajos esperando turno de verdad.

#### DT-02 · OPE · El retiro de un worker corta el trabajo en curso

- **Descripción y ubicación.** `asynq.Config` en
  `backend/cmd/worker/main.go:124-134` no fija `ShutdownTimeout`, y asynq
  espera 8 s por defecto. Ni `docker-compose.yml` ni
  `deploy/compose.worker.yml` fijan `stop_grace_period`, que en Docker es de
  10 s. Una transcodificación larga duró 196 s en la Entrega 2.
- **Evidencia.** Código citado. El trabajo cortado vuelve a la cola cuando el
  `reaper` detecta 5 minutos sin latido (`reaper.go:47`).
- **Impacto.** Cada reducción del grupo de workers interrumpe las
  transcodificaciones en curso. El trabajo se recupera, pero tarda 5 minutos
  más y repite el procesamiento. El retiro seguro de workers es un requisito
  verificable de la entrega.
- **Prioridad.** P1.
- **Tratamiento.** Fijar `ShutdownTimeout` por encima de la transcodificación
  más larga medida, alinear `stop_grace_period` y el tiempo de apagado de la
  instancia del grupo, y dejar de aceptar trabajos al recibir la señal.
- **Criterio de verificación.** Prueba controlada del escenario 3. Se retira un
  worker con una transcodificación activa, el trabajo termina o se recupera, y
  existe un solo juego de derivados HLS.
- **Responsable y plazo.** Backend y calidad, con Infraestructura elástica.
  Cierre de la Entrega 3.
- **Estado.** En tratamiento. Corregido en `692f30d` y `e0b38f2`. El worker maneja SIGTERM con `Start` y no con `Run`, que apagaba el servidor por su cuenta y cortaba los trabajos sin esperar. Deja de tomar trabajos, espera `WORKER_SHUTDOWN_TIMEOUT` (8 s en local, 50 s en `worker` y 280 s en `worker-media` en la nube, con `stop_grace_period` de 15, 60 y 300 s) y corta lo que no termina para que el runner lo marque reclamable. Un trabajo en `running` con el latido vencido 90 s se reclama sin esperar al reaper.

  **Verificado en parte (Sesión C, E16).** Se retiró `mooc-worker-0w44` con una transcodificación activa: el `delete` empezó a las 22:14:19, el trabajo terminó a las 22:15:48 en el intento 1 y el `delete` acabó a las 22:16:06. El criterio se cumple —el trabajo terminó, sin reintento y con un solo juego de derivados—, pero **el plazo de 280 s no se puso a prueba**: la máquina dispuso de 107 s en total y al trabajo le bastaron 89. Lo que falta por saber es si GCE habría esperado los 280 s configurados o lo habría cortado antes, y eso solo lo contesta una transcodificación más larga que el margen real. Mientras no se mida, `WORKER_SHUTDOWN_TIMEOUT=280` es una intención, no una garantía. **Intentado en 3B el 28 de octubre, sin éxito.** Se subieron dos videos del perfil C a las 17:26 para que el autoscaler retirara una instancia ocupada. Las transcodificaciones terminaron a las 17:31:14 y 17:31:19 y la retirada fue a las 17:32:10: cincuenta segundos tarde. La causa es la propia defensa del sistema —la señal cuenta los trabajos en curso, así que el grupo no retira una instancia que está transcodificando—, lo que hace el caso peligroso estructuralmente raro y difícil de provocar. **Repetido con éxito el 28 de octubre a las 19:02**: se retiró `mooc-worker-1bs0` con dos transcodificaciones en vuelo (18:59:34→19:04:17 y 18:59:40→19:04:13) y las dos terminaron en el intento 1, sin reintento. El trabajo necesitó 88 s tras el inicio del `delete` y los tuvo. El criterio queda cumplido de forma deliberada;  lo que sigue sin medirse es el límite real de GCE. En las dos observaciones el trabajo necesitó ~88 s y el margen concedido fue ~107 s, así que **los 280 s configurados prometen más de lo que la plataforma parece dar**. Lo prudente es bajar el valor a lo observado y confiar en el reclamo por latido vencido para lo que no quepa.

#### DT-03 · ARQ · Las conexiones a la base no alcanzan para tres instancias

- **Descripción y ubicación.** `db-g1-small` admite unas 50 conexiones. Los
  pools son de 25 por API y de 10 por proceso de worker
  (`infra/environments/entrega2/terraform.tfvars:54-55` en el tag `entrega-2`). Con una instancia de
  cada grupo suman 45. Con tres de cada una piden 135. `migrate` abre hasta 10
  más al arrancar cada API. `WORKER_CONCURRENCY` vale 20 por defecto con un
  pool de 10 (`backend/internal/platform/config/config.go:158`).
  `infra/modules/cloudsql/main.tf` no fija `max_connections`.
- **Evidencia.** Configuración citada. El único control es un comentario en
  `terraform.tfvars:50`.
- **Impacto.** Al escalar, las instancias nuevas reciben errores de conexión
  justo cuando más capacidad hace falta. La prueba de autoscaling mediría el
  límite de la base y no el de la aplicación.
- **Prioridad.** P1.
- **Tratamiento.** Repartir los pools para el máximo de instancias, con la
  fórmula escrita en el documento de arquitectura. Bajar el pool de `migrate`
  a 1 y ajustar `WORKER_CONCURRENCY` al pool.
- **Criterio de verificación.** Con tres instancias de cada grupo,
  `pg_stat_activity` no supera el límite y los logs no registran errores de
  conexión.
- **Responsable y plazo.** Infraestructura elástica. Cierre de la Entrega 3.
- **Estado.** En tratamiento. Corregido en `692f30d`. Pools de 8 por API y 3 por proceso de worker: con tres instancias de cada grupo suman 42 de las 47 conexiones disponibles, y las 5 restantes cubren el `migrate` de una API que arranca y una consola. La fórmula está en `infra/environments/entrega3/terraform.tfvars`.

  **Verificado en parte (Sesión C, C5).** `pg_stat_activity` dio 16 conexiones con 3 APIs y 1 worker, y Cloud Monitoring un máximo de 27 con 3 + 3, frente a las 47 útiles. Sin errores de conexión. Pero los pools abren a demanda y la carga de la sesión era ligera: 27 no se acerca al techo de 42 porque nadie lo pidió. La comprobación que cierra el hallazgo es `pg_stat_activity` durante el nivel L3 del escenario 1, que es el único que llena el pool de la API.

#### DT-04 · SEG · `/readyz` expone errores internos y el worker no tiene comprobación de disponibilidad

- **Descripción y ubicación.** `backend/cmd/api/health.go:34-50` devuelve el
  texto del error de PostgreSQL, Redis o el almacén de objetos en el cuerpo de
  la respuesta. El error del driver de PostgreSQL puede incluir usuario, base y
  dirección. Caddy publica `/readyz` hacia Internet
  (`deploy/Caddyfile.produccion:29`). El worker solo tiene un `/healthz` que
  siempre responde 200 (`backend/cmd/worker/main.go:165-171`).
- **Evidencia.** Código citado.
- **Impacto.** `/readyz` será la comprobación de salud del balanceador. Cuando
  falle una dependencia, cualquiera podrá leer detalles de la red interna. En
  el grupo de workers, una instancia que perdió Redis seguiría marcada como
  sana.
- **Prioridad.** P1. La comprobación de salud no debe exponer información
  sensible y debe reflejar la capacidad real de atender.
- **Tratamiento.** Responder solo `ok` o `fail` por dependencia y registrar el
  detalle en el log. Agregar al worker una comprobación que haga ping a Redis y
  a PostgreSQL.
- **Criterio de verificación.** Prueba unitaria del handler con una dependencia
  caída que compruebe que el cuerpo no contiene el error. Con Redis detenido, el
  grupo de workers marca la instancia como no sana.
- **Responsable y plazo.** Backend y calidad. Cierre de la Entrega 3.
- **Estado.** En tratamiento. Corregido en `e0b38f2`: `httpx.Disponibilidad` responde `ok` o `fail` por dependencia y registra el error en el log, con prueba unitaria. El worker expone `/readyz` en el puerto 8081 con PostgreSQL y Redis. Falta que la comprobación de salud del grupo de workers apunte a ese `/readyz`.

#### DT-05 · INF · Máquinas con IP pública (Blocker `terraform:S6329`)

- **Descripción y ubicación.** `infra/modules/maquinas/main.tf:61` da IP
  externa al Web Server de la Entrega 2. `infra/modules/generador/main.tf:48`
  da IP externa al generador de carga.
- **Evidencia.** Regla `terraform:S6329`, severidad Blocker, 2 ubicaciones.
- **Impacto.** En el Web Server la IP pública era la entrada de la aplicación.
  En la Entrega 3 esa entrada pasa al balanceador, y una instancia de API con
  IP propia sería una puerta que lo esquiva. En el generador la IP evita el
  agotamiento de puertos de Cloud NAT, que en la Entrega 2 hizo perder 813
  conexiones. El firewall solo deja entrar SSH desde el rango de IAP
  (`generador/main.tf:75-86`) y la máquina usa OS Login.
- **Prioridad.** P1. Es un Blocker.
- **Tratamiento.** Plan en tres pasos.
  1. La plantilla de instancia de los grupos de API y workers se crea sin
     `access_config`. La salida a Internet va por Cloud NAT.
  2. El firewall de la API admite solo los rangos del balanceador y de sus
     comprobaciones de salud (`130.211.0.0/22` y `35.191.0.0/16`).
  3. Para el generador se acepta la IP pública mientras dure la medición, con la
     justificación anterior. Se destruye al terminar las pruebas.
- **Criterio de verificación.** El análisis del commit final no reporta
  `S6329` en el módulo de los grupos. `gcloud compute instances list` no
  muestra IP externa en ninguna instancia de los grupos.
- **Responsable y plazo.** Infraestructura elástica. Cierre de la Entrega 3.
- **Estado.** Resuelto. Las plantillas de los grupos no tienen IP externa
  (`692f30d`). `infra/environments/entrega2` y `infra/modules/maquinas` salen
  de `main` en `cfa7fe5`, porque la configuración de la línea base vive en el
  tag `entrega-2`. El generador se acepta como decisión de diseño (DT-20).

### Prioridad 2

#### DT-06 · COD · Funciones de dominio con complejidad cognitiva alta (`go:S3776`, Critical)

- **Descripción y ubicación.** 12 funciones superan el límite de 15. Las de
  mayor riesgo son reglas de negocio.

  | Función | Ubicación | Complejidad |
  | --- | --- | --- |
  | `ValidarPublicacion` | `backend/internal/modules/authoring/domain.go:276` | 56 |
  | Suite de contrato del almacén | `backend/internal/adapters/objectstore/contrato/contrato.go:39` | 49 |
  | `Verificar` (medios) | `backend/internal/modules/media/service.go:379` | 29 |
  | `main` de la semilla | `backend/cmd/seed/main.go:56` | 25 |
  | Prueba de sesión de reproducción | `backend/internal/modules/media/sesion_test.go:41` | 24 |
  | `UpsertQuiz` | `backend/internal/adapters/postgres/assessment.go:51` | 23 |
  | `Calificar` | `backend/internal/modules/assessment/domain.go:160` | 23 |
  | Otras cinco | `media/domain.go:166`, `postgres/authoring.go:206`, `assessment/http.go:75`, `learning/service.go:190` y `:296` | 18 a 20 |

- **Evidencia.** Regla `go:S3776`.
- **Impacto.** `ValidarPublicacion` decide si un curso se puede publicar y
  `Calificar` decide la nota de un intento. Las dos sostienen invariantes de los
  ADR 0007 y 0013. Con esa complejidad, un cambio de regla es difícil de revisar
  y fácil de romper.
- **Prioridad.** P2. Las dos tienen pruebas y hoy funcionan.
- **Tratamiento.** Partir `ValidarPublicacion` en una función por regla y
  `Calificar` en una por tipo de pregunta, con las pruebas de tabla actuales
  como red. La suite de contrato, la semilla y las pruebas se dejan como están
  porque no son código de producción.
- **Criterio de verificación.** `go:S3776` desaparece en los archivos de
  `modules/`. `make invariantes` y `make demo` siguen en verde.
- **Responsable y plazo.** Backend y calidad. Entrega 4.
- **Estado.** Resuelto en `885df7e`. Se partieron las doce funciones, incluidas la suite de contrato, la semilla y la prueba, porque el umbral se mide sobre el código total. `make calidad` no registra `go:S3776` en `backend/`, y `make ci` sigue en verde.

#### DT-07 · COD · La interfaz es un solo módulo de 1.157 líneas

- **Descripción y ubicación.** `frontend/app.js` contiene todas las vistas, el
  enrutador y el estado. Concentra 63 de los 156 hallazgos y 616 minutos de
  deuda estimada.
- **Evidencia.**
  - `javascript:S4624`, Major, 49 plantillas anidadas.
  - `javascript:S3776`, Critical, en `app.js:328`, `app.js:641` y `api.js:26`.
  - `javascript:S2004`, Critical, funciones anidadas a más de 5 niveles en
    `app.js:410` y `:418`.
  - `javascript:S3358`, Major, 6 ternarios anidados.
  - `javascript:S4123`, Critical, en `app.js:154`, y `javascript:S6544`, el
    único bug del análisis, en `app.js:319`.
  - Las pruebas unitarias cubren solo `api.js`.
- **Impacto.** El bug y el `await` señalado tienen la misma causa. `cleanup`
  empieza como función vacía y la vista de aprendizaje la reemplaza por una
  que devuelve una promesa. El enrutador la llama sin esperarla
  (`app.js:1066`). La evidencia de cierre de un recurso puede perderse al
  navegar, y el progreso del estudiante se calcula con evidencias. El resto
  hace lento revisar cualquier cambio de vista.
- **Prioridad.** P2. La interfaz está probada de punta a punta en el CI
  (`make frontend-e2e`).
- **Tratamiento.**
  1. Corregir `cleanup` para que siempre devuelva una promesa y esperarla en el
     enrutador. Es la causa del bug.
  2. Separar las vistas en módulos por rol.
  3. Extraer las plantillas anidadas a funciones.
- **Criterio de verificación.** `S6544` y `S4123` desaparecen. La prueba de
  navegador sigue en verde. Ningún archivo de la interfaz pasa de 400 líneas.
- **Responsable y plazo.** Frontend. El paso 1 al cierre de la Entrega 3 y los
  pasos 2 y 3 en la Entrega 4.
- **Estado.** En tratamiento. Los pasos 1 y 3 se hicieron en `0ca7a6a`:
  `cleanup` es asíncrona desde el principio, las plantillas anidadas pasaron a
  funciones y las vistas complejas se partieron. SonarQube no reporta
  hallazgos en `frontend/`. Queda el paso 2, separar las vistas en módulos,
  para la Entrega 4.

#### DT-08 · COD · Literales repetidos y firmas largas en Go

- **Descripción y ubicación.**
  - `go:S1192`, Critical, 6 ubicaciones. Tipos MIME en
    `media/domain.go:123-124`, el mensaje de sesión en
    `identity/http.go:168`, la cabecera `X-Request-Id` y el campo `petición` en
    `platform/httpx/middleware.go:64` y `:141`, y el código de error
    `resource.not_available` en `authoring/domain.go:322`.
  - `go:S107`, Major. `postgres/learning.go:273` recibe 9 parámetros.
  - `go:S978`, Major. Una variable `max` oculta la función de Go en
    `assessment/domain.go:250` y `badges/imagen.go:86`.
- **Evidencia.** Reglas citadas.
- **Impacto.** Un código de error del contrato escrito tres veces puede
  cambiar en un sitio y no en los otros, y el cliente recibiría códigos
  distintos para el mismo caso. El resto tiene impacto bajo.
- **Prioridad.** P2. Las correcciones son de pocos minutos.
- **Tratamiento.** Constantes para los literales, una estructura de parámetros
  para `learning.go:273` y nombres que no oculten funciones de Go.
- **Criterio de verificación.** Las tres reglas desaparecen del backend y
  `make rapido` sigue en verde.
- **Responsable y plazo.** Backend y calidad. Cierre de la Entrega 3.
- **Estado.** Resuelto en `885df7e`. `InsertEvent` recibe la evidencia completa. `make calidad` no registra hallazgos en `backend/`.

#### DT-09 · PRU · Cobertura unitaria baja en módulos con reglas de negocio

- **Descripción y ubicación.** La cobertura unitaria es de 9,5 %. Estos
  paquetes no tienen ningún `_test.go`.
  - `modules/learning`, 943 líneas, con el cálculo del progreso.
  - `modules/assessment`, 790 líneas, con los intentos.
  - `adapters/postgres`, 1.629 líneas.
  - `platform/jobs`, que contiene `TryClaim`, la base de la idempotencia de
    los workers.
  - `platform/idempotency`, con el middleware de `Idempotency-Key`.
- **Evidencia.** Medidas de cobertura por directorio en el análisis.
- **Impacto.** Estas reglas están cubiertas por `make demo`, `make
  invariantes`, la colección de Postman y la prueba de navegador, todas en el
  CI. Esa cobertura no se mide. Una regresión se detecta tarde y con un error
  de extremo a extremo difícil de localizar.
- **Prioridad.** P2.
- **Tratamiento.** Pruebas unitarias primero para `jobs.TryClaim` y para el
  middleware de idempotencia, que sostienen el invariante 6 del proyecto. Luego
  para el cálculo del progreso en `learning`.
- **Criterio de verificación.** Los cinco paquetes tienen pruebas. La cobertura
  de `modules/` pasa de 14,2 % a más de 40 %.
- **Responsable y plazo.** Backend y calidad. `jobs` e `idempotency` al cierre
  de la Entrega 3. `learning` y `assessment` en la Entrega 4.
- **Estado.** Pendiente.

#### DT-10 · ARQ · Cada URL firmada es una llamada a IAM, en serie

- **Descripción y ubicación.** `backend/internal/adapters/gcs/gcs.go:64-74`
  firma sin clave privada, con una llamada a IAM `signBlob` por URL. El
  playlist de cada variante firma sus segmentos uno por uno
  (`media/hls.go:139-153`). La carga multipart firma todas sus partes en serie
  (`media/service.go:191-206`), hasta 1.024.
- **Evidencia.** En la Entrega 2 cada firma tardó cerca de 200 ms y cada
  playlist cerca de 1 s.
- **Impacto.** Latencia del reproductor y de la autorización de cargas, y
  consumo de cuota de IAM que crece con los estudiantes.
- **Prioridad.** P2. Firmar sin clave descargada es una decisión de seguridad
  (ADR-0015, D3) que se mantiene.
- **Tratamiento.** Firmar en paralelo con un límite de concurrencia y
  reutilizar el playlist firmado mientras la firma siga vigente.
- **Criterio de verificación.** El p95 de la petición de playlist en el
  escenario 2 baja de 1 s.
- **Responsable y plazo.** Backend y calidad. Entrega 4.
- **Estado.** Pendiente.

#### DT-11 · SEG · Un solo Redis, sin contraseña, para cola, sesiones y límites

- **Descripción y ubicación.** `REDIS_ADDR` es único
  (`backend/internal/platform/config/config.go:124`) y separa usos por base
  lógica. En la nube, `deploy/compose.worker.yml` levanta Redis con AOF, sin
  `requirepass` ni política de memoria. Lo protege solo el firewall.
- **Evidencia.** Configuración citada.
- **Impacto.** Si Redis cae, se cierran todas las sesiones y se detiene la
  cola. Los trabajos sobreviven en PostgreSQL. Cualquier máquina de la subred
  puede leer las sesiones.
- **Prioridad.** P2. Es un punto único de falla aceptado por costo
  ([alcance](../../arquitectura/alcance-entrega-3.md), Memorystore cuesta casi
  tres veces más). La falta de contraseña no tiene esa justificación.
- **Tratamiento.** Contraseña de Redis en Secret Manager, leída por el
  arranque igual que la de la base. `maxmemory` con `noeviction` para que la
  cola no pierda trabajos.
- **Criterio de verificación.** `redis-cli` sin contraseña desde otra máquina de
  la subred recibe `NOAUTH`.
- **Responsable y plazo.** Infraestructura elástica. Cierre de la Entrega 3.
- **Estado.** Pendiente. El punto único de falla queda aceptado hasta la
  Entrega 5.

#### DT-12 · INF · Registros de infraestructura incompletos

- **Descripción y ubicación.**
  - `terraform:S6258`, Major. Cloud SQL sin `log_connections`
    (`infra/modules/cloudsql/main.tf:17`). Buckets sin registro de acceso
    (`gcs/main.tf:18`). Subred sin registros de flujo (`red/main.tf:11`).
  - `terraform:S6412`, Minor. Buckets sin versionado.
- **Evidencia.** Reglas citadas.
- **Impacto.** Ante un incidente no hay rastro de quién se conectó a la base ni
  de quién leyó un objeto. Sin versionado, un borrado accidental de un original
  no se recupera.
- **Prioridad.** P2. Los registros de flujo y de acceso tienen costo.
- **Tratamiento.** Activar `log_connections` en Cloud SQL, que no cuesta.
  Versionado con retención corta en el bucket de originales. Los registros de
  flujo se activan solo durante las pruebas de carga.
- **Criterio de verificación.** `S6258` desaparece en `cloudsql` y el bucket
  de originales tiene versionado.
- **Responsable y plazo.** Infraestructura elástica. Entrega 4.
- **Estado.** Resuelto en código en `cfa7fe5`: `log_connections` y
  `log_disconnections` en Cloud SQL, registros de flujo con muestreo de 0,1 en
  la subred, versionado con borrado de versiones viejas a 1 día y registros de
  acceso en un bucket `registros` que borra a los 30 días. Falta aplicarlo en
  `entrega3`.

#### DT-22 · COD · La sesión no se renueva con el uso, aunque el código diga que sí

- **Descripción y ubicación.** `backend/internal/adapters/sessions/redis.go:82-84`
  lleva el comentario «TTL deslizante: la actividad prolonga la sesión hasta el
  tope absoluto», pero lo único que hace al leer una sesión es escribir
  `last_seen_at`: **nunca llama a `Expire` para extenderla**. `Expire` solo
  aparece en `Create` (líneas 47 y 51). Y `SessionMaxTTL`
  (`config.go:83` y `158`, `SESSION_MAX_TTL=720h` en `.env.example`) **no se usa
  en ninguna parte del código**: se declara, se lee y se ignora.
- **Evidencia.** Encontrado midiendo, no leyendo. En la sesión de medición del
  28 de octubre, los niveles M2-2 y M3-1 del escenario 2 devolvieron 27 y 26
  respuestas `401` —«la sesión no existe, expiró o fue revocada»— y no subieron
  ni un archivo. Las sesiones se habían creado a la 01:25 y `SESSION_TTL` son 12
  horas: murieron a las 13:25, entre un nivel y el siguiente, pese a llevar toda
  la mañana en uso. `grep -rn "SessionMaxTTL" backend/` solo devuelve la
  declaración y la lectura.
- **Impacto.** Una persona que trabaje doce horas seguidas pierde la sesión sin
  aviso en mitad de lo que esté haciendo, aunque no haya dejado de usar la
  plataforma. El comportamiento documentado en `.env.example` promete lo
  contrario, así que nadie lo va a buscar ahí. Para las pruebas de carga
  significa que los datos preparados caducan y una campaña larga mide con
  sesiones muertas sin que nada falle de forma evidente: k6 sale con código 0 y
  las tablas salen vacías.
- **Prioridad.** P2. No hay pérdida de datos ni agujero de seguridad —si acaso
  el efecto es más conservador que lo prometido—, pero es una promesa incumplida
  en el camino más transitado de la aplicación.
- **Tratamiento.** Decidir cuál de las dos cosas es la correcta y dejar que el
  código y la documentación digan lo mismo. Si el TTL debe deslizarse, extender
  la expiración en la lectura con el tope de `SessionMaxTTL`. Si no, quitar
  `SessionMaxTTL` de la configuración y del `.env.example`, y corregir el
  comentario.
- **Criterio de verificación.** Una prueba que cree una sesión con un TTL corto,
  la use pasada la mitad de ese plazo y compruebe que sigue viva después del
  plazo original, sin pasar del tope absoluto.
- **Responsable y plazo.** Backend y calidad. Entrega 4.
- **Estado.** Pendiente.

### Prioridad 3

#### DT-13 · COD · Guiones de carga y de métricas

- **Descripción y ubicación.** `capacity-planning/` reúne 51 hallazgos.
  - `javascript:S2245`, Major, 11. `Math.random` en los guiones de k6.
  - `javascript:S3776` y `python:S3776`, Critical. `preparar.js:38` y tres
    funciones de `tabla.py`.
  - `python:S1192`, Critical. Encabezados de tabla repetidos.
  - `javascript:S1186`, Critical. Funciones vacías en `preparar.js:143` y
    `preparar-medios.js:59`.
  - `javascript:S8786`, `S7767`, `S6661` y `S3358`, Major. Una expresión
    regular con retroceso en `medios.js:97` y detalles de estilo.
- **Evidencia.** Reglas citadas.
- **Impacto.** `Math.random` elige qué curso o recurso visita cada usuario
  virtual. No protege nada, así que `S2245` no aplica. Las funciones vacías son
  `catch` que ignoran a propósito un error esperado. El resto es
  mantenibilidad de herramientas que no se despliegan.
- **Prioridad.** P3. Las pruebas de la Entrega 3 deben repetir los mismos
  guiones que la línea base de la Entrega 2. Cambiarlos ahora introduce una
  diferencia que habría que declarar y discutir en el informe de capacidad.
- **Tratamiento.** Después de cerrar las mediciones de la Entrega 3, refactorizar
  `tabla.py` y `preparar.js`, documentar los `catch` vacíos y simplificar la
  expresión regular.
- **Criterio de verificación.** Los Critical desaparecen de `capacity-planning/`.
- **Responsable y plazo.** Capacidad. Entrega 4.
- **Estado.** Resuelto en `e2087c9`. Los guiones conservan el recorrido, la
  mezcla, las pausas y las tasas de la Entrega 2. Cambia la fuente de
  aleatoriedad, de `Math.random` a la Web Crypto API, con la misma distribución
  uniforme. El informe de capacidad lo declara como diferencia. La salida de
  `tabla.py` es idéntica sobre los resultados de la Entrega 2.

#### DT-14 · OPE · Faltan trabajos programados

- **Descripción y ubicación.** No existen `assessment.expire_attempts` ni
  `media.sweep_uploads`. `progress.recompute` es una constante sin manejador
  (`backend/internal/platform/jobs/jobs.go:34`). El vencimiento de un intento se
  evalúa al leerlo o enviarlo (`assessment/service.go:158`), y el estado
  `expired` nunca se asigna.
- **Evidencia.** Código citado.
- **Impacto.** La calificación es correcta porque el vencimiento se comprueba
  al enviar. Los informes de intentos muestran como abiertos intentos vencidos.
  Las cargas abandonadas dejan partes en el almacén, con costo.
- **Prioridad.** P3.
- **Tratamiento.** Un programador de asynq en el worker, protegido con el
  bloqueo consultivo que ya usa la poda. Regla de ciclo de vida en el bucket
  para las cargas incompletas.
- **Criterio de verificación.** Un intento vencido aparece como `expired` sin que
  nadie lo lea.
- **Responsable y plazo.** Backend y calidad. Entrega 4.
- **Estado.** Pendiente.

#### DT-15 · OPE · No hay trazas distribuidas

- **Descripción y ubicación.** El `trace_id` es el identificador de la
  petición y viaja a los trabajos y a los logs
  (`backend/internal/platform/httpx/middleware.go:60`). No hay OpenTelemetry.
- **Evidencia.** `backend/go.mod` no tiene dependencias directas de OpenTelemetry.
- **Impacto.** Una petición lenta se puede seguir en los logs pero no
  descomponer en tiempo de base, de Redis y de firma.
- **Prioridad.** P3.
- **Tratamiento.** OpenTelemetry hacia Cloud Trace, reutilizando el `trace_id`
  actual. Requiere ADR por ser dependencia nueva.
- **Criterio de verificación.** Una petición de playlist muestra en Cloud Trace
  sus tramos de base y de firma.
- **Responsable y plazo.** Backend y calidad. Entrega 5.
- **Estado.** Pendiente.

#### DT-16 · OPE · El respaldo de la base nunca se ha restaurado

- **Descripción y ubicación.** Cloud SQL guarda 7 copias automáticas sin
  recuperación a un punto en el tiempo (`infra/modules/cloudsql/main.tf`). No
  hay script ni objetivo de `make` que restaure una copia.
- **Evidencia.** Configuración citada.
- **Impacto.** El tiempo de recuperación es desconocido.
- **Prioridad.** P3.
- **Tratamiento.** `make restore-test`, que restaura la última copia en una
  instancia temporal, corre `make invariantes` contra ella y mide el tiempo.
- **Criterio de verificación.** Tiempo de restauración medido y registrado.
- **Responsable y plazo.** Infraestructura elástica. Entrega 5.
- **Estado.** Pendiente.

#### DT-17 · INF · Imágenes de contenedor sin versión fija

- **Descripción y ubicación.** `docker-compose.yml` usa `:latest` en Mailpit,
  Swagger UI, Prometheus y Grafana. El CI y el `Makefile` usan
  `redocly/cli:latest`. PostgreSQL, Redis y Caddy van por versión mayor.
  `deploy/` sí fija versiones, pero ninguna imagen está fijada por digest.
- **Evidencia.** Archivos citados.
- **Impacto.** Una versión nueva de redocly puede poner el CI en rojo sin un
  cambio en el repositorio. Ya pasó con MinIO.
- **Prioridad.** P3. No afecta lo desplegado.
- **Tratamiento.** Versiones exactas en todas las imágenes.
- **Criterio de verificación.** `grep -rn ':latest'` en Compose, `Makefile` y
  CI no devuelve imágenes.
- **Responsable y plazo.** Backend y calidad. Entrega 4.
- **Estado.** Pendiente.

#### DT-18 · INF · MinIO local depende de una imagen archivada

- **Descripción y ubicación.** `docker-compose.yml:51` usa
  `bitnamilegacy/minio`, un catálogo congelado.
- **Evidencia.** Comentario de `docker-compose.yml:40-49` y la nota del 26 de
  septiembre en `arquitectura/pendientes.md`.
- **Impacto.** Si la imagen desaparece, el entorno local y el CI no arrancan.
  No afecta la nube, que usa Cloud Storage.
- **Prioridad.** P3.
- **Tratamiento.** Si vuelve a romperse, cambiar a un servidor S3 mantenido con
  `make ci` como juez, o copiar la imagen a Artifact Registry.
- **Criterio de verificación.** `make ci` en verde con la imagen nueva.
- **Responsable y plazo.** Backend y calidad. Cuando falle.
- **Estado.** Aceptado.

#### DT-19 · ARQ · Excepción de módulos entre `admin` e `identity`

- **Descripción y ubicación.** `scripts/arquitectura.sh:43` permite que
  `postgres/admin.go` escriba en el esquema de `identity` (4 escrituras). Hay
  además 34 lecturas entre esquemas de módulos.
- **Evidencia.** Salida de `make arch`.
- **Impacto.** Extraer `identity` como servicio aparte obligaría a reescribir
  esas consultas. El monolito modular no lo exige.
- **Prioridad.** P3.
- **Tratamiento.** Ninguno mientras el proyecto siga como monolito. `make arch`
  cuenta las lecturas para que el costo esté a la vista.
- **Responsable y plazo.** Backend y calidad. Sin plazo.
- **Estado.** Aceptado (ADR-0001, enmienda del 7 de septiembre).

#### DT-20 · SEG · Hallazgos revisados como decisión de diseño

| Regla | Ubicación | Decisión |
| --- | --- | --- |
| `terraform:S6404`, Major | `infra/modules/gcs/main.tf:67` | El bucket de insignias es de lectura pública porque la imagen de la insignia se verifica sin sesión. El rol `legacyObjectReader` no permite listar, así que no se pueden enumerar insignias ajenas |
| `terraform:S6329`, Blocker | `infra/modules/generador/main.tf:48` | El generador tiene IP externa porque por Cloud NAT perdió 813 conexiones. Existe solo mientras se mide, el firewall solo deja entrar SSH por IAP y usa OS Login |
| `terraform:S6405`, Major | `infra/modules/generador/main.tf:62` y `cola/main.tf:46` | Corregido en `cfa7fe5` con `block-project-ssh-keys`, también en las plantillas de los grupos |
| `docker:S6471`, Minor | `frontend/Dockerfile:1` | Corregido en `0ca7a6a`. Caddy corre como el usuario `web` en el puerto 8080 |
| `Web:S6819` y `css:S4666`, Major | `frontend/index.html:31`, `frontend/styles.css:699` | Corregidos en `0ca7a6a` |

- **Responsable y plazo.** Infraestructura elástica para Terraform y Frontend
  para el resto. Entrega 4.
- **Estado.** `S6329` en el generador y `S6404` en el bucket de insignias se
  aceptan. Se excluyen solo esas reglas en esos archivos, con la razón en
  `sonar-project.properties`. El resto está corregido.

#### DT-21 · OPE · El correo de producción no sale a Internet

- **Descripción y ubicación.** En la nube el SMTP es Mailpit
  (`infra/environments/entrega2/maquinas.tf:68-70` en el tag `entrega-2`).
- **Evidencia.** Configuración citada.
- **Impacto.** La verificación de cuenta solo funciona leyendo Mailpit por un
  túnel. Las cuentas de la demostración se crean con la semilla.
- **Prioridad.** P3. El registro por la interfaz no es parte de los recorridos
  mínimos de la entrega.
- **Tratamiento.** Proveedor SMTP con la credencial en Secret Manager.
- **Criterio de verificación.** Un registro en la nube recibe el correo en un
  buzón real.
- **Responsable y plazo.** Infraestructura elástica. Entrega 5.
- **Estado.** Aceptado.

## 4. Priorización y plan de tratamiento

El orden atiende primero lo que puede invalidar las pruebas de autoscaling,
después lo que expone información y por último lo que solo cuesta tiempo de
mantenimiento.

| Orden | Elemento | Acción | Responsable | Plazo |
| --- | --- | --- | --- | --- |
| 1 | DT-03 | Pools repartidos para tres instancias por grupo | Infraestructura elástica | Antes del escenario 3 |
| 2 | DT-01 | Publicación con `TaskID` | Backend y calidad | Antes del escenario 3 |
| 3 | DT-02 | Tiempo de gracia del worker y de la instancia | Backend y calidad | Antes del escenario 3 |
| 4 | DT-05 | Plantillas sin IP externa y firewall del balanceador | Infraestructura elástica | Con el Terraform de la Entrega 3 |
| 5 | DT-04 | `/readyz` sin detalle y comprobación del worker | Backend y calidad | Con el Terraform de la Entrega 3 |
| 6 | DT-11 | Contraseña de Redis | Infraestructura elástica | Cierre de la Entrega 3 |
| 7 | DT-08, DT-07 paso 1, DT-09 (`jobs` e `idempotency`) | Correcciones cortas y pruebas de idempotencia | Backend y calidad, Frontend | Cierre de la Entrega 3 |
| 8 | DT-06, DT-07, DT-09, DT-10, DT-12, DT-13, DT-14, DT-17, DT-20, **DT-22** | Refactorizaciones y pruebas | Según el elemento | Entrega 4 |
| 9 | DT-15, DT-16, DT-21 | Trazas, restauración y correo | Según el elemento | Entrega 5 |
| | DT-18, DT-19 | Sin acción | | Aceptados |

Los cinco elementos P1 son condiciones de los requisitos de la entrega. Su
registro aquí no los da por cumplidos, y la entrega no se cierra con ellos
pendientes.

## 5. Correcciones realizadas durante este ejercicio

| Corrección | Commit | Verificación |
| --- | --- | --- |
| Configuración del análisis, `make calidad` y job «Calidad (SonarQube)» en el CI | El de este documento | `make calidad` termina con los dos umbrales en `ok` |
| `scripts/calidad.py` se corrigió antes de la línea base. Tenía un literal repetido (`python:S1192`), un f-string sin campos (`python:S3457`) y una construcción de diccionario señalada por `S7494` y `S7500` | El de este documento | `scripts/` no tiene hallazgos en el análisis |
| Los 27 hallazgos del backend (DT-06 y DT-08): 12 `go:S3776`, 6 `go:S1192`, 2 `go:S107`, 2 `go:S978` y 5 de nombres (`go:S117`, `go:S100`) | `885df7e` | `make calidad`: 0 hallazgos en `backend/`, 129 en total y 861 min de deuda estimada. `make ci` en verde |
| Publicación idempotente en la cola (DT-01) | `e0b38f2` | `make prueba-cola`, en el CI |
| Retiro de workers: tiempo de gracia y reclamo de trabajos con el latido vencido (DT-02) | `e0b38f2` | `make ci`. La verificación final es el escenario 3 |
| `/readyz` sin detalle del error y `/readyz` en el worker (DT-04) | `e0b38f2` | Prueba unitaria `TestDisponibilidadNoExponeElError` |

Los 156 hallazgos de la sección 2 son la medición inicial, antes de cualquier
corrección. La línea base para las entregas siguientes es el análisis del
commit final (sección 6), con los elementos que sigan abiertos.

## 6. Línea base del commit final

Al cerrar la Entrega 3 se completa esta sección con:

1. El commit del tag `entrega-3`.
2. El enlace a la ejecución del job «Calidad (SonarQube)» de ese commit y su
   artefacto `calidad-<sha>`.
3. Las cifras de ese análisis. Si difieren de la sección 2, se explica qué
   elemento del inventario las cambió.
4. El estado final de cada elemento.

La exportación de `docs/entrega3/calidad/` se regenera con `make calidad` sobre
el árbol limpio de ese commit. Un cambio de versión de SonarQube, del scanner o
del perfil de reglas cambia las cifras sin que cambie el código, y se registra
como cambio de línea base.
