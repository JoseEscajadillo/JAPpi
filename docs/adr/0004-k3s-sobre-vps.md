# 0004. K3s sobre VPS como plano de datos

- **Estado:** Aceptado
- **Fecha:** 2026-09-29
- **Decisores:** equipo JAPpi

## Contexto

JAPpi ejecuta las aplicaciones de sus clientes. El precio de entrada es de $5 al mes con frontend, backend, Postgres y Redis, así que el coste por cliente tiene que ser muy bajo. Además queremos controlar y entender el despliegue nosotros mismos, no delegarlo en un servicio gestionado.

## Decisión

- El plano de datos es un clúster **K3s** sobre **VPS** (Hetzner o DigitalOcean).
- Arrancamos con un nodo servidor y crecemos añadiendo nodos agentes.
- Cada proyecto de cliente vive en su propio **namespace**, con:
  - `ResourceQuota` y `LimitRange` según su plan;
  - `NetworkPolicy` que niega todo el tráfico entre namespaces por defecto;
  - `RuntimeClass` gVisor para el código del cliente.
- Ingress con **Traefik** (viene con K3s) y **cert-manager** para los certificados TLS.
- El Postgres de los clientes se gestiona con el operador **CloudNativePG**.

## Alternativas consideradas

- **Kubernetes gestionado (EKS, AKS, GKE):** 70–150 USD al mes solo por el plano de control y los nodos mínimos; inviable con planes de $5 en esta fase.
- **Docker Compose o Swarm en cada VPS:** más simple, pero sin cuotas, sin políticas de red y sin reconciliación; lo tendríamos que reinventar.
- **Nomad:** buena opción, pero perderíamos el ecosistema (operadores, cert-manager, CloudNativePG).

## Consecuencias

- **Positivas:** coste de ~5–15 € al mes para empezar; los planes se traducen directamente en cuotas de Kubernetes; aprendemos a operar Kubernetes de verdad.
- **Negativas:** operamos nosotros el clúster: backups, actualizaciones y guardias. Un solo nodo servidor es un punto único de fallo hasta que montemos alta disponibilidad (HA con 3 servidores y etcd embebido). gVisor añade algo de latencia a las syscalls.
