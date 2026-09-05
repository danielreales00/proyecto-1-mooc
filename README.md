# Plataforma MOOC

Plataforma web de cursos masivos abiertos en línea. Proyecto del curso de
desarrollo de soluciones cloud (maestría). Backend en Go — monolito modular con
workers asíncronos independientes — sobre PostgreSQL, Redis y almacenamiento de
objetos, ejecutado íntegramente en Docker.

## Estado

**Entrega 1 en preparación.** Documentación de arquitectura completa. En
`feat/esqueleto-backend` hay una rebanada vertical del backend: registro de
estudiante → trabajo asíncrono → correo → verificación → login → `/me` →
revocación de sesión.

Para esta entrega, backend y workers van completos y funcionales; **no hay
frontend**, y la demostración se hace con Postman.

## Documentación

Todo está en [`arquitectura/`](arquitectura/README.md):

- [Enunciado](arquitectura/enunciado.md) y [aclaraciones del profesor](arquitectura/aclaraciones-profesor.md)
- [Matriz de requisitos](arquitectura/requisitos.md) y [alcance de la Entrega 1](arquitectura/alcance-entrega-1.md)
- [Decisiones de arquitectura (ADR)](arquitectura/adr/README.md)
- [Diseños](arquitectura/disenos/README.md): componentes, modelo de datos, API, trabajos asíncronos, estados, objetivos de servicio, ruta a GCP

## Puesta en marcha

```bash
make up          # levanta el stack completo y aplica migraciones
make seed        # datos sintéticos para la demostración
make test        # pruebas unitarias
make smoke       # recorre el flujo de punta a punta y verifica las condiciones
make scale       # 3 instancias de api y 3 de worker (CE-01)
```

`make help` lista el resto: `logs`, `jobs`, `audit`, `psql`, `cover`, `tidy`, `clean`.

| Servicio | URL |
| --- | --- |
| API (tras el proxy) | http://localhost:8090/readyz |
| Mailpit | http://localhost:8026 |
| MinIO | http://localhost:9011 |

Los puertos del host se configuran en `.env` y usan un rango propio para poder
convivir con otros stacks de Docker en la misma máquina.

Requiere Docker en marcha (`sudo service docker start` en WSL). **No hace falta
tener Go instalado**: se compila dentro de contenedores.

## Estructura prevista

```
deploy/           Caddyfile del proxy de entrada
backend/
  cmd/            api · worker · migrate · seed
  internal/
    platform/     config · logging · problem · httpx · ids · jobs · passwords · dbx
    adapters/     postgres · rediscli · sessions · objectstore · mailer · queue
    modules/      identity · audit          (authoring, media, assessment… pendientes)
  migrations/     SQL versionado, hacia adelante
  openapi/        openapi.yaml              (pendiente)
arquitectura/     ADRs, diseños, requisitos, pendientes
postman/          colección de la demostración (SEG-1 … SEG-9)
scripts/          smoke.sh
.github/          CI: build, lint, seguridad, migraciones y pruebas
docker-compose.yml · Makefile · .env.example
```

## Equipo

4 personas. Reparto de módulos en
[`arquitectura/disenos/api-v1.md`](arquitectura/disenos/api-v1.md).
