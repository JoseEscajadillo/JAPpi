# JAPpi: guía para Claude Code

JAPpi es un PaaS (tipo Railway o Render) que despliega frontend + backend + PostgreSQL + Redis desde un repo, con detección automática de monorepos y cableado de variables entre servicios. Lo desarrolla un equipo de tres personas.

## Antes de escribir código

- Si la tarea añade o cambia un servicio, un caso de uso, un evento o una tabla, usa la skill **systems-design**.
- Para crear un servicio o un caso de uso, usa **hexagonal-service**. Para registrar una decisión, **new-adr**.
- Reglas de código: `CONTRIBUTING.md`. Decisiones: `docs/adr/0013-monolito-modular-hasta-validar-el-producto.md`. Arquitectura: `docs/c4/README.md`. Contrato REST: `docs/api.md`. Entorno: `docs/development.md`. Sprint actual: `docs/roadmap.md`.

## Reglas que no se negocian

- Arquitectura hexagonal por módulo: `domain` → `app` → `adapters`/`cmd`. La prueba de arquitectura actual no se desactiva durante la migración.
- Etapa 1: una API Go y una app Next.js modulares. Los módulos Go se llaman directamente; los trabajos se persisten en PostgreSQL. El código NATS actual se conserva solo hasta migrar su comportamiento con pruebas.
- Los secretos nunca van en eventos, logs ni en el entorno de build.
- El código de los clientes no es de confianza: siempre en su namespace, con cuota, NetworkPolicy y gVisor.

## Comandos

```bash
./scripts/dev.sh check          # gofmt + vet + pruebas
./scripts/dev.sh detect <dir>   # qué detectaría JAPpi en un repo
./scripts/dev.sh infra-up       # NATS + Postgres
./scripts/dev.sh cluster-up     # K3s local (k3d)
```

Comentarios y documentación en español; identificadores en inglés.
