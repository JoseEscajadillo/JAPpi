# 0001. Registrar las decisiones de arquitectura con ADR

- **Estado:** Aceptado
- **Fecha:** 2026-09-29
- **Decisores:** equipo JAPpi

## Contexto

JAPpi es un proyecto de tres personas con avances semanales. Las decisiones se toman en chats y llamadas, y en pocas semanas nadie recuerda por qué se eligió una cosa u otra. Las personas que se sumen después (o un evaluador) necesitan entender el porqué sin reconstruir conversaciones.

## Decisión

Registraremos cada decisión de arquitectura relevante como un ADR en `docs/adr/`, con el formato de [`template.md`](template.md), revisado por PR como cualquier código.

## Alternativas consideradas

- **Wiki o Notion:** queda fuera del repo y deja de corresponder al código que describe.
- **Nada, "está en el código":** el código dice qué hace, no por qué ni qué se descartó.

## Consecuencias

- **Positivas:** historial de decisiones versionado junto al código; las discusiones se cierran por escrito.
- **Negativas:** cuesta unos minutos por decisión, y hay que tener la disciplina de no editar ADR aceptados.
