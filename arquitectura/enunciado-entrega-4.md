# Enunciado de la Entrega 4 — Despliegue en Cloud Run y Cloud Pub/Sub

Transcripción fiel de `2026-20 Entrega 4 - Cloud Run y Pub-Sub.pdf`, para consultarla sin
abrir el PDF. **Ante cualquier diferencia, manda el PDF.** Nuestra lectura está
en [`alcance-entrega-4.md`](alcance-entrega-4.md).

Desplegar la aplicación de las entregas anteriores sobre Cloud Run e integrar
Cloud Pub/Sub para ejecutar tareas asíncronas y comunicar el backend con los
workers. Se parte de la base de código y las funcionalidades existentes.

## 1. Despliegue y escalamiento

Desplegar **tres servicios independientes en Cloud Run**, con imágenes
publicadas en Artifact Registry.

| Servicio | Responsabilidad | Máximo de instancias configurado |
| --- | --- | --- |
| Frontend | Servir la interfaz de la aplicación. | 3 |
| Backend | Exponer la API y publicar tareas en Pub/Sub. | 3 |
| Workers | Recibir mensajes y ejecutar tareas asíncronas. | 3 |

Configurar el escalamiento automático con **máximo de 3 instancias por
servicio**, con un contenedor de aplicación por instancia. No se exige mantener
tres instancias activas permanentemente.

Documentar región, mínimo de instancias, CPU, memoria, concurrencia y tiempo
máximo de solicitud de cada servicio. Las réplicas deben operar sin depender
del almacenamiento local para persistir información.

Conectar el frontend con el backend desplegado, configurar CORS cuando
corresponda y conservar la autenticación existente. Backend y workers deben
acceder a Cloud SQL y Cloud Storage según sus responsabilidades.

> El requisito se verifica sobre el límite configurado. Cloud Run puede
> superarlo temporalmente, según su documentación de escalamiento.

### Arquitectura general

Frontend → (API) → Backend → (publica tareas) → Pub/Sub → (push autenticado) →
Workers. Backend y workers acceden a Cloud SQL y Cloud Storage.

Frontend, backend y workers en Cloud Run. Máximo configurado de 3 instancias
por servicio. Pub/Sub conecta el backend con los workers mediante un topic y
una suscripción push.

## 2. Tareas asíncronas mediante Cloud Pub/Sub

Sustituir el mecanismo de mensajería previo por un topic de Pub/Sub y una
**suscripción push autenticada** dirigida al servicio de workers.

El backend debe publicar tareas y responder sin esperar su ejecución. Pub/Sub
entregará cada mensaje a un worker, que ejecutará la tarea y persistirá su
estado y resultado según corresponda. La aplicación debe permitir verificar su
finalización.

Confirmar los mensajes después de completar y persistir el procesamiento.
Configurar tiempos compatibles y tolerar reentregas sin duplicar los efectos de
una tarea.

## 3. Configuración y acceso

- Utilizar cuentas de servicio con permisos mínimos para los recursos
  requeridos.
- Restringir la invocación de workers mediante IAM y autorizar la identidad de
  la suscripción push.
- Gestionar configuración mediante variables de entorno y secretos mediante
  Secret Manager, sin claves ni contraseñas en el repositorio o las imágenes.

## 4. Entregables y verificación

Entregar en el repositorio:

- **Guía reproducible de despliegue**, con comandos o scripts, imágenes,
  configuración de servicios, topic, suscripción y permisos.
- **Diagrama actualizado**, con los componentes desplegados y sus conexiones.
- **Evidencias de funcionamiento**, con URL de frontend y backend, límites de
  instancias y registros que permitan seguir una tarea desde su publicación
  hasta su finalización.
- **Verificación breve del escalamiento**, con métricas del número de
  instancias ante solicitudes concurrentes y mensajes pendientes, utilizando una
  carga controlada.
- **Video de sustentación de máximo 20 minutos**, con enlace accesible para el
  evaluador. Debe demostrar la aplicación desplegada, el recorrido completo de
  una tarea por Pub/Sub y los workers, la configuración y evidencia de
  escalamiento, y las decisiones de acceso y permisos.

## 5. Rúbrica de evaluación

La calificación suma 0 a 100 puntos. Se asigna uno de los tres puntajes por
criterio según las evidencias.

| Criterio | Cumplimiento completo | Cumplimiento parcial | Sin cumplimiento |
| --- | --- | --- | --- |
| Despliegue e integración (30) | 30. Tres servicios operativos e integrados con sus dependencias | 15. Despliegue con fallas parciales de integración | 0. Aplicación no operativa en Cloud Run |
| Pub/Sub y workers (25) | 25. Flujo completo, autenticado, con confirmación y reentregas sin efectos duplicados | 12. Ejecución demostrada; confirmación o reentregas incompletas | 0. Sin ejecución demostrada mediante Pub/Sub |
| Escalamiento (20) | 20. Máximo de 3 por servicio y escalamiento demostrado con métricas | 10. Límites correctos; verificación incompleta | 0. Límites incorrectos o sin evidencia |
| Configuración y acceso (15) | 15. Permisos mínimos, workers protegidos y secretos gestionados correctamente | 7. Controles incompletos, sin credenciales expuestas | 0. Credenciales expuestas o workers sin protección |
| Documentación y video (10) | 10. Guía reproducible, diagrama coherente y video de hasta 20 minutos con las demostraciones exigidas | 5. Entregables incompletos o video mayor a 20 minutos | 0. Sin documentación ni video verificables |
