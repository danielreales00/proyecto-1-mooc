# ADR-0006 — Sesiones opacas en Redis, no JWT de sesión

- **Estado:** Aceptado
- **Fecha:** 2026-09-05
- **Requisitos:** `RF-01`, `RF-02`, `CA-06`, `CE-02`

## Contexto

El enunciado exige **sesiones revocables** y **revocación inmediata** (§5.1,
§10.2 SEG-1), y asigna a Redis el soporte de sesiones (§4). Un JWT
autocontenido no se puede revocar antes de que expire sin mantener, de todos
modos, una lista de revocación en un almacén compartido — es decir, sin volver a
Redis.

## Decisión

**Token de sesión opaco.** 32 bytes aleatorios de `crypto/rand`, codificados en
base64url. Se entrega al cliente; en Redis se guarda **su SHA-256**, nunca el
token.

- Clave: `sess:{sha256(token)}` → hash con `user_id`, `role`, `created_at`,
  `last_seen_at`, `ip`, `user_agent`. TTL deslizante de 12 h, tope absoluto 30 días.
- Índice inverso `user_sessions:{user_id}` (set) para listar y revocar todas las
  sesiones de un usuario de un golpe (`RF-02`).
- Espejo en `identity.sessions` (PostgreSQL) para el listado administrativo y la
  auditoría. **Redis manda sobre la validez**; PostgreSQL guarda la historia.
- Revocar = borrar la clave de Redis. Efecto inmediato en la siguiente petición,
  sin ventana de gracia.
- Transporte: cabecera `Authorization: Bearer <token>`. Cuando llegue el frontend
  (E2) se añadirá cookie `HttpOnly; Secure; SameSite=Lax` con doble envío de
  token CSRF; hoy, con Postman, la cabecera basta y no hay superficie CSRF.
- Contraseñas con **Argon2id** (`m=64 MiB, t=3, p=2`), sal por usuario.
- Rate limiting en Redis con ventana deslizante: `5/min` por IP en `login`, y
  `3/hora` por correo en recuperación de contraseña.

**Dónde sí se usa JWT:** en tokens de un solo propósito y corta vida que viajan
por correo o por URL — verificación de correo (24 h) y restablecimiento de
contraseña (1 h) — firmados con HS256 y clave del entorno. Ahí no hace falta
revocar caso por caso, y se invalidan al consumirse.

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| JWT de acceso + refresh | Sin lectura de Redis por petición | Revocación inmediata imposible sin lista de revocación (que ya es Redis) | El enunciado exige revocación inmediata |
| Sesión solo en PostgreSQL | Una sola fuente de verdad | Una escritura por petición para el `last_seen`; Redis existe justamente para esto | §4 asigna sesiones a Redis |
| Token opaco sin hashear en Redis | Más simple | Un volcado de Redis entrega sesiones válidas | Coste nulo hashear |

## Consecuencias

- Una lectura de Redis por petición autenticada. Es una operación de sub-ms y
  Redis ya está en la ruta por el rate limiting.
- La API sigue sin estado: el estado vive en Redis, no en el proceso, así que
  `--scale api=3` funciona sin sesiones pegajosas.
- Si Redis cae, todos quedan deslogueados. Aceptable: `appendonly yes` y la
  sesión se puede rehacer con login.

## Cómo se verifica

- SEG-1: iniciar sesión en dos clientes, revocar una desde administración, la
  petición siguiente de ese cliente responde `401` sin esperar expiración.
- El token no aparece en claro en `redis-cli --scan` ni en los logs.
- Seis intentos de login en un minuto → `429`.
