# Preguntas al profesor

Dudas abiertas y sus respuestas. Cuando llega una respuesta, se copia a
`aclaraciones-profesor.md` y se marca aquí como resuelta.

## Abiertas

### 1. Objetivos de latencia y disponibilidad
El enunciado exige "objetivos de latencia y disponibilidad" documentados (§9) y
que la prueba de carga no presente incumplimientos críticos (§10), pero no fija
umbrales. Propusimos los nuestros en `disenos/objetivos-de-servicio.md`
(p95 ≤ 250 ms en lectura autenticada, ≤ 800 ms en escritura compleja, 99,5 %
mensual). **¿Hay umbrales esperados, o los fija cada equipo?**

### 2. Alcance de la prueba de carga en la Entrega 1
La prueba de carga de Etapa 1 (§10) apunta a 2.000 concurrentes. Sin frontend, la
carga se genera contra la API con k6. **¿Se espera alcanzar los 2.000 concurrentes
en esta entrega, o basta demostrar el método y el escalamiento horizontal?**

### 3. Escaneo antimalware
`RF-05` pide escaneo antimalware. Planeamos ClamAV en contenedor, con EICAR como
caso de prueba. **¿Es suficiente, o se espera integración con un servicio
externo?**

### 4. CDN sin nube
`RF-06` pide distribución por CDN. En Docker Compose no hay CDN real; serviríamos
desde MinIO con cabeceras de caché y dejaríamos Cloud CDN para la entrega de GCP.
**¿Se acepta esa sustitución en la Entrega 1?**

### 5. Cobertura de los segmentos que dependen del frontend
Sin frontend, SEG-5 (navegación por teclado, visor PDF) y la auditoría WCAG 2.2
AA no se pueden demostrar. **¿Se posponen íntegros a la entrega del frontend, o
se espera alguna evidencia sustituta en esta?**

### 6. Duración y formato del video
**¿Cuánto debe durar la demostración y qué formato de entrega se espera?**

### 7. Repositorio y entrega
**¿La entrega es un repositorio por equipo, un tag, o un archivo comprimido?
¿Hay alguna organización de GitHub donde deba vivir?**

### 8. Datos sintéticos
La demostración se ejecuta con datos sintéticos (§10.1). **¿Hay un conjunto
provisto, o cada equipo genera el suyo?**

## Resueltas

### Alcance de la Entrega 1 — resuelta
**Pregunta:** ¿qué se espera para la Entrega 1?
**Respuesta:** backend y capa de workers completos y funcionales; frontend no;
Postman como interfaz para pruebas y demostración; Go; todo empaquetado y
ejecutado con Docker. Texto completo en `aclaraciones-profesor.md`.
