# Alcance de la Entrega 3 — Aplicación elástica IaaS

Fuente: `2026-20 Entrega 3 - Aplicación Elástica IaaS.pdf`, transcrito en
[`enunciado-entrega-3.md`](enunciado-entrega-3.md). Este documento es nuestra
lectura: qué entra, qué decidimos y qué trabajo queda. Ante discrepancia, manda
el PDF.

**Duración:** cuatro semanas. Los cuatro integrantes participan en
implementación, pruebas, análisis y sustentación.

## Lo que ordena todo

> No satisface el componente de arquitectura un grupo ampliado únicamente de
> forma manual ni una solución que escale la API pero mantenga fija la capa
> worker.

Hay dos grupos elásticos, de 1 a 3 instancias cada uno, y los dos tienen que
crecer y decrecer solos durante las pruebas, con evidencia. El resto de la
entrega (frontend en la nube, pruebas comparativas, SonarQube y deuda técnica)
se construye alrededor de eso.

## Qué se evalúa y cómo estamos

| Componente | Estado al 21 de octubre | Qué falta |
| --- | --- | --- |
| Arquitectura elástica | No empezado | Plantillas, dos grupos, balanceador HTTPS, autoscaling de las dos capas, Redis fuera de los grupos |
| Frontend funcional | Hecho y probado en local (ADR-0017) | Desplegarlo, probar los permisos negativos, matriz de recorridos con evidencia en la nube |
| Integridad y operación | Base de la Entrega 2 | Migraciones serializadas (hecho), retiro seguro de workers, reaper, reserva de conexiones |
| Pruebas comparativas | Guiones y línea base de la Entrega 2 | Escenarios 1 y 2 por el balanceador, escenario 3A y 3B, tabla comparativa |
| Calidad del código | Análisis en el CI y en local (ADR-0018) | Reporte del commit final |
| Documentación y deuda | No empezado | `docs/entrega3/arquitectura.md`, `deuda-tecnica.md`, verificación del frontend |
| Sustentación | No empezado | Video y tag `entrega-3` |

## Arquitectura objetivo

| Pieza | Decisión | Por qué |
| --- | --- | --- |
| Entrada | Balanceador de aplicaciones externo con HTTPS y certificado gestionado de Google sobre `<ip>.sslip.io` | El enunciado exige HTTPS en el balanceador. Con sslip.io no hace falta comprar dominio |
| Grupo de API | MIG zonal en `us-central1-a`, de 1 a 3 `e2-highcpu-2`. Cada instancia corre Caddy en el puerto 80, `api`, `frontend` y `swagger` | El frontend se queda junto a la API, como lo dejó el ADR-0017: mismo origen y sin CORS para la API. Su versión es la de la plantilla |
| Grupo de workers | MIG zonal de 1 a 3 `e2-highcpu-2`. Cada instancia corre `worker`, `worker-media` y ClamAV, con la misma concurrencia que en la Entrega 2 | La concurrencia por instancia queda comparable y el escalado es horizontal |
| Cola | VM dedicada `e2-small` con Redis 7.4 en contenedor y AOF. Mailpit va con ella | Fuera de los dos grupos. Unos 0,45 USD al día frente a 1,2 de Memorystore. Queda como punto único de falla documentado |
| Base y objetos | Cloud SQL `db-g1-small` y los cuatro buckets, sin cambios | Mismas condiciones que la línea base |
| Migraciones | `migrate` sigue en el arranque de cada API, serializado con un bloqueo consultivo | Hecho en `cmd/migrate`, probado con `make migraciones-concurrentes` |
| Calidad | SonarQube Community como servicio efímero del CI y con `make calidad` en local (ADR-0018) | La misma versión del analizador en local y en el pipeline, sin cuentas ni secretos externos |

### Señales de escalado propuestas

| Grupo | Señal | Por qué |
| --- | --- | --- |
| API | Utilización del balanceador, con un máximo de peticiones por segundo por instancia | En la Entrega 2 la CPU del Web Server no pasó del 60 % aunque la base estaba saturada. Un objetivo de CPU podría no disparar nunca. Las peticiones por instancia sí escalan con la demanda |
| Workers | Profundidad de la cola `bulk`, como métrica de grupo, con un número de trabajos pendientes por instancia | FFmpeg fue el cuello de botella del escenario 2, y la métrica ya llega a Cloud Monitoring por el Ops Agent |

Los valores concretos (peticiones por instancia, trabajos por instancia,
ventanas de estabilización) se fijan en un ADR nuevo a partir de las mediciones
de la Entrega 2.

## Trabajo de código que sí hay que hacer

1. **Migraciones serializadas.** Hecho: bloqueo consultivo y prueba de tres
   procesos simultáneos en `make ci` y en el CI.
2. **CI en verde.** Hecho: Go 1.26.10 fijado. El CI estaba en rojo desde el
   commit del frontend por GO-2026-6617.
3. **Reserva de conexiones a la base.** `db-g1-small` admite unas 50 conexiones.
   Con tres APIs a 25 y tres workers a 20 se pedirían 135. Hay que repartir
   los pools para el máximo de instancias, o subir `max_connections`, y
   documentarlo.
4. **Reaper.** Republica trabajos que siguen esperando en Redis. En la Entrega 2
   produjo 2.554 duplicados. Ahora además inflaría la señal de escalado de los
   workers: el grupo crecería por trabajos que no existen.
5. **Retiro seguro de workers.** Al apagarse, el worker espera 8 s (el valor por
   defecto de asynq) y devuelve el trabajo a la cola. Una transcodificación
   larga dura 196 s. Hay que fijar el tiempo de gracia del worker y del
   contenedor, y demostrar que el trabajo cortado se recupera sin derivados
   duplicados.
6. **Identificar la instancia que respondió.** Una cabecera con el nombre de la
   máquina, que Caddy puede añadir sin tocar Go, para demostrar el reparto del
   balanceador.
7. **Permisos negativos en la prueba de navegador.** Que un estudiante no pueda
   usar el estudio docente y que un profesor no pueda editar un curso ajeno.

## Entregables

| # | Entregable | Ubicación |
| --- | --- | --- |
| 1 | Plataforma desplegada con frontend, autoscaling en las dos capas, cola compartida | GCP |
| 2 | Release del código y la configuración | Tag `entrega-3` |
| 3 | Documento de arquitectura, con operación y reconstrucción | `docs/entrega3/arquitectura.md` |
| 4 | Verificación funcional del frontend, con matriz de recorridos y rutas | `docs/entrega3/` |
| 5 | Informe de capacidad con los escenarios 1, 2, 3A y 3B | `capacity-planning/pruebas_de_carga_entrega3.md` |
| 6 | Primer documento de deuda técnica | `docs/entrega3/deuda-tecnica.md` |
| 7 | Evidencias reproducibles enlazadas desde los informes | `capacity-planning/`, ejecuciones del pipeline |
| 8 | Video de sustentación | Enlazado desde el README |

El README debe dar las URL HTTPS del frontend y del balanceador, sin
credenciales.

## Costos

La estimación debe cubrir los dos grupos con una y con tres instancias, además
de balanceador, cola, base, objetos, red y monitoreo, con fecha y horas de uso.
El presupuesto `mooc-entrega2` de 50 USD sigue vigente. El balanceador cobra
por regla de reenvío aunque no haya tráfico, así que se apaga con el resto
cuando no se usa.

## Fuera de alcance

Alta disponibilidad multizonal, failover de la base, CDN, Kubernetes, Cloud
Run, despliegue continuo, panel administrativo completo, registro y
recuperación de contraseña en la interfaz.

## Reparto propuesto

| Frente | Contenido |
| --- | --- |
| Infraestructura elástica | Terraform del entorno `entrega3`: VM de Redis, plantillas, grupos, balanceador, autoscalers, firewall |
| Backend y calidad | Puntos 3 a 6 de «Trabajo de código», SonarQube Cloud y el documento de deuda |
| Frontend | Despliegue en la nube, permisos negativos, matriz de verificación |
| Capacidad | Escenarios 1 y 2 por el balanceador, 3A y 3B, línea de tiempo del autoscaling, informe |
