# Colección de Postman

Es la **interfaz de la demostración** de la Entrega 1: el profesor relevó el
frontend y aceptó Postman como forma de interactuar con el sistema.

| Archivo | |
| --- | --- |
| `mooc.postman_collection.json` | La colección, una carpeta por segmento de la §10.2 del enunciado |
| `mooc.postman_environment.json` | Variables del entorno local |
| `archivo-de-prueba.mp4` | 256 KiB deterministas para SEG-3: cabe en una parte y su SHA-256 es fijo, que es lo que `POST /assets/init` exige por adelantado |

## Uso

```bash
make up      # levanta el stack
make seed    # datos sintéticos (§10.1)
```

Importar ambos archivos en Postman, seleccionar el entorno **MOOC local** y
ejecutar las carpetas en orden con el Collection Runner. Ver «Cómo importar».

## Cómo importar

En Postman: botón **Import** (arriba a la izquierda, junto a *New*) y arrastrar
los dos `.json`. Después, en el desplegable de arriba a la derecha, elegir el
entorno **MOOC local**; sin eso `{{base_url}}` queda vacío y todo falla con un
error de DNS.

**Si Postman corre en Windows y el repositorio está en WSL**, los archivos no
están en el disco de Windows. En el diálogo de *Import*, pegar esta ruta en la
casilla del nombre de archivo:

```
\\wsl.localhost\Ubuntu\home\schica\cloud-development\proyecto-1-mooc\postman
```

Ese mismo camino sirve para el `archivo-de-prueba.mp4` de SEG-3. Para que
Postman lo acepte conviene apuntar ahí su directorio de trabajo:
*Settings → General → Working directory*. Si no, hay que activar en esa misma
pantalla la opción de leer archivos fuera del directorio de trabajo.

Alternativa si la ruta UNC da problemas: copiar la carpeta al disco de Windows
con `cp -r postman /mnt/c/Users/<tu-usuario>/Desktop/` e importar desde allí.
El inconveniente es que esa copia no se actualiza sola cuando cambia la
colección.

Sin Postman, la colección también se ejecuta desde la línea de órdenes:

```bash
make postman           # segmentos rápidos; es lo que corre el CI
make postman-completo  # todo, con 11 s entre peticiones (~8 min)
```

**Por qué hay dos.** El servidor exige 10 s entre heartbeats a propósito
(ADR-0012): inundar de heartbeats no debe simular permanencia. Ejecutar la
colección entera respetando esa regla tarda varios minutos, así que el CI corre
los segmentos que no dependen del tiempo, y el recorrido completo automático es
`make demo`.

## Estructura

Una carpeta por segmento, en el orden en que se graba el video:

| Carpeta | Estado |
| --- | --- |
| SEG-1 · Identidad y administración | **Completa** — falta `admin` |
| SEG-2 · Autoría y publicación | **Completa** |
| SEG-3 · Carga multimedia | **Completa** — un paso manual: elegir el archivo en «2 · Subir la parte 1» |
| SEG-4 · Procesamiento y fallos | Se demuestra con `make demo` |
| SEG-5 · Catálogo, inscripción y consumo | **Completa** |
| SEG-6 · Quiz | **Completa** |
| SEG-7 · Progreso y aprobación | **Completa** — necesita 10 s entre heartbeats |
| SEG-8 · Insignia | **Completa** — depende de que SEG-7 llegue a aprobada |
| SEG-9 · Operación | `make demo` y `make scale` |

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
- La lectura del correo en Mailpit **se reintenta a sí misma** hasta 20 veces:
  el worker es asíncrono y la API respondió 202 sin esperarlo. Por eso
  `make postman` pasa `--delay-request 300`.
- Cuando un módulo entre, su carpeta se llena en el mismo PR. Un requisito no
  está terminado si no aparece aquí (ver `arquitectura/alcance-entrega-1.md`).

## Equivalente en línea de órdenes

`scripts/smoke.sh` cubre el mismo recorrido de SEG-1 sin Postman, y es lo que
corre en CI. Cuando se añada una petición aquí, conviene reflejar la
comprobación allí.
