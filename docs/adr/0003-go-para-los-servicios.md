# 0003. Go para los servicios del backend

- **Estado:** Aceptado
- **Fecha:** 2026-09-29
- **Decisores:** equipo JAPpi

## Contexto

Los servicios de JAPpi hablan todo el día con Kubernetes, BuildKit, registries de contenedores y NATS. Necesitamos binarios pequeños que arranquen rápido en un clúster con poca memoria.

## Decisión

Todos los servicios del backend se escriben en **Go** (versión fijada en `go.mod`).

## Alternativas consideradas

- **TypeScript (Node):** un solo lenguaje con el frontend, pero los clientes de Kubernetes y BuildKit son de segunda, y el consumo de memoria por servicio es mayor.
- **Rust:** excelente rendimiento, pero la velocidad de desarrollo de un equipo de tres importa más aquí, y el ecosistema cloud-native es sobre todo Go.

## Consecuencias

- **Positivas:** client-go, BuildKit, NATS, cert-manager y k3s están escritos en Go: usamos sus librerías oficiales. Binarios estáticos de ~15 MB en imágenes `distroless`.
- **Negativas:** el equipo necesita aprender Go idiomático; el frontend va en otro lenguaje (TypeScript), así que los contratos compartidos con el dashboard se duplican (mitigado generando tipos TS desde los structs cuando haga falta).
