# Enunciado de la Entrega 5 — Funciones serverless e infraestructura como código

Transcripción fiel de `2026-20 Entrega 5 - Serverless y Terraform.pdf`, para consultarla
sin abrir el PDF. **Ante cualquier diferencia, manda el PDF.** Nuestra lectura
está en [`alcance-entrega-5.md`](alcance-entrega-5.md).

Identificar oportunidades de uso de servicios serverless, implementar al menos
un cambio en la arquitectura y automatizar su despliegue con Terraform.

## 1. Análisis y cambio mínimo

Identificar qué componentes ya son serverless y qué otros podrían adoptar este
modelo. Documentar cada oportunidad con el componente, servicio propuesto,
beneficio y limitaciones. **Las alternativas adicionales solo deben
documentarse.**

**Migrar al menos una tarea del worker a una Cloud Function de segunda
generación (Cloud Run functions)**, activada mediante Cloud Pub/Sub. Justificar
la elección y conservar el comportamiento y resultado de la tarea.

Ajustar la integración para que el worker anterior deje de ejecutar la tarea
migrada. Mantener el manejo de reintentos sin duplicar sus efectos.

## 2. Despliegue automatizado con Terraform

Automatizar el despliegue de la arquitectura resultante. Terraform debe
gestionar:

- Los servicios de **Cloud Run**, sus imágenes, configuración y escalamiento.
- Los topics de **Cloud Pub/Sub**, las suscripciones y la integración con sus
  consumidores.
- La **Cloud Function**, su código desplegable y el disparador de eventos.
- Las cuentas de servicio, permisos, referencias a secretos y demás
  dependencias necesarias.

Parametrizar el entorno y documentar los prerrequisitos, la preparación de
imágenes y código, y el manejo de recursos existentes.

El despliegue y sus actualizaciones deben ejecutarse con `terraform init`,
`terraform plan` y `terraform apply`, sin configurar manualmente los recursos
incluidos en el alcance. No almacenar credenciales, secretos ni archivos de
estado en el repositorio.

### Componente serverless

Backend en Cloud Run → Pub/Sub → Eventarc → Cloud Function → Cloud SQL y Cloud
Storage.

El backend publica la tarea y el evento activa la función que reemplaza su
ejecución en el worker. Cloud SQL y Cloud Storage se utilizan según la tarea.
Terraform configura los recursos y sus permisos.

## 3. Entregables y verificación

Entregar en el repositorio:

- **Análisis y diagrama actualizado**, con las oportunidades identificadas y el
  cambio implementado.
- **Código de la función y archivos de Terraform**, con instrucciones
  reproducibles de despliegue.
- **Evidencias del despliegue con Terraform** y de una tarea real procesada
  desde Pub/Sub hasta la función y su resultado.
- **Video de sustentación de máximo 20 minutos**, con enlace accesible para el
  evaluador. Debe explicar la migración y demostrar el despliegue con Terraform
  y la ejecución de una tarea mediante Pub/Sub y la función.

## 4. Rúbrica de evaluación

La calificación suma 0 a 100 puntos. Se asigna uno de los tres puntajes por
criterio según las evidencias.

| Criterio | Cumplimiento completo | Cumplimiento parcial | Sin cumplimiento |
| --- | --- | --- | --- |
| Análisis serverless (15) | 15. Compara servicios, beneficios y límites. Justifica la tarea elegida | 7. Identifica alternativas, pero la justificación está incompleta | 0. No presenta análisis de oportunidades |
| Migración a la función (25) | 25. Tarea operativa desde Pub/Sub, resultado correcto y reintentos sin duplicados | 12. Tarea operativa. Migración o reintentos incompletos | 0. No demuestra una tarea migrada y operativa |
| Automatización con Terraform (35) | 35. Despliega y actualiza Cloud Run, Pub/Sub, función y dependencias de forma reproducible | 17. Despliegue parcial. Requiere configuración manual de recursos | 0. No demuestra un despliegue mediante Terraform |
| Configuración y acceso (15) | 15. Entorno parametrizado, permisos mínimos y secretos y estado protegidos | 7. Controles o parametrización incompletos, sin credenciales expuestas | 0. Credenciales expuestas o accesos necesarios sin protección |
| Documentación y verificación (10) | 10. Documentación, evidencias y video de hasta 20 minutos con las demostraciones exigidas | 5. Entregables incompletos o video mayor a 20 minutos | 0. Sin documentación, evidencias ni video verificables |
