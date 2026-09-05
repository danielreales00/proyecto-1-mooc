# Colección de Postman

Es la **interfaz de la demostración** de la Entrega 1: el profesor relevó el
frontend y aceptó Postman como forma de interactuar con el sistema.

| Archivo | |
| --- | --- |
| `mooc.postman_collection.json` | La colección, una carpeta por segmento de la §10.2 del enunciado |
| `mooc.postman_environment.json` | Variables del entorno local |

## Uso

```bash
make up      # levanta el stack
make seed    # datos sintéticos (§10.1)
```

Importar ambos archivos en Postman, seleccionar el entorno **MOOC local** y
ejecutar las carpetas en orden con el Collection Runner.

## Estructura

Una carpeta por segmento, en el orden en que se graba el video:

| Carpeta | Estado |
| --- | --- |
| SEG-1 · Identidad y administración | Parcial — falta `admin` |
| SEG-2 · Autoría y publicación | Pendiente |
| SEG-3 · Carga multimedia | Pendiente |
| SEG-4 · Procesamiento y fallos | Pendiente |
| SEG-5 · Consumo de contenido | Pendiente |
| SEG-6 · Quiz | Pendiente |
| SEG-7 · Progreso y aprobación | Pendiente |
| SEG-8 · Insignia y actualización | Pendiente |
| SEG-9 · Operación | Parcial — el escalamiento ya está verificado |

**Las carpetas vacías se dejan a propósito.** Miden el avance contra la
demostración, que es como se califica, en lugar de contra el número de
endpoints implementados.

## Reglas

- Cada petición lleva **aserciones**, no solo la llamada. Una petición sin
  `pm.test` no acredita nada.
- Las variables encadenan las peticiones (`session_token`, `verify_token`): las
  carpetas se ejecutan en orden y no requieren copiar y pegar a mano.
- El correo de registro se genera con marca de tiempo en el script previo, para
  que la demo se pueda repetir sin limpiar la base.
- Cuando un módulo entre, su carpeta se llena en el mismo PR. Un requisito no
  está terminado si no aparece aquí (ver `arquitectura/alcance-entrega-1.md`).

## Equivalente en línea de órdenes

`scripts/smoke.sh` cubre el mismo recorrido de SEG-1 sin Postman, y es lo que
corre en CI. Cuando se añada una petición aquí, conviene reflejar la
comprobación allí.
