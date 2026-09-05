# ADR-0003 — PostgreSQL como fuente de verdad, con migraciones SQL versionadas

- **Estado:** Aceptado
- **Fecha:** 2026-09-05
- **Requisitos:** `RT-03`, `CE-01`, `RNF-03`, `RNF-04`

## Contexto

El enunciado fija PostgreSQL como fuente de verdad transaccional y prohíbe
binarios en la base. Falta decidir cómo evoluciona el esquema, qué tipos de
identificador se usan y cómo se cumple RPO ≤ 15 min / RTO ≤ 4 h.

## Decisión

- **PostgreSQL 17.** Una sola base, un esquema por módulo de dominio (`identity`,
  `authoring`, `media`, `assessment`, `progress`, `audit`), lo que hace visible la
  frontera del ADR-0001 y facilita un corte futuro.
- **Migraciones:** archivos `NNNN_descripcion.up.sql` en `backend/migrations/`,
  numerados y **solo hacia adelante**. Sin `down`: revertir en producción se hace
  con una migración nueva. Las aplica `cmd/migrate` en un contenedor de arranque
  que la API y los workers esperan (`depends_on: service_completed_successfully`).
- **Identificadores:** `uuid` v7 generado en la aplicación (ordenado en el
  tiempo, buena localidad de índice, no revela cardinalidad como un `serial`).
- **Concurrencia:** aislamiento por defecto `READ COMMITTED`; las operaciones que
  deben ser únicas se protegen con **restricciones únicas parciales**, no con
  lecturas previas (ver ADR-0008).
- **Dinero de tiempo:** todo `timestamptz` en UTC.
- **Sin `DELETE` en tablas de dominio con historia:** los usuarios y los cursos se
  marcan (`status`); las inscripciones se retiran, no se borran (`RF-10` exige
  reinscribir conservando progreso).
- **Auditoría inmutable:** tabla `audit.events` en modo *append-only*, garantizado
  por un trigger `BEFORE UPDATE OR DELETE` que lanza excepción, y por permisos
  del rol de aplicación.
- **Respaldo:** `pg_dump` completo diario más WAL archivado continuo a
  almacenamiento de objetos, con un job de restauración probado
  (`make restore-test`) que mide RTO. El WAL cada ≤ 5 min deja RPO holgado frente
  a los 15 min exigidos.

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| Migraciones con `golang-migrate` | Herramienta conocida | Una dependencia y un binario más | 120 líneas propias en `cmd/migrate` bastan y quedan bajo nuestro control |
| Migraciones con `down` | Rollback teórico | Casi nunca funciona con datos reales y da falsa confianza | Se prefiere avanzar |
| `bigserial` | Compacto | Enumerable desde fuera; complica el escalado | Seguridad y portabilidad |
| Un esquema único `public` | Menos ceremonia | La frontera entre módulos se vuelve invisible | Se pierde el control del ADR-0001 |

## Consecuencias

- Los `GRANT` por esquema permiten comprobar en pruebas que un módulo no puede
  escribir en las tablas de otro.
- Numerar migraciones a mano produce conflictos cuando dos personas trabajan a la
  vez; se resuelve renumerando al rebasar, y CI verifica que no haya números
  repetidos.
- UUIDv7 requiere generar el ID en Go antes del `INSERT`, lo que además simplifica
  la idempotencia.

## Cómo se verifica

- `make migrate` sobre una base vacía y sobre la base de la rama anterior.
- `make restore-test`: restaura el último respaldo en un contenedor limpio,
  ejecuta el smoke test y reporta el tiempo transcurrido.
- Prueba que intenta `UPDATE` sobre `audit.events` y espera error.
