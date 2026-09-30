# 0012. Aislamiento de los proyectos en el clúster

- **Estado:** Aceptado
- **Fecha:** 2026-09-30
- **Decisores:** equipo JAPpi

## Contexto

JAPpi ejecuta código de desconocidos en máquinas compartidas. Un proyecto no puede:

- leer ni atacar a otro proyecto;
- escapar al nodo;
- llegar a la red interna ni al servicio de metadatos del proveedor;
- acaparar CPU o memoria;
- usar nuestra IP para enviar spam o minar criptomonedas.

Además, los planes de $5, $12 y $20 se diferencian por recursos, y esos límites se tienen que cumplir de verdad.

## Decisión

Cada proyecto vive en un namespace `prj-<id>` que el deployer crea y mantiene. Las decisiones viven en `services/deployer/internal/domain` y las prueba `go test`.

| Capa | Medida |
|------|--------|
| **Admisión** | Pod Security Admission en modo `restricted` (enforce, audit y warn): el API server rechaza pods privilegiados, con root o con capacidades extra, aunque alguien se salte el deployer. La prueba de integración lo verifica contra un clúster real. |
| **Proceso** | UID fijo 10001 con GID 0, `allowPrivilegeEscalation: false`, se quitan todas las capabilities, seccomp `RuntimeDefault`, sin token de la API de Kubernetes y sin variables de otros Services (`enableServiceLinks: false`). |
| **Kernel** | RuntimeClass **gVisor** en producción: las syscalls del contenedor no llegan directamente al kernel del nodo. |
| **Red** | Todo denegado por defecto. Se permite: tráfico dentro del mismo proyecto; entrada solo desde el ingress controller; DNS del clúster; y salida a internet **solo a los puertos TCP 80 y 443**, nunca a redes privadas (10/8, 172.16/12, 192.168/16, 100.64/10) ni a 169.254/16 (metadatos). |
| **Recursos** | `ResourceQuota` por plan sobre las **peticiones** de CPU (la CPU ociosa se puede usar a ráfagas) y sobre las peticiones y **límites** de memoria. `LimitRange` da valores por defecto a los pods que no los declaran. |
| **Disponibilidad** | Rolling update con `maxUnavailable: 0`: el pod viejo sirve hasta que el nuevo está listo. |

Valores iniciales por plan (`domain/tiers.go`):

| Plan | CPU pedida | Memoria pedida | Límite de memoria | Pods | Disco | Por contenedor | Réplicas |
|------|-----------|----------------|-------------------|------|-------|----------------|----------|
| trial / hobby ($5) | 500m | 1 GiB | 2 GiB | 10 | 5 GiB | 100m–1 CPU, 192–384 MiB | 1 |
| pro ($12) | 1 CPU | 2 GiB | 4 GiB | 20 | 20 GiB | 200m–2 CPU, 384–768 MiB | 1 |
| team ($20) | 2 CPU | 4 GiB | 8 GiB | 40 | 50 GiB | 200m–2 CPU, 384–768 MiB | 2 |

Una prueba (`TestEveryTierFitsARollingUpdate`) garantiza que un proyecto típico, con frontend, backend, Postgres y Redis, siempre cabe en su cuota durante un rolling update.

## Alternativas consideradas

- **Un clúster o una VM por cliente** (Firecracker, como Fly.io): el aislamiento más fuerte, pero demasiado caro y complejo para planes de $5 en esta fase. Se revisará para un plan empresarial.
- **Solo namespaces y NetworkPolicy, sin gVisor:** una fuga del kernel comprometería el nodo entero, con los proyectos de otros clientes dentro.
- **Salida a internet sin restricciones:** es lo más cómodo para el cliente, pero es la puerta al spam y al abuso.

## Consecuencias

- **Positivas:** varias capas independientes (si una falla, quedan las demás); los planes se cumplen por el propio Kubernetes; todo es declarativo y verificable.
- **Negativas:**
  - Las imágenes deben funcionar con un UID arbitrario y el grupo 0, con sus carpetas escribibles por el grupo, al estilo OpenShift. El builder debe generarlas así.
  - Las apps que se conectan a servicios externos por puertos que no son 80/443 (una base de datos gestionada fuera, SMTP de un proveedor) no funcionarán hasta ofrecer excepciones por plan.
  - gVisor añade latencia a las syscalls y no todas las apps son compatibles, así que habrá que medirlo.
  - Los valores de las cuotas son un punto de partida y se recalibran con uso real (ver el modelo de costes en `docs/system-design.md`).
