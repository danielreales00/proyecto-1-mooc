# Guía de informes de arquitectura

Convenciones de todo informe de arquitectura del proyecto. Rigen de la entrega 1
a la 5.

## Destinatario y tono

El informe se evalúa contra los siete criterios de la sección 9 del enunciado.
Cada sección debe poder leerse con esos criterios a la vista. Prosa directa. Sin
relleno ni lenguaje de folleto.

## Reglas de escritura

- Español. Identificadores, nombres de tablas y nombres de paquetes en inglés.
- Frases cortas. Una idea por frase.
- Ninguna frase repite con otras palabras algo ya dicho.
- Queda prohibido el patrón «afirmación breve: explicación». Se reescribe como
  oración completa. Los dos puntos solo introducen listas, definiciones formales
  o citas.
- Queda prohibido el contraste retórico del tipo «es un requisito, no una
  opción». La afirmación se enuncia de forma directa y neutra.
- Sin emojis. Sin rayas ni guiones largos. Sin punto y coma.
- No se citan los identificadores de segmento de la colección de Postman.
- El informe no cita el enunciado ni al profesor. Requisitos, restricciones y
  alcance se presentan como del proyecto.

## Estructura

Plantilla arc42, doce secciones, en su orden: 1 Introducción y objetivos,
2 Restricciones, 3 Contexto y alcance, 4 Estrategia de solución, 5 Vista de
bloques, 6 Vista de ejecución, 7 Vista de despliegue, 8 Conceptos transversales,
9 Decisiones de arquitectura, 10 Requisitos de calidad, 11 Riesgos y deuda
técnica, 12 Glosario.

El informe abre con el título y los nombres del equipo. La sección 1 incluye el
alcance de la entrega. Una sección que no aplique
conserva su título y una línea que explica la
omisión. No se agregan secciones fuera de la plantilla.

## Precedencia de fuentes

Aclaración del profesor, enunciado, ADR, código, en ese orden. Cuando el código
contradice un ADR, el informe lo señala en la sección 11 y nombra los dos
archivos. La contradicción nunca se calla.

## Regla de evidencia

Toda afirmación apunta a una orden que la comprueba, a un archivo o a una
prueba. Una afirmación sin ese respaldo se borra.

Lo diseñado y no implementado va a la sección 11. No cuenta como entregado.

## Cómo se verifican los números

Los números se miden en el código. No se copian de la documentación.

| Número | Dónde se mide |
| --- | --- |
| Operaciones de la API | `backend/openapi/openapi.yaml`. Las que llevan `x-estado: planificado` no están implementadas |
| Servicios en ejecución | `docker-compose.yml` |
| Aserciones que pasan | La salida de `make demo`, `make postman` y `make invariantes` |

Si lo medido no coincide con otro documento del repositorio, manda lo medido.
La diferencia se corrige en ese documento y no se lista en el informe.

## Diagramas

Mermaid, con notación C4 en los niveles 1 y 2, dibujada con `flowchart` porque el
renderizador C4 de mermaid cruza las líneas. Cada caja lleva nombre, tipo entre
corchetes y descripción. Solo se dibuja lo que existe en el código. Cada
diagrama va seguido de una explicación en prosa de sus cajas y relaciones.

## Formato y salida

El informe se escribe en Markdown. El PDF sale de `make informe ENTREGA=N`, que
convierte cada diagrama con mermaid-cli y arma el documento con pandoc. El
Markdown es el original. El PDF nunca se edita a mano.

Restricciones para que la conversión no se rompa:

- Sin HTML incrustado.
- Encabezados hasta el nivel 3.
- El título del informe es el único encabezado de nivel 1.
- Cada sección arc42 abre con un encabezado de nivel 2.
- El PDF no lleva índice ni saltos de página entre secciones.
- Tablas de seis columnas como máximo.
- Un diagrama por bloque de código mermaid.
- Imágenes por ruta relativa dentro de `arquitectura/`.

## Nomenclatura y continuidad

Un archivo por entrega, `arquitectura/informe-entrega-N.md`.

Un informe publicado no se reescribe. Desde la entrega 2, el informe abre con
una sección breve que dice qué cambió desde el anterior y por qué.

## Extensión

Máximo 2.700 palabras, sin contar el código de los diagramas.
