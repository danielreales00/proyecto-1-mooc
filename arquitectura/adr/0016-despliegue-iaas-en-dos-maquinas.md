# ADR-0016 — Despliegue IaaS en dos máquinas para la Entrega 2

- **Estado:** Aceptado — 2026-09-27
- **Fecha:** 2026-09-27
- **Requisitos:** `RT-01`, `RT-06`, `CE-01`
- **Refina y corrige:** ADR-0015 (D4, D5, D6, D7). Mantiene intactos D1, D2 y D3.

## Contexto

ADR-0015 se escribió con la arquitectura objetivo del proyecto en la cabeza:
Cloud Run, Memorystore, Cloud CDN y un balanceador con certificado gestionado.
El enunciado de la Entrega 2, publicado después, **prohíbe explícitamente esos
cuatro servicios** para esta etapa:

> No se utilizarán CDN, plataformas administradas de ejecución de contenedores
> ni funciones serverless para sustituir los componentes solicitados.
>
> No se implementarán escalado automático, balanceadores de carga, réplicas de
> aplicación entre máquinas ni mecanismos de alta disponibilidad.

Y fija la forma del despliegue: **dos máquinas virtuales**, *Web Server* y
*Worker Server*, de **2 vCPU, 2 GiB de RAM y 30 GiB de disco cada una**, con
PostgreSQL en el servicio administrado, los objetos en el almacenamiento
administrado del proveedor y el sistema de mensajería en un contenedor dentro
del *Worker Server*.

No es un capricho del enunciado: la entrega se evalúa por la **capacidad medida
de esa configuración básica**, y una plataforma que escala sola no tiene punto
de degradación que medir. El límite es el objeto de estudio.

La consecuencia es que ADR-0015 describe el destino y no el siguiente paso.
Este ADR fija el siguiente paso sin borrar el destino.

## Decisión

### D1 — Cómputo: Compute Engine con Docker Compose en cada máquina

Dos instancias de Compute Engine, cada una corriendo el mismo `docker compose`
que se usa en local, recortado a lo que le toca:

| Máquina | Contenedores | Red |
| --- | --- | --- |
| **Web Server** | `caddy` (TLS y entrada), `api`, `swagger` | IP externa estática, 80/443 abiertos a Internet |
| **Worker Server** | `redis`, `worker`, `worker-media`, `clamav` | **Sin IP externa.** Solo la subred privada; salida por Cloud NAT |

Esto **cierra D4 de ADR-0015 para esta entrega**: `worker-media` va en una
máquina, no en Cloud Run, porque Cloud Run está prohibido aquí. La pregunta que
D4 quería responder —si la transcodificación cabe— se responde igual, midiendo,
pero ahora sobre 2 vCPU fijos.

El motivo de fondo para reutilizar Compose en vez de escribir `systemd` units o
un orquestador es el ADR-0010: lo que corre en la nube debe poder correr en
local cambiando variables de entorno. Dos archivos de composición que heredan
del mismo `docker-compose.yml` conservan esa propiedad; un mecanismo de arranque
distinto la rompe el primer día.

> **Matiz del 2026-09-27, al implementarlo.** «Heredan» es de contenido, no de
> `extends`: las máquinas no tienen el repositorio —reciben solo su composición
> por los metadatos de la instancia— y `extends` necesitaría el archivo base
> al lado. `deploy/compose.web.yml` y `deploy/compose.worker.yml` repiten los
> servicios de `docker-compose.yml` con los mismos *healthchecks* y variables, y
> solo cambian lo que la nube cambia: imágenes del registro con el SHA, base en
> Cloud SQL, Redis en el Worker Server. La propiedad que importa se mantiene:
> los mismos binarios, leyendo las mismas variables.

### D2 — Redis único, en el Worker Server, y lo que eso cuesta

El enunciado dice dónde va:

> Su sistema de mensaje (Redis, RabbitMQ u otro) se desplegará en un contenedor
> en *Worker Server*, accesible por la red privada.

Nuestro Redis no es solo la cola. Sirve cuatro cosas en cuatro bases: cola (0),
sesiones (1), caché (2) y límite de tasa (3). El `worker` solo usa la cola; la
`api` usa las otras tres **y** la cola, para encolar.

**Se acepta un solo Redis en el Worker Server, con lo que implica:** cada
petición autenticada a la API cruza la red privada para validar la sesión y
contar el límite de tasa, y vuelve a cruzarla si encola. Es el salto de red más
repetido de toda la plataforma y la primera hipótesis de cuello de botella del
escenario 1.

La alternativa —un Redis en cada máquina, el de sesiones local a la API— exige
una variable de configuración nueva (`REDIS_SESSION_ADDR`), porque hoy
`REDIS_ADDR` es una sola dirección para los cuatro usos. Se descarta por ahora
por dos razones: el enunciado sitúa el sistema de mensajería en el Worker
Server, y **medir primero da el número que justificaría el cambio**. Si el
escenario 1 muestra que el salto domina la latencia, partir Redis es la
propuesta de evolución, con su medición detrás. Esa es exactamente la forma de
hallazgo que la entrega pide.

### D3 — Cloud SQL zonal, sin alta disponibilidad, con IP privada

ADR-0015 (D6) pedía PostgreSQL 17 con alta disponibilidad regional y
recuperación a un punto en el tiempo. El enunciado lo prohíbe: *«Despliegue en
una sola zona de disponibilidad, sin réplicas de lectura»*.

- **PostgreSQL 17**, edición Enterprise, **zonal**, sin réplicas.
- **Solo IP privada**, por acceso privado a servicios (peering de VPC). Sin IP
  pública: el enunciado exige que la conexión a la base administrada sea
  privada, y sin IP pública no hay forma de equivocarse.
- Copias automáticas diarias **sí**: son la vía para borrar la instancia al
  final sin perder la evidencia, que el enunciado pide como control de costo.
- El tamaño de cómputo y de disco se elige en la fase 1 según lo que la cuenta
  admita, y **se registra en el informe**. No lo fija el enunciado.

Las máquinas hablan con la base por la IP privada, sin conector y sin
intermediario: `DATABASE_URL` apunta directamente. Con dos instancias fijas, la
aritmética de conexiones del ADR-0015 (D6) deja de ser un riesgo —no hay
escalado que multiplique pools— y se convierte en un parámetro a medir:
`DB_MAX_CONNS` por máquina, contra el límite de la instancia.

### D4 — TLS termina en Caddy, sobre la IP externa del Web Server

Sin balanceador no hay certificado gestionado de Google, y el enunciado exige
que el acceso de usuarios sea por HTTPS conservando las cookies seguras y la
protección CSRF.

**Caddy en el Web Server, con certificado de Let's Encrypt automático.** Eso
necesita un nombre, no una IP. Dos caminos:

1. **Dominio propio**, si el equipo tiene uno. Es lo limpio.
2. **`<ip-con-guiones>.sslip.io`** sobre la IP externa estática. `sslip.io`
   resuelve cualquier IP codificada en el nombre y está en la lista de sufijos
   públicos, así que Let's Encrypt emite certificado para él sin trámite.

Se recomienda el segundo mientras no haya dominio: cuesta cero, no depende de
que nadie registre nada y se cambia después poniendo otro nombre en el
`Caddyfile`. Lo que **no** se hace es certificado autofirmado: obligaría a
saltarse la advertencia del navegador en el video de sustentación y a desactivar
la verificación en el generador de carga, que es tanto como no medir TLS.

La IP externa es **estática**, no efímera: una IP efímera cambia al reiniciar la
máquina y deja el certificado y el nombre apuntando a otro sitio.

### D5 — HLS directamente desde Cloud Storage, con URL firmada

El enunciado zanja lo que ADR-0015 (D2) dejaba para la fase de CDN:

> En esta etapa, el contenido multimedia se servirá directamente desde el
> almacenamiento de objetos. La distribución mediante CDN se abordará en una
> etapa posterior.

`POST /enrollments/{id}/media-sessions` **sigue devolviendo
`delivery: "signed_url"`**, igual que en la Entrega 1. No cambia una línea del
contrato ni del cliente. La cookie firmada de CDN se queda donde estaba, en la
etapa siguiente.

Esto es la comprobación de que D2 valía la pena: el contrato se diseñó para
absorber este cambio, y el enunciado acaba de mover la fecha sin coste.

### D6 — Un solo entorno

ADR-0015 (D7) pedía `dev` y `prod`. Con una entrega de semana y media y dos
máquinas encendidas, duplicar el entorno duplica la factura para proteger un
entorno que nadie va a usar.

**Un solo entorno, `entrega2`**, con la estructura de módulos de ADR-0015 (D5)
intacta: el día que haga falta `prod`, se crea con las mismas variables. La
estructura de directorios es lo que hace barato tener dos entornos; tener dos
entornos encendidos no lo es.

### D7 — Secretos: contenedor en Secret Manager, valor leído al arrancar

El enunciado es explícito: nada de credenciales en el repositorio, en las
imágenes ni en las evidencias, y la configuración sensible se inyecta en
ejecución.

- Terraform crea los **contenedores** de Secret Manager, nunca los valores
  (ADR-0015, D5: un secreto en el estado de Terraform es un secreto en claro
  dentro de un bucket).
- El *script* de arranque de cada máquina lee los valores con la **identidad de
  la propia instancia** y escribe el `.env` que consume Compose, con permisos
  `0600` y fuera de cualquier imagen.
- Se versiona un `.env.example` sin valores, como pide el enunciado.

D3 de ADR-0015 —**nunca una clave JSON de cuenta de servicio**— sigue igual, y
aquí es más fácil de cumplir que en Cloud Run: una instancia de Compute Engine
tiene identidad propia por definición.

## Qué de ADR-0015 sigue en pie

| Decisión | Estado tras este ADR |
| --- | --- |
| **D1** · Adaptador GCS nativo, multipart emulado con `compose` | **Intacto y más necesario que nunca.** La carga sigue yendo directa al almacenamiento y las URL se siguen firmando sin clave descargada |
| **D2** · Credencial de reproducción por sesión | **Intacto.** Se queda en `signed_url`; la cookie de CDN espera (ver D5 de aquí) |
| **D3** · Workload Identity Federation, sin claves JSON | **Intacto** |
| **D4** · Dónde corre `worker-media` | **Resuelto para esta entrega:** en el Worker Server. La pregunta vuelve a abrirse cuando vuelva Cloud Run |
| **D5** · Estructura de Terraform y estado remoto | **Intacto**, con un entorno en vez de dos |
| **D6** · Cloud SQL con HA, Memorystore con réplica | **Corregido:** zonal sin HA, y Memorystore sustituido por un contenedor |
| **D7** · Dos entornos | **Corregido:** uno |

## Consecuencias

**A favor:**

- El despliegue se parece mucho más a lo que ya funciona. Las mismas imágenes,
  el mismo Compose, el mismo Caddy. `/docs` deja de ser un hueco —el cuarto de
  los que se encontraron el 26 de septiembre— porque Caddy viaja con nosotros.
- `TRUSTED_PROXY_HOPS=0` es correcto en la nube, porque Caddy **sobrescribe**
  `X-Forwarded-For` en vez de añadirle. No hay que medir cuántos saltos añade la
  infraestructura: no hay infraestructura delante.
- `REDIS_TLS` queda en `false`: el tráfico no sale de la subred privada. El
  trabajo hecho no se pierde, se activa el día que vuelva Memorystore.

**En contra, y hay que decirlo:**

- **Ni una sola pieza es redundante.** Dos máquinas fijas, una base zonal, un
  Redis sin réplica. Cualquiera de las tres cae y la plataforma cae. Es una
  decisión del enunciado, no un descuido, y el informe la documenta como el
  punto único de falla que es.
- **Perder el Worker Server pierde las sesiones**, no solo la cola: el Redis que
  se cae es el mismo que guarda las sesiones de todo el mundo. Los trabajos
  sobreviven porque viven en PostgreSQL (ADR-0004), las sesiones no.
- **2 GiB en el Worker Server son pocos.** `clamd` mantiene la base de firmas
  residente —del orden de 1 GiB— y al lado tienen que caber Redis, dos procesos
  Go y FFmpeg. Es la primera cosa que puede no caber, y el enunciado prevé el
  caso: si una dependencia no arranca con esos recursos, se documenta el fallo,
  el ajuste mínimo y su efecto en costo y capacidad.
- **30 GiB de disco también son pocos** para transcodificar: el original más las
  tres calidades HLS conviven en disco mientras dura el trabajo, y el disco
  aloja además las imágenes y la base de firmas. Es la segunda cosa que puede
  romperse, y se vigila en el escenario 2.

## Cómo se verifica

- `terraform plan` sin cambios pendientes tras `apply`, con el estado en GCS.
- `gcloud compute instances list` muestra dos instancias y **solo una con IP
  externa**.
- La instancia de Cloud SQL no tiene IP pública, y conectarse desde fuera de la
  VPC falla.
- `https://<host>/healthz` responde con certificado válido emitido por una
  autoridad pública.
- La colección de Postman de la Entrega 1 pasa entera contra el `base_url`
  desplegado. Es la prueba de que ADR-0010 era cierto.
- `grep -rn "credentials.json\|service_account.*key" .` sigue sin devolver nada.
