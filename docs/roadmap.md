# Hoja de ruta

Sin calendario semanal fijo: avanzamos por fases y cada fase termina con una **demo que funciona**. Los cambios de plan se anotan aquí.

**Leyenda:** ✅ hecho · 🔨 en curso · ⬜ pendiente

## Fase 0: cimientos ✅

- ✅ Monorepo, módulo Go y CI
- ✅ Contratos de eventos, bus NATS y bus en memoria, con suite de contrato
- ✅ Reglas hexagonales comprobadas en `go test` (`pkg/archtest`)
- ✅ ADR 0001–0010 y diagramas C4 (fase 1 añadió los ADR 0011–0012)
- ✅ Detector de monorepos, cableado de variables, dominios reservados, redespliegue selectivo
- ✅ `github-integration`: webhook verificado → `repo.pushed`
- ✅ `control-plane`: `repo.pushed` → despliegues → `build.requested`

## Fase 1: push → URL con HTTPS 🔨

**Demo:** hacer push a un repo con una app Next.js y verla en `https://web-...jappi.app`.

| Tarea | Responsable |
|-------|-------------|
| Elegir VPS y región (ver las preguntas abiertas de `system-design.md`) y comprar el dominio | Equipo |
| Clúster K3s en el VPS: Traefik, cert-manager con wildcard, NATS, registry (guía en `deploy/k8s/README.md`) | Persona 1 |
| Adaptador Postgres del control-plane (`migrations/0001_init.sql` ya escrito) | Persona 2 |
| Adaptador `RepoSource` de GitHub (tarball del commit con el token de la instalación) | Persona 2 |
| `builder`: BuildKit + Railpack, con `BuildEnv` como build args | Persona 3 |
| ✅ `deployer`: server-side apply + vigilancia del rollout + `deployment.status_changed` (ADR-0011) | Claude |
| ✅ Aislamiento base: PSA restricted, cuotas por plan, NetworkPolicy, UID fijo, gVisor (ADR-0012) | Claude |
| ✅ control-plane: `build.succeeded` → `deploy.requested`; historial con `superseded` | Claude |
| ✅ Workspace: `scripts/dev.sh`, clúster k3d, Dockerfile, CI con kind e imágenes | Claude |
| ✅ System Design: capacidad, cuellos de botella, SLO y costes (`docs/system-design.md`) | Claude |
| Publicar las imágenes en GHCR desde la CI al hacer merge a `main` | Persona 1 |

## Fase 2: operar lo desplegado ⬜

- Dashboard: zonas `shell` y `console` (Multi-Zones, ADR-0005)
- Logs en vivo (Vector → Loki → servicio `logs` → WebSocket)
- Historial de despliegues y rollback (un `deploy.requested` con la imagen anterior, ADR-0011), con **outbox transaccional** para los comandos del dashboard
- Variables secretas del usuario hasta el clúster (pendiente del ADR-0011)
- Reconciliador de pushes perdidos (cuello de botella 5 del System Design)
- Variables de entorno desde el dashboard (con la marca de *build*)
- Reinicios y redeploy manual

## Fase 3: el diferencial completo ⬜

- Servicio `addons`: Postgres con CloudNativePG, Redis y credenciales en Secrets
- Pantalla "Importar repositorio" con la vista previa de `AnalyzeRepository`, editable antes de confirmar
- Seguir las dependencias `workspace:*` (Prisma en `packages/db`)
- Volver a analizar en cada push que toque manifiestos (package.json, go.mod...)

## Fase 4: SaaS ⬜

- Servicio `billing`: Stripe Checkout, prueba de 30 días (ADR-0010), webhooks
- Planes → ResourceQuota ($5 / $12 / $20)
- Zona `billing` del dashboard y portal del cliente de Stripe

## Fase 5: producción seria ⬜

- Métricas por servicio (Prometheus) y alertas
- K3s en alta disponibilidad (3 servidores)
- Repartir `deploy.requested` en shards por servicio para tener varias réplicas del deployer (cuello de botella 3)
- `topologySpreadConstraints` para las apps con 2 o más réplicas
- Dormir las apps hobby inactivas y Postgres compartido para hobby (cuellos de botella 1 y 2)
- Copias de seguridad de Postgres (CloudNativePG → almacenamiento S3)
- Anti-abuso: límites de salida, bloqueo de SMTP, detección de minería
- Entornos de preview por PR
