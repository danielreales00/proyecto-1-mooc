# CLAUDE.md — Plataforma MOOC

Proyecto de la maestría (curso de desarrollo de soluciones cloud). Plataforma web
de cursos masivos abiertos en línea, operada por una sola organización.
Equipo de 4 personas, 4–5 entregas en 13 semanas.

## Dónde está la información

| Documento | Para qué |
| --- | --- |
| `arquitectura/enunciado.md` | Transcripción fiel del PDF del profesor. **Fuente de verdad funcional.** |
| `arquitectura/aclaraciones-profesor.md` | Aclaraciones posteriores, por entrega. Prevalecen sobre el enunciado. |
| `arquitectura/requisitos.md` | Matriz de requisitos con IDs trazables (`RF-`, `CA-`, `RT-`, `CE-`). |
| `arquitectura/alcance-entrega-1.md` | Qué se entrega ahora y qué queda para después. |
| `arquitectura/adr/` | Decisiones de arquitectura. Una decisión por archivo, numeradas. |
| `arquitectura/disenos/` | Modelo de datos, API, trabajos asíncronos, máquinas de estado, ruta a GCP. |
| `arquitectura/guia-de-informes.md` | Convenciones de escritura de los informes de arquitectura. Valen para todas las entregas. |
| `arquitectura/pendientes.md` | Notas de trabajo. No es entregable. |
| `arquitectura/preguntas-profesor.md` | Dudas abiertas y sus respuestas. |
| `2026-20 proyecto-plataforma-mooc.pdf` | El PDF original. |

**Regla de precedencia:** aclaración del profesor > enunciado > ADR > código.
Si el código contradice un ADR, o el ADR se actualiza o el código se corrige;
nunca se deja la contradicción en silencio.

## Restricciones que no se negocian

Vienen del enunciado (sección 7) y de la aclaración del profesor:

- **Go** para backend y workers. Monolito modular; el dominio no depende del
  framework HTTP ni del proveedor cloud.
- **Todo corre en Docker / Docker Compose.** No hay toolchain de Go instalada en
  la máquina local: compilar, probar y ejecutar se hace dentro de contenedores.
  Los scripts de `scripts/` tampoco corren en la máquina: van en la imagen
  `mooc-herramientas` (`scripts/Dockerfile`), que el Makefile invoca por debajo.
  Un script nuevo se escribe para esa imagen, no para el bash de quien lo lanza.
- API y workers **sin estado**, escalables a varias instancias
  (`docker compose up --scale api=3 --scale worker=3`).
- **PostgreSQL** es la fuente de verdad transaccional. **Redis** soporta
  sesiones, caché, rate limiting y la cola (asynq).
- **Ningún binario en la base relacional.** Originales, derivados HLS, PDF
  convertidos e imágenes de insignias van a almacenamiento de objetos.
- API REST bajo `/api/v1`, OpenAPI 3.1, errores uniformes, paginación por
  cursor, `ETag` e `Idempotency-Key`.
- Entorno local con **MinIO** (objetos) y **Mailpit** (correo).
- Los workers consumen con **idempotencia, reintentos con backoff y DLQ**.

## Invariantes de dominio

Se rompen fácil y cuestan puntos en la evaluación. Antes de tocar estas áreas,
leer el ADR correspondiente:

1. Una versión de curso **publicada es inmutable**. Toda edición posterior va
   sobre un borrador nuevo. → `adr/0007`
2. El progreso se ancla a **`stable_id`**, no al `id` de fila, para sobrevivir a
   una versión nueva. → `adr/0007`
3. **La clave correcta de un quiz nunca sale del servidor.** Ni en el detalle
   del recurso, ni en el snapshot del intento, ni en la retroalimentación
   cuando la política lo prohíbe. → `adr/0013`
4. El **porcentaje de progreso lo calcula el servidor** a partir de evidencias
   (apertura, heartbeats, permanencia). Un porcentaje enviado por el cliente se
   rechaza y se audita. → `adr/0012`
5. La **insignia se emite una sola vez** por inscripción aprobada, y su URL
   pública no expone el correo del estudiante. → `adr/0008`
6. Un trabajo entregado dos veces **no produce dos salidas**. → `adr/0008`

## Convenciones

- **Documentación y mensajes de commit en español.** Identificadores, nombres de
  paquetes y de tablas en inglés.
- **Pocas dependencias.** `net/http` de la stdlib para el ruteo; se agrega una
  librería solo cuando resuelve algo que la stdlib no. Cada dependencia nueva
  se justifica en un ADR. → `adr/0002`
- **Errores uniformes** en toda la API: mismo cuerpo JSON, mismo formato.
- Las migraciones son **SQL versionado, hacia adelante**. Nada de ORM que
  genere el esquema.
- Los secretos salen de variables de entorno (`.env` local, gestor externo en
  la nube). `.env` nunca se versiona; `.env.example` sí.
- **Nunca se crean claves JSON de cuenta de servicio de GCP**, ni siquiera para
  probar. La autenticación hacia GCP va por Workload Identity Federation. Una
  clave creada "temporalmente" acaba en el repositorio. → `adr/0015` (D3)
- **El contrato manda.** Un endpoint nuevo entra primero en
  `backend/openapi/openapi.yaml`. La API lo sirve en `GET /openapi.yaml`.

## Estado actual

Entrega 1 entregada y grabada. **63 de 89 operaciones** responden; el resto
lleva `x-estado: planificado` en el contrato. El flujo completo se recorre de
punta a punta con `make demo` y la colección entera con `make postman-completo`.

Después de grabar el video se añadió la **transcodificación a HLS**
(`worker-media` con FFmpeg sobre la cola `bulk`) y la entrega firmada del
manifiesto. Falta ClamAV: mientras no esté, `media.probe` deja el asset en
`clean` y encola la transcodificación desde ahí. Ver
[`arquitectura/media-plan.md`](arquitectura/media-plan.md) y la nota de estado
al final del `adr/0011`.

**El guion de `demo/guion.md` describe lo que se grabó**, no el estado actual
del repositorio: no se actualiza hacia atrás.

La demostración se hace con Postman. El guion del video está en
`demo/guion.md`, dimensionado para el **máximo de 20 minutos**, y el reparto
entre las cuatro personas en `demo/reparto.md`. Ver
`arquitectura/alcance-entrega-1.md` y `arquitectura/pendientes.md`.

**Antes de empujar, `make ci` y esperar su código de salida.** No basta leer
«Todo en verde» dentro del log: eso lo imprime `make demo`, que es una parte.

En entregas siguientes: frontend y despliegue en **GCP**. Las decisiones de hoy
se toman sin amarrarse al proveedor; el mapeo previsto está en
`arquitectura/disenos/ruta-a-gcp.md`.
