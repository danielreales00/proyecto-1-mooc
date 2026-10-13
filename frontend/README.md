# Interfaz web MOOC

El frontend vive en la rama `feat/complete-frontend`. Sigue `DESIGN.md`:
canvas blanco, tipografía Space Grotesk, superficies verde oscuro y piedra,
acentos coral, botones en píldora y listas separadas por reglas. No contiene
cursos, progreso, usuarios ni métricas simuladas.

## Ejecutar

Desde la raíz: `make up`. Abre http://localhost:8090.
La API conserva `/api/v1`, la documentación `/docs`, y los controles de salud
sus rutas originales. El proxy sirve la interfaz y la API en el mismo origen.
En Apple Silicon, Compose ejecuta ClamAV como amd64; el resto puede usar ARM64.
`make seed` carga las cuentas sintéticas ya definidas en el backend; sus
credenciales están documentadas en el código de la semilla.

No hay build JS. `frontend/Dockerfile` copia los archivos estáticos y los assets
vendorizados (Space Grotesk, hls.js y js-sha256 con sus licencias).
`make publicar` incluye la imagen frontend; la composición de producción y
Terraform incluyen el servicio y CORS de los buckets. Esos cambios de nube no
se aplican automáticamente desde esta rama.

## Recorridos

- Catálogo público con búsqueda, idioma, categoría y paginación; temario e inscripción.
- Registro, verificación por código de correo, login y cierre de sesión.
- Mis cursos, aula, lectura Markdown saneada por el servidor, recursos externos,
  documentos, imágenes y video/audio HLS. Evidencias open/heartbeat/close con
  permanencia visible; el servidor calcula el progreso.
- Evaluaciones con opciones simples/múltiples, guardado, confirmación de entrega,
  historial, continuidad de intentos y resultados según la política del servidor.
- Insignias, verificación pública y enlace compartible.
- Estudio docente: metadatos, versiones, módulos, unidades, recursos, Markdown
  con ETag, editor de preguntas y respuestas, vista previa, validación y publicación.
- Carga multipart firmada, reanudación seleccionando el archivo original y su ID,
  consulta de estado, lectura de archivo y cancelación.
- Administración de usuarios, invitaciones docentes, roles/estados, sesiones,
  auditoría, trabajos y reencolado, revocación motivada de insignias.

La auditoría encuentra 65 rutas implementadas y 25 operaciones planificadas.
La tabla completa está en [ENDPOINTS.md](ENDPOINTS.md).
Operaciones planificadas (como recuperación de contraseña) se consultan en
`/docs`; no se muestran como funciones disponibles.

## Verificar

`make frontend-test` prueba el contrato del cliente HTTP. `make frontend-e2e`
prueba el navegador contra el stack y crea un curso sintético: autoría,
publicación, inscripción, lectura, quiz, insignia y administración. Requiere
`make seed` y los workers activos. `make rapido`, `make contrato` y `make openapi`
verifican las extensiones del backend y el contrato.

La prueba usa Chrome for Testing 155.0.8059.39, fijado en su Dockerfile, para
verificar H.264/AAC reales, además de la interfaz. Fuente de los binarios:
https://googlechromelabs.github.io/chrome-for-testing/.

La prueba de navegador crea puentes TCP dentro de su contenedor hacia proxy,
MinIO y Mailpit: conserva los hosts de las URLs firmadas locales. Usa los
puertos por defecto de `.env.example`. La fixture MP4 fue generada con FFmpeg
(color de prueba y tono, tres segundos), sin contenido de terceros.

Las dependencias npm son exclusivamente de desarrollo. Nunca se incluyen
node_modules en la imagen servida. Los tokens se guardan por pestaña en
sessionStorage, se eliminan al salir o recibir 401 y no viajan al almacén de objetos.
