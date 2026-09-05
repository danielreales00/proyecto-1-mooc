# Diseños

| Documento | Qué contiene |
| --- | --- |
| [contexto-y-componentes.md](contexto-y-componentes.md) | Diagramas de contexto y de componentes, módulos del monolito, servicios de Compose, cómo escala cada pieza |
| [modelo-de-datos.md](modelo-de-datos.md) | ERD, tablas por esquema, restricciones que sostienen las reglas de negocio, índices |
| [api-v1.md](api-v1.md) | Inventario de endpoints, convenciones transversales, códigos de error, reparto entre las 4 personas |
| [trabajos-asincronos.md](trabajos-asincronos.md) | Catálogo de trabajos, encolado con *outbox*, reintentos, DLQ, fallos inyectables para la demo |
| [maquinas-de-estado.md](maquinas-de-estado.md) | Curso, asset, intento de quiz, inscripción e insignia |
| [objetivos-de-servicio.md](objetivos-de-servicio.md) | Latencia, disponibilidad, RPO/RTO y plan de prueba de carga |
| [ruta-a-gcp.md](ruta-a-gcp.md) | Arquitectura destino, equivalencias y lo que queda por resolver |

Los diseños describen **qué hay**. Las razones están en `../adr/`. Si un diseño
explica por qué se eligió algo, eso pertenece a un ADR.
