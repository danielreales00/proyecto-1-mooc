# Verificación funcional del frontend. Entrega 3

Recorridos mínimos de estudiante y profesor, la ruta de la interfaz que los
resuelve y la evidencia que los comprueba. La interfaz se sirve desde el grupo
de API, en el mismo origen que `/api/v1`. Su despliegue, la sesión y el CORS
están en el [documento de arquitectura](arquitectura.md), sección 8.

## Cómo se verifica

La evidencia es una prueba de navegador, `frontend/tests/e2e.mjs`. Usa Chrome
for Testing, que decodifica H.264 y AAC, así que la reproducción del video es
real. Recorre la interfaz como lo haría una persona y no llama a la API para
suplir ningún paso. Solo la usa directamente para comprobar que el servidor
rechaza lo que la interfaz no ofrece.

| Dónde | Orden | Cuándo corre |
| --- | --- | --- |
| Stack local | `make frontend-e2e` | En cada `make ci` y en cada ejecución del CI |
| Nube, por el balanceador | `make frontend-e2e-nube` | En cada sesión con el entorno encendido. Deja la salida y las capturas en [`evidencias/frontend/`](evidencias/frontend/) |

La prueba crea su propio curso con un nombre único, así que no depende de
datos preparados. Las cuentas de profesor y de estudiante sí son de la semilla.

## Recorrido conectado

1. `profesor@mooc.local` crea un curso con un módulo, una unidad, una lectura
   en Markdown, una imagen, un video y un quiz.
2. Carga la imagen y el video desde la biblioteca de medios. Espera a que el
   servidor los verifique, los escanee y transcodifique el video.
3. Valida la versión y la publica. La versión queda inmutable.
4. `estudiante2@mooc.local` encuentra el curso en el catálogo y se inscribe.
5. Lee la lección, ve la imagen y reproduce el video. La credencial de
   reproducción se renueva sin cortar el video.
6. Presenta el quiz, lo aprueba y recibe la insignia, que se verifica por su
   enlace público.

## Matriz del estudiante

| Flujo | Ruta de la interfaz | Evidencia en la prueba |
| --- | --- | --- |
| Iniciar sesión | `#/login` | `login("estudiante2@mooc.local")` llega a «Sigue avanzando» |
| Cerrar sesión | Botón «Salir» de la cabecera | `logout()` vuelve al inicio |
| Consultar el catálogo | `#/catalog` | Búsqueda por el nombre del curso recién publicado |
| Ver el detalle de un curso | `#/course/{slug}` | Enlace «Explorar curso» desde la tarjeta |
| Inscribirse | Botón «Inscribirme al curso» en `#/course/{slug}` | Abre el aula del curso |
| Acceder a sus cursos | `#/learning` | El aula abre con la primera lección |
| Navegar módulos, unidades y recursos | `#/learn/{inscripción}`, barra lateral | Se abren la lectura, la imagen, el video y el quiz |
| Consumir multimedia y documentos | `#/learn/{inscripción}` | La imagen carga con ancho mayor que cero. El video avanza más de 0,5 s sin error de reproducción |
| Consultar su progreso | Indicador de `#/learn/{inscripción}` y barra de `#/learning` | Tras la lección y el quiz, `#/learning` muestra un porcentaje mayor que cero, calculado por el servidor |
| Presentar un quiz | `#/attempt/{intento}` | Guarda respuestas, confirma la entrega |
| Consultar el resultado | `#/attempt/{intento}` tras entregar | «Objetivo alcanzado.» |

## Matriz del profesor

| Flujo | Ruta de la interfaz | Evidencia en la prueba |
| --- | --- | --- |
| Iniciar y cerrar sesión | `#/login` y botón «Salir» | `login("profesor@mooc.local")` y `logout()` |
| Consultar sus cursos | `#/studio` | El estudio lista los cursos del profesor y abre el recién creado |
| Crear y editar un curso | Formulario «Crear un curso» en `#/studio` y editor de la versión en `#/studio/{curso}/{versión}` | Curso con título, categoría y descripción, editado con su estructura y contenido |
| Estructura de módulos y unidades | `#/studio/{curso}/{versión}` | «Nuevo módulo» y «Nueva unidad» |
| Agregar recursos | «Agregar recurso» en cada unidad | Lectura, imagen, video y quiz |
| Cargar archivos y ver su estado | `#/media` | Carga por partes firmadas y consulta del estado hasta `ready` |
| Crear o editar un quiz | `#/quiz-editor/{recurso}` | Pregunta con dos opciones y la correcta marcada |
| Publicar cuando cumple las reglas | «Validar contenido» y «Publicar versión» | «Todo listo para publicar.» y versión inmutable |
| Consultar el resultado de la publicación | `#/studio/{curso}/{versión}` | La versión aparece como publicada, sin formularios de edición |

## Autorización

| Comprobación | Interfaz | Servidor |
| --- | --- | --- |
| Un estudiante no usa el estudio | `#/studio` muestra «Esta sección requiere otro rol» y el menú no ofrece «Estudio» | `POST /courses`, `PATCH /courses/{id}` y `POST …/publish` responden 403 |
| Una profesora no modifica un curso ajeno | El curso de otro profesor no aparece en su estudio | `PATCH /courses/{id}`, `POST /courses/{id}/versions` y `POST …/unpublish` responden 403 |

La interfaz oculta lo que el rol no permite. Quien decide es el servidor: las
rutas de autoría exigen rol de profesor o administrador y cada operación
comprueba que el curso sea del autor (`backend/internal/modules/authoring`).

## Estados de la interfaz

| Estado | Cómo se muestra |
| --- | --- |
| Carga | «Abriendo tu espacio» al navegar y «Cargando cursos» en el catálogo |
| Contenido vacío | Mensajes propios, por ejemplo «Todavía no tienes cursos» o «Este recurso todavía no tiene contenido» |
| Procesamiento | Progreso del hash y de cada parte al cargar, y el estado del archivo (`Recibido`, `En revisión`, `Procesando`, `Listo`) |
| Éxito | Avisos como «Recurso guardado.» o «Todo listo para publicar.» |
| Error | Bloque con el mensaje del servidor, el detalle por campo y la referencia de la traza |

Un archivo en procesamiento no se ofrece como disponible. La validación de la
versión lo rechaza con «aún no está disponible (estado …)», y sin versión
publicada el estudiante no ve el recurso.

## Resultados

| Fecha | Dónde | Commit | Resultado |
| --- | --- | --- | --- |
| 2026-10-25 | Stack local, `make frontend-e2e` | El de este documento | Pasa, sin errores en la consola del navegador |
