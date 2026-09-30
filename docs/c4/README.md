# Arquitectura C4 de JAPpi

Modelo [C4](https://c4model.com) en cuatro niveles de zoom. Los diagramas están en Mermaid, así que GitHub los renderiza directamente y se editan como texto en el mismo PR que cambia la arquitectura.

> **Regla:** si un PR añade un servicio, un contenedor, un sistema externo o una relación nueva, actualiza el diagrama correspondiente en el mismo PR.

---

## Nivel 1: contexto del sistema

Quién usa JAPpi y con qué sistemas externos habla.

```mermaid
C4Context
  title JAPpi: contexto del sistema

  Person(dev, "Desarrollador", "Cliente de JAPpi. Conecta su repo y gestiona sus despliegues.")
  Person_Ext(visitor, "Usuario final", "Visita la aplicación que el cliente desplegó.")

  System(jappi, "JAPpi", "PaaS: construye, despliega y opera frontend, backend, PostgreSQL y Redis desde un repo.")

  System_Ext(github, "GitHub", "Aloja el código del cliente. Envía webhooks de push a la GitHub App de JAPpi.")
  System_Ext(stripe, "Stripe", "Suscripciones, prueba gratuita y cobros.")
  System_Ext(dns, "Proveedor DNS", "Zona del dominio base; valida el certificado wildcard (DNS-01).")

  Rel(dev, jappi, "Importa repos, ve logs, gestiona variables y hace rollback", "HTTPS")
  Rel(visitor, jappi, "Usa las apps desplegadas", "HTTPS")
  Rel(github, jappi, "Webhooks de push", "HTTPS + HMAC")
  Rel(jappi, github, "Descarga el código de un commit", "GitHub App API")
  Rel(jappi, stripe, "Crea sesiones de checkout", "HTTPS")
  Rel(stripe, jappi, "Webhooks de suscripción", "HTTPS + firma")
  Rel(jappi, dns, "Registros ACME", "API")
```

---

## Nivel 2: contenedores

Las piezas desplegables de JAPpi. Salvo el dashboard, todo se comunica por eventos a través de NATS (ADR-0002).

```mermaid
C4Container
  title JAPpi: contenedores

  Person(dev, "Desarrollador")
  System_Ext(github, "GitHub")
  System_Ext(stripe, "Stripe")

  System_Boundary(jappi, "JAPpi") {
    Container(web, "Dashboard", "Next.js, Multi-Zones", "shell, console y billing (ADR-0005)")
    Container(cp, "control-plane", "Go", "Proyectos, servicios, variables, despliegues. Detector de monorepos y cableado (ADR-0008).")
    Container(gh, "github-integration", "Go", "Recibe y verifica los webhooks; publica repo.pushed.")
    Container(builder, "builder", "Go + BuildKit", "Construye imágenes OCI (Railpack o Dockerfile) y las sube al registry.")
    Container(deployer, "deployer", "Go + client-go", "Reconcilia el estado deseado en K3s: Deployment, Service, Ingress, Secrets.")
    Container(addons, "addons", "Go", "Aprovisiona Postgres (CloudNativePG) y Redis por proyecto.")
    Container(billing, "billing", "Go", "Stripe: prueba, planes, cuotas.")
    Container(logs, "logs", "Go", "Transmite logs y métricas al dashboard (WebSocket).")

    ContainerQueue(nats, "Bus de eventos", "NATS JetStream", "Stream JAPPI, subjects jappi.*")
    ContainerDb(db, "Platform DB", "PostgreSQL", "Una base de datos por servicio (esquemas separados).")
    ContainerDb(registry, "Registry", "OCI Distribution", "Imágenes de los clientes, referenciadas por digest.")
    ContainerDb(loki, "Loki + Prometheus", "Observabilidad", "Logs y métricas de las apps de los clientes.")
  }

  Rel(dev, web, "Usa", "HTTPS")
  Rel(web, cp, "Consultas y comandos", "HTTPS/JSON")
  Rel(web, logs, "Logs en vivo", "WebSocket")
  Rel(github, gh, "Webhook push", "HTTPS")
  Rel(stripe, billing, "Webhooks", "HTTPS")

  Rel(gh, nats, "repo.pushed")
  Rel(cp, nats, "build.requested, rollback.requested")
  Rel(builder, nats, "build.succeeded / build.failed")
  Rel(deployer, nats, "deployment.status_changed")
  Rel(billing, nats, "subscription.changed")

  Rel(cp, db, "Lee/escribe")
  Rel(builder, registry, "Push de imágenes")
  Rel(logs, loki, "Consulta")
```

---

## Nivel 3: componentes del control-plane

El hexágono del servicio central (ADR-0006). Las flechas apuntan siempre hacia dentro: los adaptadores dependen de `app`, y `app` depende de `domain`.

```mermaid
C4Component
  title control-plane: componentes

  Container_Boundary(cp, "control-plane") {
    Component(httpin, "adapters/in/http", "net/http", "API del dashboard")
    Component(evin, "adapters/in/events", "Consumidor NATS", "Traduce repo.pushed a HandlePush")
    Component(cli, "cmd/jappi-detect", "CLI", "Analiza una carpeta local")

    Component(uc_push, "app.HandlePush", "Caso de uso", "Push → despliegues + build.requested")
    Component(uc_an, "app.AnalyzeRepository", "Caso de uso", "Vista previa de un repo antes de importarlo")

    Component(stack, "domain/stack", "Dominio", "Detectores (Node, Go, Python, Dockerfile), servicios afectados")
    Component(wiring, "domain/wiring", "Dominio", "Decide qué variables inyectar y con qué nombre")
    Component(domains, "domain/domains", "Dominio", "Dominios reservados")
    Component(deploy, "domain/deployment", "Dominio", "Máquina de estados del despliegue")

    Component(mem, "adapters/out/memory", "Adaptador", "Repositorios en memoria (dev/tests)")
    Component(pg, "adapters/out/postgres", "Adaptador (pendiente)", "Repositorios en PostgreSQL")
    Component(repofs, "adapters/out/repofs", "Adaptador", "RepoSource: carpeta local / tarball de GitHub")
  }

  ContainerQueue(nats, "NATS JetStream")
  ContainerDb(db, "Platform DB", "PostgreSQL")

  Rel(evin, uc_push, "Llama")
  Rel(httpin, uc_an, "Llama")
  Rel(cli, uc_an, "Llama")
  Rel(uc_push, stack, "Usa")
  Rel(uc_push, wiring, "Usa")
  Rel(uc_push, deploy, "Usa")
  Rel(uc_an, stack, "Usa")
  Rel(uc_an, wiring, "Usa")
  Rel(uc_an, domains, "Usa")
  Rel(mem, uc_push, "Implementa ProjectFinder, DeploymentSaver")
  Rel(pg, db, "SQL")
  Rel(repofs, uc_an, "Implementa RepoSource")
  Rel(evin, nats, "Consume")
```

---

## Nivel 4: despliegue

Dónde corre cada cosa (ADR-0004).

```mermaid
C4Deployment
  title JAPpi: despliegue en K3s sobre VPS

  Deployment_Node(vps1, "VPS servidor", "Hetzner CX32, Ubuntu 24.04") {
    Deployment_Node(k3s_s, "K3s server") {
      Deployment_Node(ns_sys, "namespace jappi-system") {
        Container(cp_i, "control-plane, github-integration, builder, deployer, addons, billing, logs", "Pods Go")
        ContainerQueue(nats_i, "NATS JetStream", "StatefulSet, 3 réplicas")
        ContainerDb(db_i, "Platform DB", "CloudNativePG")
      }
      Deployment_Node(ns_ing, "kube-system") {
        Container(traefik, "Traefik + cert-manager", "Ingress, TLS wildcard")
      }
    }
  }

  Deployment_Node(vps2, "VPS agentes (1..n)", "Hetzner CX22") {
    Deployment_Node(k3s_a, "K3s agent") {
      Deployment_Node(ns_cli, "namespace prj-<id> (uno por proyecto)", "ResourceQuota + NetworkPolicy + gVisor") {
        Container(app_fe, "frontend del cliente", "Pod")
        Container(app_be, "backend del cliente", "Pod")
        ContainerDb(app_pg, "Postgres del cliente", "CloudNativePG")
        ContainerDb(app_rd, "Redis del cliente", "StatefulSet")
      }
    }
  }

  Rel(traefik, app_fe, "HTTPS")
  Rel(traefik, app_be, "HTTPS")
  Rel(app_be, app_pg, "TCP 5432, dentro del namespace")
```

---

## Flujo dinámico: de un push a producción

```mermaid
sequenceDiagram
  autonumber
  participant GH as GitHub
  participant GI as github-integration
  participant N as NATS
  participant CP as control-plane
  participant B as builder
  participant D as deployer
  participant K as K3s

  GH->>GI: POST /webhooks/github (push)
  GI->>GI: verifica HMAC, extrae archivos cambiados
  GI->>N: repo.pushed (id = github-<delivery>)
  N->>CP: repo.pushed
  CP->>CP: servicios afectados + cableado de variables
  CP->>N: build.requested por servicio (id = build-<deployment>)
  N->>B: build.requested
  B->>B: descarga el commit, construye con BuildEnv
  B->>N: build.succeeded (imagen@sha256)
  N->>D: build.succeeded
  D->>K: server-side apply (Deployment, Service, Ingress, Secret)
  K-->>D: rollout listo y readiness OK
  D->>N: deployment.status_changed = healthy
  N->>CP: actualiza el historial (el anterior pasa a superseded)
```
