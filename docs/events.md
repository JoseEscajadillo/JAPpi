# Catálogo de eventos

Contratos en [`pkg/contracts/events`](../pkg/contracts/events). Todos viajan dentro de un `Envelope` (`id`, `type`, `version`, `occurred_at`, `project_id`, `data`) por el stream `JAPPI` de NATS JetStream, en el subject `jappi.<type>`.

| Tipo | Publica | Consumen | Cuándo |
|------|---------|----------|--------|
| `repo.pushed` | github-integration | control-plane | Push verificado a una rama |
| `build.requested` | control-plane | builder | Por cada servicio afectado por un push, o por un redeploy manual |
| `build.succeeded` | builder | control-plane | Imagen subida al registry (por digest) |
| `build.failed` | builder | control-plane | El build no terminó |
| `deploy.requested` | control-plane | deployer | Tras un build correcto, en un rollback o en un redeploy. Lleva la especificación completa (ADR-0011) |
| `deployment.status_changed` | deployer | control-plane, logs | Cada transición: `deploying`, `healthy`, `failed` |
| `subscription.changed` | billing | control-plane, deployer | Alta, cambio de plan, impago, cancelación |

## Reglas

1. **Idempotencia.** Todo handler tolera recibir el mismo evento dos veces. Cuando el origen trae un ID estable, se reutiliza: `github-<X-GitHub-Delivery>`, `build-<deploymentID>`, `deploy-<deploymentID>`, `status-<deploymentID>-<estado>`.
2. **Compatibilidad.** Dentro de una `version` solo se **añaden** campos opcionales. Renombrar o quitar un campo exige una versión nueva, un periodo publicando ambas y un ADR.
3. **Nada de secretos** en los payloads. Se referencian (`SecretRef{name, key}`) y Kubernetes los inyecta en el pod desde un Secret del namespace. El nombre del Secret de un add-on sigue el contrato `events.AddonSecretName` (`jappi-addon-<addon>`).
4. **Orden.** No se garantiza el orden entre eventos. Los consumidores lo toleran: `AdvanceTo` ignora los estados viejos y un build viejo que termina tarde no pisa a uno nuevo.
5. **Errores.** Un handler devuelve `events.Permanent(err)` si reintentar no sirve (payload corrupto, proyecto borrado). Así el mensaje se descarta en vez de reintentarse 5 veces.
6. **Nombres.** `<entidad>.<verbo_en_pasado>`, en inglés y en minúsculas.

## Añadir un evento

1. Constante en `events.go` y payload en `payloads.go`, con un comentario que diga quién lo publica.
2. Fila en esta tabla.
3. Actualiza el diagrama C4 de contenedores si aparece una relación nueva.
