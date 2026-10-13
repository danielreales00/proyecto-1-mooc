# ADR-0017 — Interfaz web sobre la API existente

Estado: aceptado para la rama del frontend.

La plataforma necesita interfaces para estudiantes, docentes y administradores.
El frontend se sirve desde un contenedor Caddy independiente detrás del proxy
actual, en el mismo origen que `/api/v1` y `/docs`. HTML, CSS y módulos JS
nativos evitan un framework y una compilación adicional. Las sesiones opacas
se conservan solo en sessionStorage; roles y propiedad siguen validándose en
el servidor. Las versiones publicadas permanecen inmutables.

La reproducción usa hls.js 1.6.13 (Apache-2.0),
vendorizada con licencia en `frontend/vendor`. Resuelve la reproducción HLS
sobre MediaSource cuando el navegador admite H.264/AAC; el soporte HLS nativo
se usa como alternativa. La credencial y las URLs de segmentos se renuevan
antes de vencer, preservando la posición, para poder ver clases largas. Fuente: https://github.com/video-dev/hls.js.

El aula incluye el enlace externo validado durante la autoría y la existencia
de un archivo. El endpoint existente de media-sessions entrega también una
URL firmada temporal para archivos sin HLS; conserva la comprobación de
inscripción y de recurso visible, estado antimalware y host del almacén.
No se abre ningún bucket ni se añade una ruta de lectura anónima de originales.
Los porcentajes y las calificaciones siguen siendo exclusivamente del servidor.

El checksum de archivos usa js-sha256 0.11.1 (MIT), también vendorizado.
Permite calcular SHA-256 de forma incremental en bloques de 8 MiB, sin cargar
un archivo de hasta 5 GiB completo en la memoria del navegador. Fuente:
https://github.com/emn178/js-sha256. La verificación del servidor sigue siendo
la autoridad y reconoce firmas de imágenes y audio admitidos por su lista MIME.
