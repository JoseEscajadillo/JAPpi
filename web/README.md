# Dashboard de JAPpi (Next.js Multi-Zones)

Todavía vacío: es una tarea de la fase 2 (ver [roadmap](../docs/roadmap.md)). Estructura acordada en el [ADR-0005](../docs/adr/0005-nextjs-multi-zones-para-microfrontends.md):

```
web/
├── apps/
│   ├── shell/      /, /pricing, /docs   → reescribe /app/* y /billing/* hacia las otras zonas
│   ├── console/    /app/*               basePath: "/app"
│   └── billing/    /billing/*           basePath: "/billing"
├── packages/
│   ├── ui/         componentes compartidos
│   └── api-client/ tipos y cliente del control-plane
├── pnpm-workspace.yaml
└── turbo.json
```

Para crearlo:

```bash
cd web
pnpm dlx create-next-app@latest apps/shell --ts --app --eslint --tailwind --src-dir --import-alias "@/*"
# repetir para console y billing, y añadir basePath en su next.config.ts
```
