# Plataforma MOOC

Plataforma web de cursos masivos abiertos en línea. Proyecto del curso de
desarrollo de soluciones cloud (maestría). Backend en Go — monolito modular con
workers asíncronos independientes — sobre PostgreSQL, Redis y almacenamiento de
objetos, ejecutado íntegramente en Docker.

## Estado

**Entrega 1 en preparación.** Documentación de arquitectura completa; el código
todavía no empieza.

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
cp .env.example .env
make up          # levanta el stack completo
make seed        # datos sintéticos
make smoke       # verificación de extremo a extremo
```

> Aún no implementado. Ver [`arquitectura/pendientes.md`](arquitectura/pendientes.md).

## Estructura prevista

```
backend/          monolito modular en Go (cmd/api, cmd/worker, cmd/migrate)
  internal/       platform · adapters · modules
  migrations/     SQL versionado
  openapi/        openapi.yaml
arquitectura/     ADRs, diseños, requisitos, pendientes
postman/          colección de la demostración (SEG-1 … SEG-9)
scripts/          smoke, carga, restauración
docker-compose.yml
```

## Equipo

4 personas. Reparto de módulos en
[`arquitectura/disenos/api-v1.md`](arquitectura/disenos/api-v1.md).
