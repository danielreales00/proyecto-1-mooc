# ADR-0014 — Markdown extendido canónico como formato del contenido textual

- **Estado:** Aceptado
- **Fecha:** 2026-09-05
- **Requisitos:** `RF-04`, `CE-03`, `CE-02`

## Contexto

El enunciado exige que el contenido textual se persista como "Markdown extendido
canónico" (§3) y recomienda prototipar temprano la biyección entre el AST del
editor y ese Markdown, con pruebas de ida y vuelta bloqueantes por tipo de nodo
(§11). En la Entrega 1 no hay editor, pero **sí hay que fijar el formato y el
normalizador**, porque el editor de E2 se construirá contra ellos.

## Decisión

### El formato

**CommonMark 0.31 + GFM** (tablas, listas de tareas, tachado, autoenlaces),
restringido al subconjunto del MVP:

| Permitido en E1 | No permitido |
| --- | --- |
| Encabezados `##`–`####` | `#` (lo aporta el título del recurso) |
| Párrafos, énfasis, código en línea | HTML crudo |
| Listas ordenadas, sin ordenar y de tareas | Definiciones, notas al pie |
| Bloques de código con lenguaje | Fórmulas (E2, `RO-03`) |
| Citas | Contenedores personalizados |
| Tablas GFM | |
| Enlaces y `![imagen]()` con destino interno `asset://{asset_id}` o URL https | `javascript:`, `data:` |
| Reglas horizontales | |

### Canónico significa normalizado al guardar

El servidor no guarda lo que le mandan: lo parsea con `goldmark`, lo valida y lo
**vuelve a serializar** con un renderizador de Markdown propio que fija las
elecciones ambiguas:

| Ambigüedad | Forma canónica |
| --- | --- |
| Énfasis | `*cursiva*`, `**negrita**` |
| Viñetas | `-` |
| Listas ordenadas | `1.` con numeración real |
| Encabezados | ATX (`##`), nunca subrayado |
| Código | vallado con ``` y lenguaje explícito |
| Regla horizontal | `---` |
| Fin de línea | `\n`, sin espacios finales, sin tabuladores |
| Líneas en blanco | exactamente una entre bloques |
| Escapado | mínimo necesario |

Guardar dos veces el mismo contenido produce **byte a byte lo mismo**, lo que
hace que el `ETag` sea estable y que el autosave no genere versiones falsas.

### Referencias a binarios

En el Markdown nunca hay URLs firmadas — caducan. Se escribe `asset://{asset_id}`
y la API resuelve a URL firmada **en el momento de servir el recurso**, tras
verificar el derecho de acceso (`CA-06`). El campo `content_md` se acompaña de
`asset_refs` (arreglo de `asset_id`) extraído al normalizar, para poder validar
en la publicación que todos los binarios referenciados están `ready` (`CA-01`).

### Renderizado y XSS

El servidor expone `content_md` (canónico) y, en el endpoint de previsualización,
`content_html` ya renderizado y **sanitizado con bluemonday** con una política de
lista blanca. El HTML crudo dentro del Markdown se descarta en el parseo, no en
la sanitización: dos barreras (`CE-02`).

### Autosave

`PATCH /resources/{id}/content` con `If-Match: <etag>`. Un `ETag` distinto
responde `412` con el contenido del servidor, para que el editor de E2 pueda
resolver el conflicto. El servidor guarda además las últimas 20 revisiones en
`authoring.content_revisions` (solo texto, barato), que es la "recuperación" que
pide `RF-04` y la base del historial de `RO-03`.

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| Guardar el AST en JSON | Sin ambigüedad; ideal para el editor | El enunciado dice Markdown | Requisito |
| Guardar HTML | Renderizado directo | Superficie de XSS enorme; difícil de versionar y de diferenciar | Seguridad |
| Markdown sin normalizar | Menos trabajo | El `ETag` cambia por espacios; el autosave produce ruido; la ida y vuelta no es comprobable | §11 pide justo lo contrario |
| Markdown puro sin GFM | Estándar estricto | Sin tablas ni listas de tareas, que el enunciado nombra | Insuficiente |

## Consecuencias

- Hay que escribir un renderizador de Markdown→Markdown sobre el AST de
  goldmark; son unas 400 líneas y es la pieza que el enunciado recomienda
  prototipar primero.
- La prueba de ida y vuelta (`normalizar(normalizar(x)) == normalizar(x)`) es
  bloqueante en CI, por tipo de nodo, como sugiere §11.
- El editor de E2 tendrá un contrato claro contra el que construirse.
- Contenido que use sintaxis fuera del subconjunto se rechaza con `422` señalando
  la línea, en lugar de guardarse y renderizarse a medias.

## Cómo se verifica

- Tabla de casos por tipo de nodo: entrada sucia → salida canónica esperada.
- Idempotencia del normalizador sobre un corpus de documentos.
- Intentar guardar `<script>alert(1)</script>` → se descarta en el parseo y no
  aparece en `content_html`.
- Guardar dos veces lo mismo → mismo `ETag`.
