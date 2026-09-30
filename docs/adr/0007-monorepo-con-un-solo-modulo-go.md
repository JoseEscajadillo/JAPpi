# 0007. Monorepo con un solo módulo Go

- **Estado:** Aceptado
- **Fecha:** 2026-09-29
- **Decisores:** equipo JAPpi

## Contexto

Tenemos varios microservicios, contratos de eventos compartidos y un frontend. Con tres personas, coordinar versiones entre repos separados costaría más que lo que aporta.

## Decisión

- **Un repositorio** con todo: `services/`, `pkg/`, `web/`, `deploy/` y `docs/`.
- **Un solo módulo Go** (`go.mod` en la raíz). Cada servicio compila a su propio binario e imagen.
- Los servicios no pueden importar el `internal/` de otro servicio (lo impide el compilador). Lo compartido va en `pkg/`, y ahí solo entran contratos y adaptadores genéricos, nunca lógica de negocio.
- La CI construye y despliega solo los servicios afectados por un cambio.

## Alternativas consideradas

- **Un repo por servicio:** independencia total, pero cada cambio de contrato exige PRs coordinados en varios repos.
- **Monorepo con un módulo por servicio (`go.work`):** versiones de dependencias independientes, pero más fricción (replace, sincronizar go.sum) sin un beneficio real a este tamaño.

## Consecuencias

- **Positivas:** un cambio de contrato y todos sus consumidores van en el mismo PR; una sola CI; refactors atómicos.
- **Negativas:** todas las dependencias Go comparten versión; hay que vigilar que `pkg/` no se convierta en un cajón de sastre.
