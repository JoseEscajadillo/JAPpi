# Hoja de ruta

Sin calendario semanal fijo: avanzamos por fases y cada fase termina con una **demo que funciona**. Los cambios de plan se anotan aquí.

**Leyenda:** ✅ hecho · 🔨 en curso · ⬜ pendiente

## Fase 0: cimientos ✅

- ✅ Monorepo, módulo Go y CI
- ✅ Contratos de eventos, bus NATS y bus en memoria, con suite de contrato
- ✅ Reglas hexagonales comprobadas en `go test` (`pkg/archtest`)
- ✅ ADR 0001–0010 y diagramas C4
- ✅ Detector de monorepos, cableado de variables, dominios reservados, redespliegue selectivo
- ✅ `github-integration`: webhook verificado → `repo.pushed`
- ✅ `control-plane`: `repo.pushed` → despliegues → `build.requested`

## Fase 1: push → URL con HTTPS 🔨

**Demo:** hacer push a un repo con una app Next.js y verla en `https://web-...jappi.app`.

| Tarea | Responsable |
|-------|-------------|
| Clúster K3s en el VPS: Traefik, cert-manager con wildcard, NATS, registry | Persona 1 |
| Adaptador Postgres del control-plane (`migrations/0001_init.sql` ya escrito) | Persona 2 |
| Adaptador `RepoSource` de GitHub (tarball del commit con el token de la instalación) | Persona 2 |
| `builder`: BuildKit + Railpack, con `BuildEnv` como build args | Persona 3 |
| `deployer`: reconciliador con client-go (server-side apply) + `deployment.status_changed` | Claude |
| Aislamiento base: namespace, ResourceQuota, NetworkPolicy, gVisor | Claude |

## Fase 2: operar lo desplegado ⬜

- Dashboard: zonas `shell` y `console` (Multi-Zones, ADR-0005)
- Logs en vivo (Vector → Loki → servicio `logs` → WebSocket)
- Historial de despliegues y rollback (`rollback.requested`)
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
- Copias de seguridad de Postgres (CloudNativePG → almacenamiento S3)
- Anti-abuso: límites de salida, bloqueo de SMTP, detección de minería
- Entornos de preview por PR
