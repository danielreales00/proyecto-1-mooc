# ADR-0007 — Versiones de curso inmutables e identificadores estables

- **Estado:** Aceptado
- **Fecha:** 2026-09-05
- **Requisitos:** `RF-03`, `RF-09`, `RF-10`, `CA-01`, `CE-03`, `CE-06`

## Contexto

El enunciado (§3) exige que una versión publicada sea inmutable, que las
ediciones vayan sobre un borrador, y que el progreso del estudiante sobreviva a
la publicación de una versión nueva gracias a **identificadores estables**. Es la
regla de negocio más delicada del proyecto: se cruza con progreso, con quizzes y
con insignias.

## Decisión

### Dos identificadores por nodo de contenido

Cada módulo, unidad y recurso lleva:

- `id` (uuid): la **fila**. Cambia en cada versión, porque cada versión tiene sus
  propias filas.
- `stable_id` (uuid): la **identidad pedagógica**. Se genera al crear el nodo por
  primera vez y **se copia tal cual** al clonar la versión.

Todo lo que registra actividad del estudiante — progreso por recurso, intentos de
quiz, última posición de reproducción — referencia `stable_id`, nunca `id`.

### Ciclo de vida

```
draft ──publicar──> published ──despublicar──> unpublished
                        ▲                          │
                        └────── publicar ──────────┘
                                                   │
                                        editar en sitio (§5.1)

published ──"nueva versión"──> se clona a un draft con version_number+1
```

`draft` y `unpublished` son editables; `published` no. Despublicar no cambia el
`version_number` ni pierde inscripciones.

- `course_versions.status ∈ {draft, published, unpublished, archived}`.
- **Una versión `published` es inmutable**: un trigger de PostgreSQL rechaza
  `UPDATE`/`INSERT`/`DELETE` sobre módulos, unidades, recursos, quizzes y
  preguntas cuya versión esté publicada. La regla vive en la base, no solo en Go,
  porque es la que más fácil se salta por descuido.
- Para la Entrega 1 se aplica lo que autoriza §5.1: **editar un curso publicado
  exige despublicarlo**. `unpublished` bloquea inscripciones nuevas pero conserva
  las existentes y su progreso.
- Publicar una versión nueva marca la anterior como `archived` y actualiza
  `courses.current_version_id`.

### Validación de publicación (`CA-01`)

`POST /courses/{id}/versions/{n}:publish` no falla al primer error: recorre todas
las reglas y devuelve **la lista completa**.

| Regla | Mensaje |
| --- | --- |
| Metadatos completos (título, resumen, categoría, idioma, portada) | `metadata.missing_fields` |
| Al menos un módulo → una unidad → un recurso visible | `structure.minimum_not_met` |
| Ningún recurso visible en estado distinto de `ready` | `resource.not_available` |
| Criterios de aprobación definidos (`% obligatorios`, `nota mínima de quiz`) | `approval.criteria_missing` |
| Todo quiz tiene ≥ 1 pregunta y cada pregunta ≥ 1 opción correcta | `quiz.invalid` |
| Todo iframe apunta a un dominio de la lista blanca | `resource.iframe_not_allowed` |

### Continuidad del progreso

Al publicar la versión N+1, la inscripción **no se migra**: apunta al curso, no a
la versión. El progreso se recalcula contra el conjunto de recursos obligatorios
de la versión vigente, emparejando por `stable_id`:

| Situación | Efecto |
| --- | --- |
| El `stable_id` sigue existiendo y sigue obligatorio | Conserva su estado de completado |
| El `stable_id` desapareció | Se ignora en el cálculo; la fila se conserva para auditoría |
| Aparece un `stable_id` nuevo y obligatorio | El porcentaje baja; la inscripción puede volver de `completed` a `in_progress` |
| El estudiante ya estaba `approved` | **Sigue `approved`.** La insignia emitida no se revoca por una versión nueva |

Esa última fila es una decisión de negocio explícita: aprobar es un hecho
histórico contra la versión que se cursó. `enrollments.approved_version_id` deja
constancia de cuál fue.

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| Un solo árbol mutable con `updated_at` | Mucho más simple | No hay inmutabilidad ni versiones | Lo prohíbe §3 |
| Versionado por *event sourcing* | Historia total | Complejidad desproporcionada para 13 semanas | Coste |
| Copiar el árbol en JSONB por versión | Clonado trivial | Se pierde la integridad referencial y consultar recursos se vuelve incómodo | Los recursos se consultan mucho |
| Inmutabilidad solo en Go | Menos SQL | Un `UPDATE` de mantenimiento la rompe en silencio | La regla debe estar donde están los datos |

## Consecuencias

- Clonar una versión copia filas de cinco tablas; se hace con `INSERT ... SELECT`
  en una transacción y es rápido a esta escala.
- El almacenamiento crece por versión, pero los binarios no se duplican: los
  recursos de la versión nueva apuntan al mismo `asset_id`.
- Los triggers de inmutabilidad hacen que un `UPDATE` mal escrito falle ruidosa e
  inmediatamente, que es lo que se quiere.
- La lista exhaustiva de errores de publicación obliga a escribir las validaciones
  como funciones puras del dominio; se prueban unitariamente sin base de datos.

## Cómo se verifica

- SEG-2: intentar publicar un curso incompleto y comprobar que devuelve **todos**
  los errores en un solo `422`, no el primero.
- `UPDATE authoring.resources SET title='x'` sobre una versión publicada → error
  de PostgreSQL.
- SEG-8: completar un curso, publicar una versión nueva con un recurso obligatorio
  añadido, y comprobar que el progreso conserva los `stable_id` previos y que la
  insignia sigue vigente.
