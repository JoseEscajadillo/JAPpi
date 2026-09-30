# 0011. Deployer sin estado, alimentado por deploy.requested

- **Estado:** Aceptado
- **Fecha:** 2026-09-30
- **Decisores:** equipo JAPpi

## Contexto

Para poner una imagen en producción, el deployer necesita saber:

- el puerto, el dominio y el plan del servicio;
- las variables cableadas y las referencias a secretos.

Todo eso vive en el control-plane. El evento `build.succeeded` solo lleva la imagen.

Había dos caminos:

- que el deployer consulte al control-plane (una llamada HTTP entre servicios, prohibida por el ADR-0002);
- que el deployer mantenga su propia copia de proyectos y servicios (datos duplicados que se desincronizan).

El rollback planteaba lo mismo: el evento `rollback.requested` obligaba al deployer a recordar despliegues anteriores.

## Decisión

- **El dueño de los datos arma la orden completa.** El control-plane consume `build.succeeded`, arma la especificación y publica **`deploy.requested`** con todo lo necesario: imagen fijada por digest, puerto, dominio, plan y variables. Los literales van tal cual y los secretos, como referencias `SecretRef`.
- **El deployer no guarda estado.** Traduce la especificación a objetos de Kubernetes, los aplica con server-side apply, vigila el rollout y publica `deployment.status_changed` con los estados `deploying`, `healthy` o `failed`.
- **El rollback es un despliegue más.** El control-plane publica `deploy.requested` con la imagen de un despliegue anterior. Se elimina `rollback.requested`.
- **Las imágenes se referencian siempre por digest.** Una etiqueta como `:latest` puede cambiar por debajo, y el rollback dejaría de ser reproducible.
- **Todo es idempotente:**
  - el evento `deploy.requested` lleva el ID `deploy-<deploymentID>`;
  - cada aviso de estado lleva el ID `status-<deploymentID>-<estado>`;
  - aplicar dos veces el mismo estado deseado no cambia nada.
- **Nunca se pisa un despliegue más nuevo.** Si un build viejo termina después que uno nuevo que ya está desplegándose o sirviendo, el control-plane marca el viejo como `superseded` y no lo despliega.

## Alternativas consideradas

- **El deployer consulta al control-plane por HTTP:** acopla la disponibilidad de los dos servicios y rompe el ADR-0002.
- **El deployer mantiene una proyección propia de los proyectos:** más eventos, datos duplicados y más formas de desincronizarse, sin ganar nada.
- **Mantener `rollback.requested`:** obliga al deployer a guardar historial, que ya vive en el control-plane.

## Consecuencias

- **Positivas:** el deployer escala horizontalmente sin coordinar estado; un rollback usa exactamente el mismo camino, ya probado, que un despliegue normal; el control-plane es la única fuente de verdad.
- **Negativas:**
  - El evento `deploy.requested` es más grande: unos pocos KB, irrelevante para NATS.
  - **Las variables definidas por el usuario todavía no viajan**, porque pueden ser secretas y un evento no debe llevarlas. Hay que decidir cómo llegan al clúster (un Secret que escriba el control-plane o cifrado de sobre); queda para la fase 2.
  - Con varias réplicas del deployer, dos despliegues del mismo servicio podrían aplicarse a la vez. Hoy se serializan dentro de una réplica (`KeyedMutex`); con más réplicas hará falta partir los subjects por servicio o un candado distribuido (ver `docs/system-design.md`).
