# Arquitectura

Documentación de arquitectura del proyecto. Se lee en este orden.

## Qué hay que construir

| Documento | |
| --- | --- |
| [enunciado.md](enunciado.md) | Transcripción fiel del PDF del profesor. Fuente de verdad funcional |
| [aclaraciones-profesor.md](aclaraciones-profesor.md) | Precisiones posteriores. **Prevalecen sobre el enunciado** |
| [requisitos.md](requisitos.md) | Matriz con IDs trazables (`RF-`, `RO-`, `CA-`, `RT-`, `CE-`, `SEG-`, `RNF-`) |
| [alcance-entrega-1.md](alcance-entrega-1.md) | Qué entra y qué no en esta entrega |

## Por qué se construye así

| | |
| --- | --- |
| [adr/](adr/README.md) | 14 decisiones de arquitectura, una por archivo |

## Cómo está construido

| | |
| --- | --- |
| [disenos/](disenos/README.md) | Componentes, modelo de datos, API, trabajos asíncronos, máquinas de estado, objetivos de servicio, ruta a GCP |

## Trabajo en curso

| | |
| --- | --- |
| [pendientes.md](pendientes.md) | Tareas, decisiones abiertas y riesgos. No es entregable |
| [preguntas-profesor.md](preguntas-profesor.md) | Dudas abiertas y respuestas |

## Cómo se cita

Todo requisito tiene un ID en `requisitos.md`. Se usa en ADRs, diseños, issues,
nombres de pruebas y mensajes de commit:

```
feat(media): carga multipart reanudable a 24 h (RF-05, RNF-07)
```

Así, cuando el profesor pregunte dónde está implementado el escaneo antimalware,
la respuesta es `git log --grep=RF-05`.

## Reglas de estos documentos

1. **Un ADR no se reescribe.** Se marca *Reemplazado por ADR-NNNN* y se escribe
   otro. La historia de las decisiones es parte de la sustentación.
2. **Los diseños describen el presente.** Si un diseño se está justificando, esa
   justificación pertenece a un ADR.
3. **El enunciado se transcribe, no se interpreta.** Las interpretaciones van en
   `requisitos.md` o en un ADR, donde se ven y se pueden discutir.
4. **Lo que no se puede demostrar, no cuenta** (§10 del enunciado). Cada ADR
   cierra con "cómo se verifica" por esa razón.
