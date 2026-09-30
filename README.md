# JAPpi

**Plataforma como servicio para desplegar aplicaciones completas.** Conecta un repositorio de GitHub y JAPpi construye, despliega y opera tu frontend, tu backend, PostgreSQL y Redis desde un solo dashboard: CI/CD, logs, redeploy en cada push, historial y rollback.

**Lo que nos diferencia:** JAPpi entiende los monorepos. Detecta qué carpeta es el frontend y cuál el backend, y **conecta los servicios solo**:

- el frontend recibe la URL del backend (incluso las variables `NEXT_PUBLIC_*` que se incrustan en el build);
- el backend recibe el origen del frontend para CORS y sus conexiones a Postgres y Redis;
- todas las variables usan los nombres que tu código ya lee.

```bash
$ go run ./services/control-plane/cmd/jappi-detect services/control-plane/testdata/acme-monorepo
# → web (next) y api (express + prisma + ioredis) detectados
# → web recibe NEXT_PUBLIC_API_URL=https://api-...; api recibe CORS_ORIGIN=https://web-..., DATABASE_URL y REDIS_URL
```

## Estado

Fase 1 en curso: el deployer ya lleva una imagen a Kubernetes con aislamiento por proyecto; faltan el builder y el VPS. Ver la [hoja de ruta](docs/roadmap.md).

## Arquitectura

- **Microservicios en Go** comunicados por eventos sobre **NATS JetStream**.
- **Arquitectura hexagonal** en cada servicio, con las reglas comprobadas en las pruebas.
- **K3s sobre VPS** como plano de datos: un namespace aislado por proyecto.
- **Dashboard en Next.js** con Multi-Zones (microfrontends).

| Documento | Contenido |
|-----------|-----------|
| [docs/system-design.md](docs/system-design.md) | System Design: capacidad, cuellos de botella, escalado, SLO y costes por plan |
| [docs/c4](docs/c4/README.md) | Diagramas C4: contexto, contenedores, componentes, despliegue y flujo de un push |
| [docs/adr](docs/adr/README.md) | Decisiones de arquitectura y sus motivos |
| [docs/events.md](docs/events.md) | Catálogo de eventos entre servicios |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Cómo trabajamos: hexagonal, SOLID, pruebas, Git, seguridad |
| [docs/development.md](docs/development.md) | Entorno de desarrollo: herramientas, `scripts/dev.sh`, clúster local |

## Arranque rápido

```bash
./scripts/dev.sh check        # gofmt + vet + pruebas, sin dependencias
./scripts/dev.sh infra-up     # NATS + Postgres
./scripts/dev.sh cluster-up   # K3s local con k3d
```

## Planes

Un mes de prueba y después $5, $12 o $20 al mes, según recursos, límites y funcionalidades.
