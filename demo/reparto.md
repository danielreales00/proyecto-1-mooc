# Reparto del guion entre cuatro

Divide `demo/guion.md` entre las cuatro personas del equipo. Los tiempos son
los del guion; suman **28 minutos** contra un objetivo de 25, así que hay que
recortar (ver «Cómo bajar de 25 minutos»).

**Criterio del reparto:** los bloques que exigen entender *por qué* está hecho
así van a Santiago; los que se sostienen leyendo la pantalla se reparten entre
Daniel, Compañero 3 y Compañero 4. Los bloques van **contiguos** por persona,
porque la demo es con estado: las variables de Postman encadenan las peticiones
y el curso que se publica en el bloque 3 es el que se consume en el 4.

## La tabla

| # | Bloque | Min | Quién | Por qué |
| --- | --- | --- | --- | --- |
| 0 | Apertura | 1 | **Daniel** | Texto escrito, sin ejecutar nada |
| 1 | Levantar el stack | 1,5 | **Daniel** | Dos órdenes y contar los contenedores |
| 1b | El contrato (Swagger) | 1 | **Daniel** | Pasear por la interfaz |
| 2 | Identidad | 3 | **Daniel** | Guionizado de punta a punta; el `202` y los dos niveles de límite están escritos |
| 3 | Autoría y publicación | 3,5 | **Compañero 3** | Ocho pasos en orden; el trigger de inmutabilidad se ve en la respuesta |
| 3b | Carga multimedia | 2,5 | **Compañero 3** | Seis pasos en terminal, más dos rechazos |
| 4 | Catálogo e inscripción | 1,5 | **Compañero 4** | Tres peticiones |
| 5 | Quiz | 2,5 | **Compañero 4** | El argumento de los dos tipos en Go está escrito y es memorable |
| 6 | Progreso y aprobación | 3 | **Santiago** | El bloque que más impresiona: señal fraudulenta, auditoría, cadencia con reloj del servidor |
| 7 | Insignia | 1,5 | **Santiago** | Es el desenlace del 6; separarlos rompe el hilo |
| 8 | Tolerancia a fallos | 2 | **Santiago** | El más delicado: outbox, reaper, idempotencia. Hay que saber leer el `intento 2` |
| 8b | Observabilidad y alerta | 2,5 | **Santiago** | Espera en vivo de ~2 min que hay que cubrir hablando |
| 9 | Operación | 1,5 | **Compañero 4** | `make demo` y la pestaña verde del CI |
| 10 | Cierre | 1 | **Santiago** | Es donde se dice qué falta; de ahí sale la primera pregunta del tribunal |

**Totales:** Daniel 6,5 · Compañero 3 6 · Compañero 4 5,5 · Santiago 10.

Santiago lleva más porque lleva lo crítico. Si hay que igualarlo, el bloque que
mejor se cede es el **7 (Insignia)**: son dos clics, aunque cueste el hilo
narrativo con el 6.

## Orden de intervención

```
Daniel ──────► Compañero 3 ──────► Compañero 4 ──────► Santiago ──► C4 ──► Santiago
 0,1,1b,2          3, 3b               4, 5           6,7,8,8b      9        10
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
| Compañero 3 | «¿Quién impide editar una versión publicada?» | `adr/0007` — un trigger de PostgreSQL, no código Go |
| Compañero 4 | «¿Cómo garantizan que la clave del quiz no se filtra?» | `adr/0013` — dos tipos distintos en Go, no un `omitempty` |
| Santiago | «¿Qué pasa si el worker ejecuta el mismo trabajo dos veces?» | `adr/0008` — claim atómico; se ejecuta, pero no produce dos salidas |

Los tres primeros deberían leer, además de su bloque, el apartado
[«Qué no prometer»](guion.md#qué-no-prometer) del guion. Es corto y evita la
única clase de error que cuesta caro: afirmar algo que hoy no es cierto.

## Cómo bajar de 25 minutos

Sobran 3 minutos. Por orden de menos daño:

1. **Bloque 8b, la espera de la DLQ (~2 min).** Inserta el trabajo que falla al
   **empezar el bloque 6**, no al llegar al 8b. Cuando lleguen, la alerta ya
   está en *firing* y solo hay que enseñarla. Es el recorte más rentable y no
   se pierde nada: los reintentos siguen quedando en los logs.
2. **Bloque 1b (1 min → 30 s).** Enseñar que la API sirve su propio contrato y
   desplegar una operación. Sin recorrer secciones.
3. **Bloque 4 (1,5 → 1 min).** Tres peticiones seguidas, sin comentar cada una.
4. **Bloque 3, el `psql` opcional.** Ya está marcado como opcional en el guion.

No recortes los tiempos de espera del worker en los bloques 2, 6 y 7: son la
prueba de que el procesamiento es asíncrono.
