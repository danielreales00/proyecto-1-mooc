# Alcance de la Entrega 5 — Funciones serverless e infraestructura como código

Fuente: `2026-20 Entrega 5 - Serverless y Terraform.pdf`, transcrito en
[`enunciado-entrega-5.md`](enunciado-entrega-5.md). Este documento es nuestra
lectura: qué pide cada requisito, cómo está hoy el repositorio frente a él, qué
decidimos y en qué orden se hace. Ante discrepancia, manda el PDF.

## Lo que ordena todo

> Migrar al menos una tarea del worker a una Cloud Function de segunda
> generación, activada mediante Cloud Pub/Sub, y automatizar el despliegue de
> la arquitectura resultante con Terraform.

Pesan 60 de los 100 puntos la migración (25) y Terraform (35). La arquitectura
de la Entrega 4 ya corre en Cloud Run con Pub/Sub y casi toda su
infraestructura ya está en Terraform, así que el trabajo se concentra en tres
frentes: la función, el enrutamiento de la tarea migrada fuera del worker y
cerrar lo que hoy todavía se hace a mano.

## Requisitos

Cada requisito tiene un ID para citarlo en commits, ADR y evidencias.

### Análisis

| ID | Requisito | Estado al 20 de noviembre |
| --- | --- | --- |
| E5-A1 | Identificar qué componentes ya son serverless | Falta el documento. Los datos están en `docs/entrega4/arquitectura.md` |
| E5-A2 | Documentar cada oportunidad con componente, servicio propuesto, beneficio y limitaciones | Falta |
| E5-A3 | Las alternativas adicionales solo se documentan, no se implementan | Regla de alcance |
| E5-A4 | Justificar la tarea elegida para migrar, comparada con las otras | Falta. Ver D1 |
| E5-A5 | Diagrama actualizado con el cambio implementado | Falta |

### Migración

| ID | Requisito | Estado |
| --- | --- | --- |
| E5-M1 | Una tarea del worker pasa a una Cloud Run function de segunda generación | Falta |
| E5-M2 | La función se activa mediante Pub/Sub, a través de Eventarc | Falta |
| E5-M3 | Se conservan el comportamiento y el resultado de la tarea | El manejador ya existe y se reutiliza. Ver D3 |
| E5-M4 | El worker deja de ejecutar la tarea migrada | Falta. Hoy los siete tipos llegan al worker por un solo topic |
| E5-M5 | Reintentos sin duplicar efectos | La idempotencia de `platform.job_runs` (ADR-0008) se reutiliza. Falta definir los reintentos de Eventarc |

### Terraform

| ID | Requisito | Estado |
| --- | --- | --- |
| E5-T1 | Cloud Run: servicios, imágenes, configuración y escalamiento | Hecho en `infra/environments/entrega4/cloudrun.tf` |
| E5-T2 | Pub/Sub: topics, suscripciones e integración con consumidores | Hecho en `pubsub.tf`. Falta el topic de la tarea migrada |
| E5-T3 | La función, su código desplegable y el disparador | Falta |
| E5-T4 | Cuentas de servicio, permisos, referencias a secretos y dependencias | Hecho para Cloud Run. Falta la identidad de la función y la del disparador |
| E5-T5 | Entorno parametrizado | Parcial. Proyecto, región y tamaños son variables. El bucket del estado y el prefijo `mooc` de los nombres están fijos |
| E5-T6 | Documentar prerrequisitos, preparación de imágenes y código, y manejo de recursos existentes | Parcial en `docs/entrega4/despliegue.md` |
| E5-T7 | Desplegar y actualizar solo con `terraform init`, `plan` y `apply` | Parcial. Ver «Lo que hoy se hace a mano» |
| E5-T8 | Sin credenciales, secretos ni estado en el repositorio | Hecho. Estado en GCS, secretos generados por Terraform en Secret Manager, `privado.auto.tfvars` fuera del repositorio |

### Entregables

| ID | Entregable | Ubicación prevista |
| --- | --- | --- |
| E5-D1 | Análisis y diagrama | `docs/entrega5/analisis-serverless.md` y `docs/entrega5/arquitectura.md` |
| E5-D2 | Código de la función y Terraform, con instrucciones reproducibles | `backend/` y `infra/environments/entrega5/`, guía en `docs/entrega5/despliegue.md` |
| E5-D3 | Evidencias del despliegue con Terraform | `docs/entrega5/evidencias/` |
| E5-D4 | Evidencia de una tarea real de Pub/Sub a la función y su resultado | `docs/entrega5/evidencias/` |
| E5-D5 | Video de hasta 20 minutos: explicar la migración, desplegar con Terraform y ejecutar una tarea por Pub/Sub y la función | Enlazado desde el README |
| E5-D6 | Versión evaluada | Tag `entrega-5` |

### Rúbrica

| Criterio | Puntos | Requisitos |
| --- | --- | --- |
| Análisis serverless | 15 | E5-A1 a E5-A5 |
| Migración a la función | 25 | E5-M1 a E5-M5 |
| Automatización con Terraform | 35 | E5-T1 a E5-T7 |
| Configuración y acceso | 15 | E5-T4, E5-T5, E5-T8 |
| Documentación y verificación | 10 | E5-D1 a E5-D6 |

## Lo que hoy se hace a mano

Es lo que puede costar puntos en E5-T7 («requiere configuración manual de
recursos»).

| Paso manual | Es configuración de un recurso del alcance | Tratamiento propuesto |
| --- | --- | --- |
| Crear el proyecto y enlazar la facturación | No, es prerrequisito | Se documenta |
| Crear el bucket del estado | No, Terraform no puede crear su propio backend | Se documenta, con su orden exacta |
| `make publicar`: construir y subir imágenes | No, es preparación de imágenes, que el enunciado pide documentar | Se documenta. Terraform fija qué imagen corre cada servicio |
| `gcloud run jobs execute mooc-migrate` y `mooc-seed` | Es operación, no configuración, pero rompe el «solo init, plan y apply» | Investigar que Terraform dispare la ejecución. Ver P4 |
| Copiar el estado de la entrega anterior | No, es manejo de recursos existentes | Se documenta |

## Decisiones propuestas

Registradas en [`adr/0021`](adr/0021-insignias-en-una-cloud-run-function.md), en estado «Propuesto» hasta revisarlas.

| # | Decisión | Por qué |
| --- | --- | --- |
| D1 | Migrar **`badge.issue`** | Usa Cloud SQL (la insignia) y Cloud Storage (su imagen), como el diagrama del enunciado. Dura menos de un segundo. Su idempotencia es doble: el reclamo de `job_runs` y una restricción única por inscripción. Su efecto duplicado sería visible (dos insignias, invariante 5), así que demuestra bien los reintentos |
| D2 | Un **topic propio** para la tarea migrada, `mooc-insignias`. El backend enruta por tipo: `badge.issue` va a ese topic y el resto sigue en `mooc-trabajos`. El worker quita `badge.issue` de sus manejadores | Un disparador de Eventarc recibe todo lo que llega a su topic. Filtrar por atributo en la función desperdiciaría invocaciones y dejaría la tarea en dos consumidores |
| D3 | La función reutiliza el `queue.Runner` y el `badges.Service` existentes. Solo cambia la entrada: un CloudEvent en vez de un push HTTP | Mismo comportamiento y resultado (E5-M3) sin duplicar la lógica |
| D4 | El código de la función es un paquete en la raíz del módulo `backend/`. Terraform lo empaqueta con `archive_file`, lo sube a un bucket con el hash en el nombre y despliega la función | Terraform gestiona el código desplegable (E5-T3). Un cambio del código cambia el hash y `apply` redespliega la función |
| D5 | La función escribe en la base por IP privada, con **salida directa a la VPC** en la misma subred que Cloud Run. Sin conector | Cloud SQL solo tiene IP privada. El proveedor admite `direct_vpc_network_interface` en la función (P2), y la salida directa no cobra por hora como el conector |
| D6 | Entorno de Terraform nuevo, `infra/environments/entrega5`, que hereda el estado de `entrega4`. `entrega4` queda congelado | El mismo patrón de las Entregas 3 y 4 |
| D7 | Nueva dependencia `functions-framework-go`, registrada en el ADR | La exige el runtime de Go de Cloud Run functions para registrar la función |
| D8 | Runtime `go126` | Coincide con `backend/go.mod` (P1) |
| D9 | Reintentos con `RETRY_POLICY_RETRY` en el disparador. La función cuenta los intentos con `job_runs.attempt`, que sube en cada reclamo: al quinto fallo marca el trabajo `dead` y confirma el evento para cortar los reintentos | Eventarc reintenta hasta 7 días y el evento no trae el número de entrega (P3). Contarlo en la base da el mismo límite de cinco que tienen los workers |

### Alternativas descartadas para la migración

| Tarea | Por qué no |
| --- | --- |
| `media.transcode_hls` | Hasta 437 s con 4 vCPU y FFmpeg. Cabe en una función, pero no aporta nada frente a Cloud Run y su candidato natural es Transcoder API, que se documenta como oportunidad |
| `media.scan` | Necesita `clamd` con más de 1 GiB de firmas cargadas. Una función arrancaría en frío en cada escalado |
| `media.probe` | Necesita `ffprobe`, que el runtime de Go no trae |
| `email.send` | Necesita llegar a Mailpit en `mooc-cola`. Funciona con la misma salida a la VPC, pero su resultado no se ve en Cloud SQL ni en Cloud Storage |
| `progress.recompute` | Solo toca la base. Es una alternativa válida, pero su efecto duplicado no se nota, así que demuestra peor los reintentos |

## Preguntas abiertas

Respondidas el 20 de noviembre con la documentación oficial. P4 se confirmó en
el despliegue del 21 de noviembre. P5 queda por medir en la factura.

| # | Pregunta | Respuesta | Fuente |
| --- | --- | --- | --- |
| P1 | ¿Hay runtime de Go 1.26 en Cloud Run functions? | Sí, `go126`, con retiro previsto para febrero o marzo de 2027. También existe `go127` | `cloud.google.com/functions/docs/runtime-support` |
| P2 | ¿Salida directa a la VPC o conector? | Salida directa: `service_config.direct_vpc_network_interface` y `direct_vpc_egress`, que por omisión solo saca los rangos privados | Documentación de `google_cloudfunctions2_function` |
| P3 | ¿Cómo limita Eventarc los reintentos? | El reintento se configura en el disparador y, por omisión, dura hasta 7 días con espera exponencial. La documentación no dice que el evento traiga el número de entrega. Se resuelve con D9 | `cloud.google.com/functions/docs/bestpractices/retries` |
| P4 | ¿Puede Terraform ejecutar los trabajos de migración y semilla? | Sí, con `run_execution_token` en `google_cloud_run_v2_job`: un valor nuevo crea una ejecución en el `apply` y espera a que termine. El proveedor estable `7.46` lo acepta, y el despliegue del 21 de noviembre corrió las migraciones así | Documentación de `google_cloud_run_v2_job` y el `apply` |
| P5 | ¿Cuánto cuesta la función? | Cobra por invocación y por tiempo de CPU, sin mínimo con 0 instancias. Sin conector no hay costo fijo nuevo. Se mide en la fase 3 | Pendiente de medir |

## Fases

| Fase | Contenido | Se verifica con | Costo |
| --- | --- | --- | --- |
| F0 | Responder P1 a P5 y escribir el ADR-0021. Hecho, salvo P4 y P5, que se confirman en F2 y F3 | El ADR revisado | Ninguno |
| F1 | Código: la función con su prueba, el enrutamiento por tipo en el publicador, `badge.issue` fuera del worker. **Hecho el 20 de noviembre** | `make ci` y `make prueba-funcion`, en verde | Ninguno |
| F2 | Terraform: entorno `entrega5`, topic, función, disparador, identidades, permisos y lo manual que se pueda automatizar. **Hecho el 21 de noviembre** | `plan` revisado, sin tocar datos | Ninguno hasta aplicar |
| F3 | Despliegue con `init`, `plan` y `apply`, y una tarea real de punta a punta. **Hecho el 21 de noviembre**: plan final sin cambios, insignia emitida por la función y reentrega sin duplicado, en `docs/entrega5/evidencias/` | Una insignia emitida por la función y una reentrega sin duplicado | Por uso |
| F4 | Análisis serverless, diagrama, guía, evidencias y guion | Revisión contra la rúbrica | Ninguno |
| F5 | Video, enlace en el README, tag `entrega-5` y apagado | El enlace y el tag | Por uso |

## Hallazgos del primer despliegue

Salieron al aplicar el 21 de noviembre y ya están corregidos en
`infra/environments/entrega5` y en el código.

| Hallazgo | Corrección |
| --- | --- |
| El buildpack de Go de Cloud Run functions exige un punto en el primer elemento de la ruta del módulo | El módulo pasa de `mooc/backend` a `mooc.local/backend` |
| La compilación lee el código de un bucket de Google, `gcf-v2-sources-*`, que solo existe tras la primera función | Permiso de lectura en el proyecto, limitado por condición a esos buckets |
| `migrations` y `openapi` son paquetes de Go, no solo archivos | El zip de la función los incluye |
| Con Cloud SQL encendiéndose en el mismo `apply`, el backend y los workers fallaban la sonda de arranque | Los servicios y los trabajos dependen de `module.cloudsql` |
| Una revisión fallida no se reintenta y Terraform no ve diferencias | Etiqueta `version` en la plantilla de cada servicio |
| El número de proyecto se leía durante el `apply` por depender de APIs nuevas, y cuatro permisos se reemplazaban | Las APIs nuevas se aplican primero, con `-target`. Se documenta en la guía |
