# Matriz de requisitos

IDs estables para citar requisitos desde ADRs, diseños, issues, pruebas y
mensajes de commit. La columna **E1** indica si entra en la Entrega 1
(backend + workers, sin frontend).

Leyenda de E1: **Sí** = se implementa completo · **Parcial** = se implementa la
parte de servidor, falta la de UI · **No** = fuera de esta entrega.

## RF — Alcance funcional obligatorio (enunciado §5.1)

| ID | Requisito | E1 | Notas |
| --- | --- | --- | --- |
| RF-01 | Registro público de estudiantes con verificación de correo, sesiones revocables y recuperación de contraseña. Profesores solo por administración. | Sí | Correo vía Mailpit. |
| RF-02 | Gestión administrativa de usuarios, roles, estados, sesiones y auditoría; protección del último administrador activo. | Sí | |
| RF-03 | Autoría de cursos: metadatos, jerarquía de cuatro niveles, ordenamiento, previsualización, estados y versiones publicadas inmutables. | Sí | La previsualización es un endpoint, no una pantalla. |
| RF-04 | Editor de bloques con autosave, recuperación y Markdown extendido canónico. | Parcial | Servidor: persistencia de bloques, autosave por `PATCH`, validación y normalización del Markdown. Editor visual: entrega posterior. |
| RF-05 | Carga multipart directa a objetos, reanudable 24 h, con verificación de integridad, MIME real y escaneo antimalware. | Sí | |
| RF-06 | Procesamiento asíncrono de video y audio a HLS, conservación del original, idempotencia y distribución por CDN. | Sí | CDN se simula con MinIO + cabeceras de caché; CDN real en GCP. |
| RF-07 | Visor PDF y reproducción adaptativa desde la última posición reportada. | Parcial | Servidor: entrega firmada del manifiesto HLS/PDF y persistencia de la última posición. Visor y reproductor: entrega posterior. |
| RF-08 | Quizzes de selección múltiple con intentos, guardado parcial, calificación en servidor y retroalimentación configurable. | Sí | |
| RF-09 | Progreso validado por servidor, finalización, aprobación e insignias únicas con imagen y URL verificable. | Sí | |
| RF-10 | Catálogo con búsqueda y filtros, inscripción, retiro y reinscripción conservando progreso y resultados. | Sí | |

## RO — Alcance opcional (enunciado §5.2)

| ID | Requisito | E1 |
| --- | --- | --- |
| RO-01 | Conversión de PPTX y ODP a PDF con previsualización. | No |
| RO-02 | Iframes restringidos: lista blanca, sandbox y política de permisos. | Parcial — la lista blanca se valida en el servidor al guardar el recurso. |
| RO-03 | Editor completo: tablas, fórmulas, tareas e historial visible de revisiones. | No |
| RO-04 | Borradores de actualización con clasificación de cambios y migración de progreso. | No — en E1 la edición de un curso publicado exige despublicarlo (§5.1). |
| RO-05 | Panel administrativo con métricas y resultados agregados por quiz. | No |
| RO-06 | Coautoría, exportación de datos personales e internacionalización. | No |
| RO-07 | Subtítulos, transcripciones, foros asíncronos y Open Badges 3.0. | No |

## CA — Condiciones verificables (enunciado §6)

Todas son exigibles en la Entrega 1: ninguna depende del frontend.

| ID | Condición | Cómo se demuestra |
| --- | --- | --- |
| CA-01 | **Publicación válida.** Solo se publica con metadatos completos, estructura mínima, criterios de aprobación y recursos visibles disponibles. | `POST /courses/{id}/versions/{n}:publish` devuelve la lista exhaustiva de errores; con todo completo, publica. |
| CA-02 | **Procesamiento asíncrono.** Carga directa al almacenamiento; el worker produce HLS sin bloquear la API y conserva el original. | La API responde `202` y el recurso pasa por `uploaded → scanning → processing → ready`. |
| CA-03 | **Tolerancia a fallos.** Entrega duplicada no genera salidas repetidas; tras tres reintentos fallidos el trabajo llega a la DLQ y emite alerta. | Reencolar el mismo job con la misma clave; inyectar fallo y observar backoff, DLQ y alerta. |
| CA-04 | **Integridad del quiz.** La clave correcta nunca llega al cliente, el envío definitivo es idempotente y la calificación se calcula en servidor. | Inspeccionar cada respuesta JSON del flujo de intento; reenviar el `submit` con la misma `Idempotency-Key`. |
| CA-05 | **Progreso verificable.** Heartbeats, permanencia y aperturas determinan el avance; porcentajes del cliente se rechazan y auditan. | Enviar `progress_percent` en el cuerpo → `422` + registro en auditoría. |
| CA-06 | **Control de acceso.** Cada endpoint respeta rol, propiedad e inscripción; materiales privados con URL firmada tras verificar el derecho. | Matriz de pruebas rol × endpoint; URL firmada expira y no se puede adivinar. |
| CA-07 | **Emisión de insignia.** Al llegar a `approved` se crea una única insignia verificable sin exponer el correo. | Disparar la aprobación dos veces; una sola insignia. `GET /verify/{codigo}` sin correo. |

## RT — Restricciones técnicas (enunciado §7)

| ID | Restricción | E1 |
| --- | --- | --- |
| RT-01 | Go, monolito modular, dominio desacoplado del framework HTTP y del proveedor cloud. | Sí |
| RT-02 | Frontend a elección, integrado por HTTPS. | No aplica en E1. |
| RT-03 | PostgreSQL transaccional; Redis para sesiones, caché, límites y cola; ningún binario en la base relacional. | Sí |
| RT-04 | Todo en Docker; entorno local con MinIO y Mailpit vía Docker Compose. | Sí |
| RT-05 | API REST `/api/v1`, OpenAPI 3.1, errores uniformes, cursores, `ETag` e `Idempotency-Key`. | Sí |
| RT-06 | WCAG 2.2 AA, OpenTelemetry, TLS 1.2+, cifrado en reposo, gestión externa de secretos. | Parcial — WCAG es del frontend. OTel, secretos externalizados y cifrado en reposo sí. TLS termina en el proxy; en local se documenta. |

## CE — Criterios de evaluación (enunciado §9)

| ID | Criterio | Prioridad | Cubierto en E1 por |
| --- | --- | --- | --- |
| CE-01 | Arquitectura y despliegue (incluye RPO ≤ 15 min, RTO ≤ 4 h) | Must | RT-01, RT-03, RT-04, escalado de API/worker, backup y restauración. |
| CE-02 | Identidad, autorización y seguridad | Must | RF-01, RF-02, CA-06. |
| CE-03 | Autoría y publicación | Must | RF-03, RF-04, CA-01. |
| CE-04 | Multimedia y distribución | Must | RF-05, RF-06, RF-07, CA-02. |
| CE-05 | Evaluación académica | Must | RF-08, CA-04. |
| CE-06 | Progreso e insignias | Must | RF-09, CA-05, CA-07. |
| CE-07 | Calidad operativa | Must/Should | Pruebas, observabilidad, prueba de carga. Accesibilidad queda pendiente por no haber frontend. |

## SEG — Segmentos de la demostración (enunciado §10.2)

Guion del video. Cada segmento es una carpeta de la colección de Postman.

| ID | Segmento | Estado en E1 |
| --- | --- | --- |
| SEG-1 | Identidad y administración | Completo por API. |
| SEG-2 | Autoría y publicación | Completo por API. |
| SEG-3 | Carga multimedia | Completo por API (multipart prefirmado desde Postman). |
| SEG-4 | Procesamiento y fallos | Completo: estados, doble entrega, backoff, DLQ, alerta, reencolado. |
| SEG-5 | Consumo de contenido | Parcial: inscripción, URL firmada del manifiesto HLS, reanudación por API. Sin reproductor ni teclado. |
| SEG-6 | Quiz | Completo por API. |
| SEG-7 | Progreso y aprobación | Completo por API, incluidas señales fraudulentas. |
| SEG-8 | Insignia y actualización | Completo por API. |
| SEG-9 | Operación | Completo: p95, escalado, observabilidad, fallos, backup y restauración. |

## RNF — No funcionales derivados

| ID | Objetivo | Origen |
| --- | --- | --- |
| RNF-01 | 50.000 usuarios registrados. | §1 |
| RNF-02 | 2.000 usuarios concurrentes. | §1 |
| RNF-03 | RPO ≤ 15 min. | §9 |
| RNF-04 | RTO ≤ 4 h. | §9 |
| RNF-05 | Objetivo de latencia p95 documentado y medido bajo carga. | §9, §10.2 SEG-9 |
| RNF-06 | Reintentos: 3 intentos con backoff exponencial, luego DLQ + alerta. | §6 |
| RNF-07 | Carga reanudable durante 24 h. | §5.1 |

> **Pendiente:** el enunciado exige "objetivos de latencia y disponibilidad"
> pero no fija números. Fijamos los nuestros en `disenos/objetivos-de-servicio.md`
> y los confirmamos con el profesor (ver `preguntas-profesor.md`).
