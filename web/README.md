# Frontend de JAPpi

Todavía vacío. El [Sprint 3](../docs/roadmap.md) inicia una sola aplicación Next.js modular. Diego prepara la base técnica los días 1–3; César y José conectan el flujo mínimo cuando la API funcione. El contrato de integración es [`docs/api.md`](../docs/api.md); no se fijará una UI de producto antes de validar el recorrido.

```
web/
├── src/app/                   rutas y composición Next.js
├── src/features/auth/         sesión y acceso
├── src/features/projects/     importación y detalle
├── src/features/deployments/  estado del despliegue
└── src/shared/                cliente HTTP, tipos y componentes comunes
```

Una app, un build y un despliegue. [ADR-0005](../docs/adr/0005-nextjs-multi-zones-para-microfrontends.md) registra la decisión anterior; [ADR-0013](../docs/adr/0013-monolito-modular-hasta-validar-el-producto.md) propone posponer Multi-Zones hasta la etapa de escala.
