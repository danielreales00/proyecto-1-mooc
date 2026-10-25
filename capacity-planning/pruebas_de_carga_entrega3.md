# Pruebas de carga. Entrega 3

Informe de capacidad de la plataforma MOOC sobre la arquitectura elástica de
Google Cloud: dos grupos de instancias que escalan solos, un balanceador HTTPS
y la cola en su propia máquina. Repite los dos escenarios de la Entrega 2 y
añade un tercero dedicado al autoscaling. La línea base es el
[informe de la Entrega 2](pruebas_de_carga_entrega2.md), medido con el tag
`entrega-2`.

> **Estado del documento.** Las secciones 2 a 7 y 10 están cerradas: fijan qué
> se mide y cómo, antes de medir. Las marcadas como pendientes se completan con
> la sesión de medición.

## 1. Conclusiones

Pendiente de la sesión de medición.

## 2. Condiciones fijas y diferencias con la línea base

| Elemento | Entrega 2 | Entrega 3 |
| --- | --- | --- |
| Región y zona | `us-central1`, `us-central1-a` | Igual |
| Entrada | Caddy con TLS en el Web Server | Balanceador externo con certificado gestionado, modo `RATE` |
| API | Una `e2-highcpu-2` | Grupo de 1 a 3 `e2-highcpu-2`, objetivo 0,8 de 40 peticiones/s por instancia |
| Workers | Una `e2-highcpu-2` con `worker` y `worker-media` | Grupo de 1 a 3 `e2-highcpu-2`, 2 trabajos de `bulk` por instancia |
| Concurrencia por instancia | `worker` 20 en `critical=6,default=3`, `worker-media` 1 en `bulk=1` | Igual |
| Redis | En el Worker Server | En `mooc-cola`, `e2-small` dedicada |
| Base de datos | Cloud SQL `db-g1-small`, zonal | Igual |
| Pool por proceso | API 25, worker 10 | API 8, worker 3. Repartido para 3 + 3 instancias contra las 47 conexiones útiles |
| Arranque de instancias | Máquinas fijas | Imagen horneada `mooc-base-<sha>` |
| Límites de tasa por IP | Multiplicados por 100 | Igual |
| Generador | `e2-standard-4` con IP externa, en `us-central1-a` | Igual |
| Herramienta | k6 2.3.0 | Igual |
| Guiones | Tag `entrega-2` | Mismo recorrido, mezcla, pausas y tasas. La aleatoriedad pasa de `Math.random` a la Web Crypto API, con la misma distribución uniforme |
| Imágenes de la aplicación | `ef646a192a82` | Pendiente: el SHA de la sesión |

Tres diferencias pueden mover los resultados y se discuten con ellos.

1. **Pool de la API.** Con una sola instancia, la API tiene 8 conexiones frente a
   25. En la Entrega 2 el pool fue el segundo límite: con 10, el p95 de L3 era
   8,1 s, y con 25, 5,6 s. Con una instancia se espera más espera por el pool
   que en la línea base.
2. **Balanceador.** Añade un salto de red y termina TLS fuera de la instancia.
   Puede sumar latencia en cada petición y quitar a la instancia el trabajo de
   TLS. Las dos cosas se miden, no se suponen.
3. **Redis en otra máquina.** Cada comando cruza la red, frente a la red de
   Docker de la Entrega 2. La ida y vuelta medida entonces fue de 0,68 ms.

El generador no puede ser el límite. Se registra su CPU en cada corrida y la
corrida se descarta si pasa del 60 % o si pierde conexiones de su lado.

## 3. Herramienta

k6 2.3.0 en contenedor, desde el generador en la nube. Las peticiones a la API
entran por el balanceador. Las transferencias de archivos van directas a Cloud
Storage con URL firmadas, igual que en la Entrega 2. Los escenarios 1 y 2 usan
tasa de llegada fija (`constant-arrival-rate`), así que la carga ofrecida no baja
cuando el servidor se pone lento.

## 4. Datos sintéticos

Los mismos de la Entrega 2, creados de nuevo en la base de la Entrega 3.

| Dato | Cantidad | Cómo se crea |
| --- | --- | --- |
| Estudiantes y profesores | 600 y 10 | `make semilla-nube CARGA=1` |
| Cursos publicados | 20, con 60 lecturas y 3 quizzes de 10 preguntas cada uno | `k6/preparar.js` |
| Inscripciones e intentos previos | 510 y 200 | `k6/preparar.js` |
| Sesiones | 600, abiertas antes de medir | `k6/preparar.js` |
| Videos | Perfiles A (20 s, 360p), B (1 min, 720p) y C (2 min, 1080p) | `medios/generar.sh` y `k6/preparar-medios.js` |

## 5. Escenario 1. Actividad académica

**Recorrido.** El de la Entrega 2, sin cambios: catálogo, ficha, inscripción en
el 15 % de las iteraciones, contenido, cinco evidencias de progreso, progreso
calculado y un quiz en el 30 %. Unas 11 peticiones por iteración, 70 % lecturas.

| Nivel | Iteraciones/s | Peticiones/s aprox. | Calentamiento | Medición | Repeticiones |
| --- | --- | --- | --- | --- | --- |
| L1 | 4 | 30 | 2 min | 8 min | 2 |
| L2 | 8 | 61 | 2 min | 8 min, y una de 15 min | 2 |
| L2b | 12 | 91 | 2 min | 8 min | 2 |
| L3 | 16 | 110 | 2 min | 8 min | 2 |

Cada nivel empieza con el grupo de API en una instancia y espera a que vuelva a
una antes del siguiente. El grupo escala durante la corrida y se registra
cuántas instancias atendieron en cada minuto. Durante L2 corre el recorrido de
integridad de la Entrega 2: envío simultáneo, reenvío con la misma
`Idempotency-Key`, intento ajeno, clave del quiz en el snapshot y porcentaje
enviado por el cliente.

**Criterios.** Los de la Entrega 2. Éxito con el p95 de cada operación dentro de
su objetivo, `5xx` por debajo del 0,5 % y *timeouts* por debajo del 0,1 %.
Saturación con el p95 por encima de 3 veces el objetivo, `5xx` por encima del
1 % o un rendimiento que deja de crecer con la tasa. No se sube de nivel tras
uno saturado.

**Pregunta que responde.** Si añadir instancias de API mueve el límite, o si el
límite se queda en Cloud SQL `db-g1-small`, como en la Entrega 2.

## 6. Escenario 1. Resultados

Pendiente de la sesión de medición.

## 7. Escenario 2. Carga y procesamiento multimedia

**Recorrido.** El de la Entrega 2. Un profesor autoriza la carga, sube las partes
directo a Cloud Storage, confirma y sondea hasta un estado terminal. Los
espectadores abren una sesión de reproducción y descargan segmentos a la
cadencia del video.

| Nivel | Cargas/min | Mezcla A/B/C | Espectadores | Duración | Repeticiones |
| --- | --- | --- | --- | --- | --- |
| M1 | 3 | 2/1/0 | 20 | 10 min | 2 |
| M2 | 6 | 3/2/1 | 45 | 10 min | 2 |
| M3 | 12 | 6/4/2 | 90 | 10 min | 1 |

Entre niveles se espera a que la cola `bulk` quede vacía y el grupo de workers
vuelva a una instancia. Cada instancia conserva la concurrencia de la Entrega 2.
Se registran las instancias activas y la concurrencia total de FFmpeg en cada
minuto, porque esa es la diferencia arquitectónica que se evalúa.

**Criterios.** Los de la Entrega 2. Éxito si la cola drena, no hay trabajos
muertos y cada video está listo en menos de 3 veces su duración. Saturación si
la cola crece de forma monótona durante 5 minutos.

**Pregunta que responde.** Cuánto acortan tres workers la espera en cola y el
tiempo hasta `ready`. La autorización, la transferencia y la reproducción no
dependen de los workers, y su cambio no se atribuye al autoscaling sin
evidencia.

## 8. Escenario 2. Resultados

Pendiente de la sesión de medición.

## 9. Escenario 3. Definición del ciclo de autoscaling

### 3A. Escalado de la API

Recorrido del escenario 1, con escrituras, para que el ciclo también ponga a
prueba la integridad. Tres fases seguidas, desde el generador:

| Fase | Tasa | Duración | Qué se espera |
| --- | --- | --- | --- |
| Baja | L1, 4 iteraciones/s | 5 min | Una instancia |
| Alta | L3, 16 iteraciones/s, con el recorrido de integridad | 15 min | Crece hasta 3 instancias |
| Baja | L1 | 30 min | Reduce una instancia cada 10 min hasta 1 |

### 3B. Escalado de los workers

Cinco videos del perfil C a la vez (`k6/escalado-medios.js`), que piden tres
instancias. Después, 30 minutos sin carga para observar la reducción. Durante
la reducción se provoca el retiro de un worker con una transcodificación en
curso, subiendo dos videos más justo antes de la ventana de reducción.

### Qué se registra

| Grupo | Línea de tiempo |
| --- | --- |
| API | Peticiones/s, señal de utilización, decisión del autoscaler, instancias creadas, sanas en el balanceador y retiradas, latencias y errores |
| Workers | Trabajos de `bulk` pendientes y en curso, decisión del autoscaler, instancias creadas, primer trabajo tomado por cada una, retiros, trabajos completados, reintentos y tiempo hasta `ready` |

Se separan dos tiempos: de la subida de la demanda a la decisión del
autoscaler, y de la decisión a que la nueva instancia atiende o procesa. Al
terminar se reconcilian los trabajos aceptados con su estado final en
`platform.job_runs` y se comprueba que cada video tiene un solo juego de
derivados.

## 10. Métricas

| Métrica | Fuente | Ventana |
| --- | --- | --- |
| Latencia p50, p95 y p99, rendimiento, errores y *timeouts* | Resumen de k6 | La fase de medición, sin calentamiento |
| CPU, memoria, red y disco de cada instancia | Agente de operaciones en Cloud Monitoring | 60 s |
| Conexiones y CPU de Cloud SQL | Cloud Monitoring | 60 s |
| Profundidad, antigüedad y tasa de la cola | `/metrics` de los workers y `cola_bulk_trabajos` | 30 a 60 s |
| Instancias por grupo | Tamaño del grupo y salud en el balanceador | 60 s |
| Eventos de escalado | Registro de operaciones de Compute Engine | Por evento |

Los percentiles de corridas distintas no se promedian. Cada corrida se reporta
por separado y la variación se da como el rango entre repeticiones. La
duración de los trabajos asíncronos no se mezcla con la latencia HTTP.

## 11. Escenario 3. Resultados

Pendiente de la sesión de medición. La Sesión C del despliegue ya observó el
ciclo en las dos capas, con lecturas del catálogo desde fuera de la nube y sin
generador
([evidencia](../docs/entrega3/evidencias/elastica/2026-10-24-sesion-c.md)). Esa
observación valida el mecanismo. Las cifras de esta sección salen de las
corridas con el generador.

## 12. Comparación con la Entrega 2

Los valores de la Entrega 2 salen de su informe. Los de la Entrega 3 se
completan con la sesión de medición.

| Escenario y operación | Nivel de carga | Métrica y unidad | Entrega 2 | Entrega 3 | Cambio observado y explicación |
| --- | --- | --- | --- | --- | --- |
| E1, recorrido completo | L1, 30 peticiones/s | p95 (ms) | 23 | Pendiente | |
| E1, recorrido completo | L2, 61 peticiones/s | p95 (ms) | 25 | Pendiente | |
| E1, recorrido completo | L2b, 91 peticiones/s | p95 (ms) | 272 | Pendiente | |
| E1, recorrido completo | L3, 110 peticiones/s | p95 (ms) | 8.067 | Pendiente | |
| E1, recorrido completo | L3 | Iteraciones perdidas | 531 | Pendiente | |
| E1, recorrido completo | Todos | Errores `5xx` y *timeouts* | 0 | Pendiente | |
| E1, integridad | L2 | Comprobaciones fallidas | 0 de 244 | Pendiente | |
| E2, autorizar carga | M2, 6 cargas/min | p95 (ms) | 308 | Pendiente | |
| E2, procesamiento | M1, 3 cargas/min | Confirmación a `ready`, mediana A / B (s) | 10 / 35 | Pendiente | |
| E2, procesamiento | M2, 6 cargas/min | Confirmación a `ready`, mediana A / B / C (min) | 9,4 / 9,6 / 11,3 | Pendiente | |
| E2, procesamiento | M2 | Transcodificaciones/min | 1,3 | Pendiente | |
| E2, cola | M2 | Profundidad máxima de `bulk` | 779 | Pendiente | |
| E2, reproducción | M2 | Segmentos fallidos | 0 de 4.689 | Pendiente | |

La profundidad de la cola de la Entrega 2 está inflada por los duplicados del
*reaper*, 856 en M2. En la Entrega 3 cada tarea lleva su clave como
identificador y la cola no admite copias, así que esa fila compara la espera
real con una medida inflada. Se discute junto al tiempo hasta `ready`, que no
tiene ese sesgo.

## 13. Limitaciones

Pendiente de la sesión de medición.

## 14. Reproducir estas pruebas

| Qué | Dónde |
| --- | --- |
| Guiones de k6 | [`k6/`](k6/): `preparar.js`, `escenario1.js`, `preparar-medios.js`, `escenario2.js`, `escalado-api.js`, `escalado-medios.js`, `comun.js` y `medios.js` |
| Videos | [`medios/generar.sh`](medios/generar.sh) |
| Resultados originales | [`resultados/`](resultados/), con el prefijo `e3-` |
| Tablas y gráficas | [`metricas/tabla.py`](metricas/tabla.py) y [`metricas/graficas.py`](metricas/graficas.py) |

```bash
make semilla-nube CARGA=1
make carga-sincronizar && make carga-medios
make carga K6=preparar.js ETIQUETA=e3-preparar
make carga-nivel K6=escenario1.js ETIQUETA=e3-L2 ARGS="-e TASA=8 -e INTEGRIDAD=1"
make carga K6=preparar-medios.js ETIQUETA=e3-preparar-medios
make carga-nivel K6=escenario2.js ETIQUETA=e3-M2 \
  ARGS="-e CARGAS=6 -e MEZCLA=3/2/1 -e ESPECTADORES=45 -e DURACION=10m -e ESPERA_MAX=20"
make carga-tabla ARGS="e1 e3-L1 e3-L2 e3-L2b e3-L3"
```
