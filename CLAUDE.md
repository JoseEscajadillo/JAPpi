# JAPpi: guía para Claude Code

JAPpi es un PaaS (tipo Railway o Render) que despliega frontend + backend + PostgreSQL + Redis desde un repo, con detección automática de monorepos y cableado de variables entre servicios. Lo desarrolla un equipo de tres personas.

## Antes de escribir código

- Si la tarea añade o cambia un servicio, un caso de uso, un evento o una tabla, usa la skill **systems-design**.
- Para crear un servicio o un caso de uso, usa **hexagonal-service**. Para registrar una decisión, **new-adr**.
- Reglas de código: `CONTRIBUTING.md`. Decisiones: `docs/adr/`. Arquitectura: `docs/c4/README.md`. Fase actual: `docs/roadmap.md`.

## Reglas que no se negocian

- Arquitectura hexagonal por servicio: `domain` → `app` → `adapters`/`cmd`. `TestHexagonalRules` lo comprueba; no se desactiva.
- Entre servicios, solo eventos NATS (`pkg/contracts/events`). Los handlers son idempotentes.
- Los secretos nunca van en eventos, logs ni en el entorno de build.
- El código de los clientes no es de confianza: siempre en su namespace, con cuota, NetworkPolicy y gVisor.

## Comandos

```bash
gofmt -l . && go vet ./... && go test ./...
go run ./services/control-plane/cmd/jappi-detect <carpeta>
docker compose -f deploy/docker-compose.dev.yml up -d
```

Comentarios y documentación en español; identificadores en inglés.
