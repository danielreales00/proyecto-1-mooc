# Aclaraciones del profesor

Precisiones recibidas fuera del PDF. **Prevalecen sobre el enunciado** cuando hay
discrepancia. Cada entrada indica a qué entrega aplica.

---

## Entrega 1 — alcance esperado

> Quisiera hacer una precisión sobre el alcance esperado para esta entrega del
> proyecto.
>
> Para esta etapa, se espera que tanto el **backend** como la **capa de workers**
> estén **completamente desarrollados y funcionales**. **No es necesario
> implementar todavía el frontend**.
>
> Para las pruebas y la demostración en el video, pueden utilizar **Postman**
> temporalmente como interfaz de interacción con el sistema. En la demostración
> deberán evidenciar el **flujo de trabajo completo** y el funcionamiento de los
> componentes desarrollados.
>
> La implementación deberá realizarse en **Go** y todos los componentes de la
> solución deberán estar **empaquetados y ejecutarse mediante Docker**.

### Cómo lo leemos

| El profesor dice | Consecuencia para nosotros |
| --- | --- |
| Backend y workers completos y funcionales | El alcance funcional del enunciado §5.1 se implementa **entero del lado servidor**. No se recorta. |
| No hace falta frontend | No se escribe UI. Los requisitos que solo existen en la UI (editor de bloques, visor PDF, navegación por teclado, WCAG 2.2 AA) se posponen; **su contraparte de API sí se implementa**. |
| Postman como interfaz | Se versiona una **colección de Postman** que recorre los nueve segmentos de la demostración (§10.2), en lugar de guiones manuales. Es un entregable de esta entrega. |
| Flujo de trabajo completo en el video | La demo debe recorrer el camino entero: registro → autoría → publicación → inscripción → carga multimedia → procesamiento → consumo → quiz → progreso → insignia. |
| Go y todo en Docker | Ya estaba en el enunciado (§7). Se refuerza: **ningún componente corre fuera de contenedores**, tampoco en desarrollo. |

### Lo que esta aclaración *no* releva

Sigue vigente todo lo que no depende del frontend:

- Idempotencia, reintentos con backoff y DLQ en los workers (§6).
- Control de acceso por rol, propiedad e inscripción (§6).
- Versiones publicadas inmutables e `stable_id` (§3).
- Clave del quiz exclusiva del servidor (§6).
- Progreso calculado en servidor y rechazo de porcentajes del cliente (§6).
- Escalamiento a múltiples instancias de API y worker (§4).
- Observabilidad, auditoría y pruebas de los flujos críticos (§5.1).

Ver `alcance-entrega-1.md` para el desglose fino de qué entra y qué no.

---

## Entrega 1 — duración del video

El video tiene un máximo de **20 minutos**.

Confirmado por el profesor el 12 de septiembre de 2026, fuera del PDF. El
enunciado no fija duración, así que este límite manda.

**Consecuencia directa:** el guion completo de `demo/guion.md` dura 30,5
minutos, así que hay que grabar la versión recortada. El presupuesto por bloque
está en el propio guion, en «Presupuesto de 20 minutos», y el reparto entre las
cuatro personas en `demo/reparto.md`.

Pasarse del límite es el único error de esta entrega que no se puede arreglar
argumentando: o cabe, o no cabe.
