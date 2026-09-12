# Reparto del guion

Quién presenta cada bloque de `demo/guion.md`.

**El video tiene un máximo de 20 minutos.** Los bloques suman **19,25**, así que
quedan 45 segundos de margen: cada minuto que uno se pasa lo paga el siguiente.

## La tabla

| # | Bloque | Min | Quién |
| --- | --- | --- | --- |
| 0 | Apertura | 0,5 | **David** |
| 1 | Levantar el stack | 1 | **David** |
| 2 | Identidad | 2 | **David** |
| 2b | Administración | 1 | **David** |
| 3 | Autoría y publicación | 2 | **Michael** |
| 3b | Carga multimedia | 1,5 | **Michael** |
| 4 | Catálogo, inscripción y consumo | 1 | **Michael** |
| 5 | Quiz | 1,5 | **Michael** |
| 6 | Progreso y aprobación | 2,5 | **Santiago** |
| 7 | Insignia | 1 | **Santiago** |
| 8 | Tolerancia a fallos | 1,5 | **Santiago** |
| 8b | Observabilidad y la alerta | 2 | **Daniel** |
| 9 | Operación | 1 | **Daniel** |
| 10 | Cierre | 0,75 | **Santiago** |

**Totales:** David 4,5 · Michael 6 · Santiago 5,75 · Daniel 3.

Los bloques van **contiguos** por persona porque la demo tiene estado: las
variables de Postman encadenan las peticiones, y el curso que se publica en el
bloque 3 es el que se consume en el 4.

## Orden de intervención

```
David ──────────► Michael ──────────► Santiago ────► Daniel ──► Santiago
 0, 1, 2, 2b       3, 3b, 4, 5          6, 7, 8       8b, 9        10
   4,5 min           6 min              5 min         3 min       0,75
```

Cuatro relevos. Cada uno abre nombrando lo que va a demostrar, no diciendo
«hola, ahora yo». Una frase de relevo evita los silencios: «con el curso
publicado, te paso el recorrido del estudiante».

## Cómo grabarlo

**Una sola máquina, una sola toma: la de Santiago**, donde el stack está
configurado y ensayado. Los demás narran su bloque mientras él conduce el
teclado, o toman el control por compartición de pantalla.

Es la opción de menos riesgo porque la demo tiene estado. Si cada uno graba en
su portátil, el curso, la inscripción y el intento de quiz son otros, y el hilo
«este es el curso que acabamos de publicar» se rompe.

**Si hace falta que cada uno opere su propia máquina**, entonces:

- Cada persona corre `make up && make seed && make demo` en verde en su equipo
  **al menos un día antes**. Si `make demo` no pasa, no se graba.
- Los bloques 0–2b, 8 y 9 funcionan sobre un stack recién sembrado, sin depender
  de nadie. El 8b necesita el trabajo lanzado unos 3,5 minutos antes: quien lo
  grabe aparte tiene que lanzarlo él mismo con la orden del bloque 6.
- Los bloques 3 a 7 son una cadena. Quien tenga el 4 y el 5 necesita un curso
  publicado: que lo cree con `make demo` antes de grabar, o que el bloque 3 le
  pase el `course_id`.

**El trabajo que muere para el bloque 8b se lanza al empezar el bloque 6**, en
la terminal y sin comentarlo, como dice el guion. Tarda unos 3,5 minutos en
llegar a la cola de muertos, que es justo lo que duran los bloques 6, 7 y 8: al
llegar al 8b la alerta ya está en *firing*. Lo lanza quien presenta el 6, no
quien presenta el 8b.

## Una pregunta por bloque

Cada quien debería poder responder esto sobre lo suyo sin mirar el guion:

| Bloque | La pregunta | Dónde está la respuesta |
| --- | --- | --- |
| 2 | «¿Por qué la API responde `202` y no `201` al registrar?» | `adr/0008` — patrón outbox: el trabajo se registra en la misma transacción y se publica tras el commit |
| 2b | «¿Por qué comprobar el último administrador dentro de la transacción?» | Fuera de ella, dos peticiones simultáneas degradan a los dos últimos y el sistema queda cerrado. Por eso va con `FOR UPDATE` |
| 3 | «¿Quién impide editar una versión publicada?» | `adr/0007` — un trigger de PostgreSQL, no código Go |
| 5 | «¿Cómo garantizan que la clave del quiz no se filtra?» | `adr/0013` — dos tipos distintos en Go, no un `omitempty` |
| 8 | «¿Qué pasa si el worker ejecuta el mismo trabajo dos veces?» | `adr/0008` — claim atómico: se ejecuta, pero no produce dos salidas |
| 8b | «¿Por qué la alerta no se queda muda la primera vez?» | Los contadores se declaran en cero al arrancar el worker; sin eso `increase()` no ve el salto de una serie que no existía a su primer valor |
| 9 | «¿Qué cubre `make demo`?» | 33 aserciones contra el sistema en ejecución, el mismo recorrido del video |

Conviene que todos lean, además de su bloque, el apartado
[«Qué no prometer»](guion.md#qué-no-prometer) del guion. Es corto y evita la
única clase de error que cuesta caro: afirmar algo que hoy no es cierto.

## Si se van de tiempo

El margen es de 45 segundos, así que no hay colchón. Por orden de qué sacrificar
primero:

1. **Bloque 2b entero.** Es el único que no sostiene una condición de aceptación
   del enunciado. Queda demostrado en la colección de Postman, que se entrega
   igual, y mencionado en el cierre.
2. **Bloque 7 a 30 s.** Solo la verificación pública, sin comentar el listado.
3. **Bloque 4 a 30 s.** Las cuatro peticiones de corrido, sin comentar.
4. **Bloque 3b a 1 min.** Dejar corriendo `make subir` y narrar solo la
   verificación del worker, sin el negativo.

Lo que **no** se toca, en ningún caso: el bloque 6 (progreso y la señal
fraudulenta), el 8 (idempotencia) y la alerta del 8b. Son condiciones de
aceptación explícitas —CA-03, CA-05, CE-06— y renunciar a ellas cuesta más que
pasarse de tiempo.

## Antes de grabar

Cronometren un ensayo completo **con las cuatro voces**. La estimación por
bloque es eso, una estimación: lo único que dice si caben en 20 minutos es un
ensayo con reloj. Si sale en 24, el recorte se decide entre todos y no en medio
de la grabación.
