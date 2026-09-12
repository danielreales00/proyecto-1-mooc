# Reparto del guion entre cuatro

Divide `demo/guion.md` entre las cuatro personas del equipo.

**El video tiene un máximo de 20 minutos.** Los bloques suman **19,25**, así que
quedan 45 segundos de margen: nadie puede improvisar de más, porque cada minuto
que uno se pasa lo paga el siguiente.

**Criterio del reparto:** los bloques que exigen entender *por qué* está hecho
así van a Santiago; los que se sostienen leyendo la pantalla se reparten entre
Daniel, Compañero 3 y Compañero 4. Los bloques van **contiguos** por persona,
porque la demo es con estado: las variables de Postman encadenan las peticiones
y el curso que se publica en el bloque 3 es el que se consume en el 4.

## La tabla

| # | Bloque | Min | Quién | Por qué |
| --- | --- | --- | --- | --- |
| 0 | Apertura | 0,5 | **Daniel** | Texto escrito, sin ejecutar nada |
| 1 | Levantar el stack | 1 | **Daniel** | Dos órdenes y contar los contenedores |
| 2 | Identidad | 2 | **Daniel** | Guionizado de punta a punta; el `202` y el `429` están escritos |
| 2b | Administración | 1 | **Daniel** | Solo dos peticiones: el `409` del último administrador y el `403` del profesor |
| 3 | Autoría y publicación | 2 | **Compañero 3** | Pasos en orden; el trigger de inmutabilidad se ve en la respuesta |
| 3b | Carga multimedia | 1,5 | **Compañero 3** | Declarar, subir, verificado y un rechazo |
| 4 | Catálogo e inscripción | 1 | **Compañero 4** | Tres peticiones |
| 5 | Quiz | 1,5 | **Compañero 4** | El argumento de los dos tipos en Go está escrito y es memorable |
| 6 | Progreso y aprobación | 2,5 | **Santiago** | El bloque que más pesa: señal fraudulenta, auditoría, cadencia con reloj del servidor |
| 7 | Insignia | 1 | **Santiago** | Es el desenlace del 6; separarlos rompe el hilo |
| 8 | Tolerancia a fallos | 1,5 | **Santiago** | El más delicado: outbox, reaper, idempotencia. Hay que saber leer el `intento 2` |
| 8b | Observabilidad y alerta | 2 | **Santiago** | La alerta de la DLQ y el reencolado por la API de administración |
| 9 | Operación | 1 | **Compañero 4** | `make demo` y la pestaña verde del CI |
| 10 | Cierre | 0,75 | **Santiago** | Es donde se dice qué falta; de ahí sale la primera pregunta del tribunal |

El bloque **1b (el contrato en `/docs`)** no entra: se menciona en una frase
durante la apertura.

**Totales:** Daniel 4,5 · Compañero 3 3,5 · Compañero 4 3,5 · Santiago 7,75.

Santiago lleva el 40 % porque lleva lo crítico. Si quieren igualarlo, el bloque
que mejor se cede es el **7 (Insignia)**: es un clic y una URL en el navegador,
aunque cueste el hilo narrativo con el 6.

## Orden de intervención

```
Daniel ────────► Compañero 3 ──────► Compañero 4 ──────► Santiago ──► C4 ──► Santiago
 0, 1, 2, 2b         3, 3b               4, 5           6,7,8,8b      9        10
   4,5 min           3,5 min            2,5 min          7 min      1 min    0,75
```

Cinco relevos. El penúltimo (bloque 9) devuelve la palabra a Santiago para el
cierre a propósito: quien reconoce los huecos es quien puede defenderlos.

**Frase de relevo**, para que no haya silencios raros: «con el curso publicado,
te paso a ti el recorrido del estudiante». Cada uno abre nombrando lo que va a
demostrar, no diciendo «hola, ahora yo».

## Cómo grabarlo

**Recomendación: una sola máquina, una sola toma, la de Santiago.** Docker está
configurado y ensayado ahí. Los demás narran su bloque mientras Santiago
conduce el teclado, o toman el control por compartición de pantalla.

Es la opción de menos riesgo por una razón concreta: la demo tiene estado. Si
cada uno graba en su portátil, el curso, la inscripción y el intento de quiz son
otros, y el hilo «este es el curso que acabamos de publicar» se rompe.

**Si el profesor exige que cada uno opere su propia máquina**, entonces:

- Cada persona corre `make up && make seed && make demo` en verde en su equipo
  **al menos un día antes**. Si `make demo` no pasa, no se graba.
- Los bloques 0–1b, 2, 8, 8b y 9 funcionan sobre un stack recién sembrado, sin
  depender de nadie.
- Los bloques 3 a 7 son una cadena. Quien tenga el 4–5 necesita un curso
  publicado: que lo cree con `make demo` antes de grabar, o que el bloque 3 le
  pase el `course_id`.

## Qué tiene que saber cada uno de su bloque

No hace falta que nadie entienda el sistema entero, pero sí que cada uno pueda
responder **una** pregunta sobre lo suyo sin mirar el guion:

| Quién | La pregunta que le van a hacer | Dónde está la respuesta |
| --- | --- | --- |
| Daniel | «¿Por qué la API responde `202` y no `201` al registrar?» | `adr/0008` — patrón outbox: el trabajo se registra en la misma transacción y se publica tras el commit |
| Daniel (2b) | «¿Por qué comprobar el último administrador dentro de la transacción?» | Fuera de ella, dos peticiones simultáneas degradan a los dos últimos y el sistema queda cerrado. Por eso va con `FOR UPDATE` |
| Compañero 3 | «¿Quién impide editar una versión publicada?» | `adr/0007` — un trigger de PostgreSQL, no código Go |
| Compañero 4 | «¿Cómo garantizan que la clave del quiz no se filtra?» | `adr/0013` — dos tipos distintos en Go, no un `omitempty` |
| Santiago | «¿Qué pasa si el worker ejecuta el mismo trabajo dos veces?» | `adr/0008` — claim atómico; se ejecuta, pero no produce dos salidas |

Los tres primeros deberían leer, además de su bloque, el apartado
[«Qué no prometer»](guion.md#qué-no-prometer) del guion. Es corto y evita la
única clase de error que cuesta caro: afirmar algo que hoy no es cierto.

## Si se van de tiempo

El margen es de 45 segundos, así que no hay colchón. Por orden de qué sacrificar
primero:

1. **Bloque 2b entero.** Es lo último que entró y lo único que no sostiene una
   condición de aceptación del enunciado. Queda demostrado en la colección de
   Postman, que se entrega igual, y mencionado en el cierre.
2. **Bloque 7 (Insignia) a 30 s.** Solo la URL pública, sin el listado.
3. **Bloque 4 a 30 s.** Catálogo e inscripción de corrido, sin comentar.
4. **Bloque 3b a 1 min.** Dejar corriendo `make subir` y narrar solo la
   verificación del worker, sin el negativo.

Lo que **no** se toca, en ningún caso: el bloque 6 (progreso y la señal
fraudulenta), el 8 (idempotencia) y la alerta del 8b. Son condiciones de
aceptación explícitas —CA-03, CA-05, CE-06— y renunciar a ellas cuesta más que
pasarse de tiempo.

## Antes de repartir nada

Cronometren un ensayo completo **con las cuatro voces** antes de grabar. La
estimación por bloque es eso, una estimación: lo único que dice si caben en 20
minutos es un ensayo con reloj. Si el ensayo sale en 24, el recorte se decide
entre todos, no en medio de la grabación.
