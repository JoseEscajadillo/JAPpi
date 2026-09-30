# Cómo contribuir a JAPpi

Gracias por sumarte. Este documento es el acuerdo del equipo sobre **cómo** construimos JAPpi: estructura, arquitectura, principios, flujo de Git y la definición de "terminado". Si algo aquí no te convence, propón el cambio en un PR a este mismo archivo; las reglas también se revisan.

> **Versión corta:**
> 1. Lee los [ADR](docs/adr/) y el [C4](docs/c4/README.md).
> 2. Cada servicio es un hexágono.
> 3. Entre servicios, solo eventos.
> 4. `go test ./...` en verde antes de pedir revisión.
> 5. Una decisión importante → un ADR.

---

## Índice

1. [Qué estamos construyendo](#qué-estamos-construyendo)
2. [Preparar el entorno](#preparar-el-entorno)
3. [Mapa del repositorio](#mapa-del-repositorio)
4. [Arquitectura hexagonal](#arquitectura-hexagonal)
5. [SOLID en Go, con ejemplos de este repo](#solid-en-go-con-ejemplos-de-este-repo)
6. [Eventos entre servicios](#eventos-entre-servicios)
7. [Pruebas](#pruebas)
8. [Estilo de código Go](#estilo-de-código-go)
9. [Frontend (Next.js Multi-Zones)](#frontend-nextjs-multi-zones)
10. [ADR y C4: documentar decisiones](#adr-y-c4-documentar-decisiones)
11. [Flujo de Git y pull requests](#flujo-de-git-y-pull-requests)
12. [Seguridad](#seguridad)
13. [Trabajar con Claude Code](#trabajar-con-claude-code)
14. [Definición de terminado](#definición-de-terminado)

---

## Qué estamos construyendo

JAPpi es un PaaS: el usuario conecta un repo de GitHub y JAPpi construye, despliega y opera su **frontend, backend, PostgreSQL y Redis** desde un solo dashboard. Incluye CI/CD, logs, redeploy en cada push, historial y rollback.

**El diferencial:** detectamos los monorepos y **cableamos solos las variables entre servicios**. El frontend recibe la URL del backend, el backend recibe el origen del frontend para CORS y las cadenas de conexión de Postgres y Redis, y todo con los nombres que el código ya usa ([ADR-0008](docs/adr/0008-deteccion-de-monorepos-y-cableado-automatico.md)).

Pruébalo en un minuto:

```bash
go run ./services/control-plane/cmd/jappi-detect services/control-plane/testdata/acme-monorepo
```

## Preparar el entorno

Guía completa, con instalación en Windows, macOS y Linux: [docs/development.md](docs/development.md).

```bash
git clone https://github.com/JoseEscajadillo/JAPpi.git && cd JAPpi
./scripts/dev.sh check        # gofmt + vet + pruebas: no necesita nada más que Go
./scripts/dev.sh infra-up     # NATS + Postgres en Docker
./scripts/dev.sh cluster-up   # K3s local con k3d + registry en localhost:5001
./scripts/dev.sh test-all     # incluye integración contra NATS y el clúster
```

## Mapa del repositorio

```
jappi/
├── services/                 un directorio por microservicio (cada uno es un hexágono)
│   ├── control-plane/        proyectos, despliegues, detector y cableado
│   ├── github-integration/   webhooks de GitHub → repo.pushed
│   └── deployer/             deploy.requested → Kubernetes (aislamiento por proyecto)
├── pkg/                      código compartido; NO lógica de negocio
│   ├── contracts/events/     contratos de eventos (el "idioma común")
│   ├── eventbus/             adaptadores del bus: natsbus, memory y su suite de contrato
│   └── archtest/             hace cumplir las reglas hexagonales en go test
├── web/                      dashboard Next.js (Multi-Zones)
├── deploy/                   docker-compose y k3d para desarrollo, manifiestos de K3s
├── scripts/dev.sh            tareas de desarrollo (check, infra, clúster, imágenes)
├── Dockerfile                imagen de cualquier servicio (--build-arg SERVICE=...)
├── docs/
│   ├── adr/                  decisiones de arquitectura
│   ├── c4/                   diagramas C4 (contexto, contenedores, componentes, despliegue)
│   ├── system-design.md      capacidad, cuellos de botella, escalado, SLO y costes
│   ├── development.md        entorno de desarrollo
│   ├── events.md             catálogo de eventos
│   └── roadmap.md            fases y reparto de tareas
└── .claude/skills/           skills de Claude Code del proyecto
```

## Arquitectura hexagonal

Cada servicio separa **lo que decide** (dominio) de **cómo se conecta con el mundo** (adaptadores). Detalle y motivos en [ADR-0006](docs/adr/0006-arquitectura-hexagonal-y-solid.md).

```
                ┌───────────────────── services/<nombre> ─────────────────────┐
  HTTP  ──▶ adapters/in ──▶  app (casos de uso + puertos)  ──▶  domain
  NATS  ──▶                        │                              ▲
  CLI   ──▶                        ▼ (interfaces)                 │ Go puro
                          adapters/out ──▶ Postgres, NATS, K8s, GitHub, Stripe
                └───────────── cmd/<nombre>/main.go conecta todo ─────────────┘
```

| Capa | Qué contiene | Puede importar |
|------|--------------|----------------|
| `internal/domain` | Entidades, reglas, máquinas de estado. Ej.: `stack`, `wiring`, `deployment` | **Solo** la librería estándar y el propio dominio |
| `internal/app` | Casos de uso (`HandlePush`) y los puertos que usan (`ProjectFinder`) | Librería estándar, `domain` y `pkg/contracts` |
| `internal/adapters/in` | HTTP, consumidores de eventos, CLI. **Solo traducen** | Cualquier cosa |
| `internal/adapters/out` | Implementaciones de los puertos | Cualquier cosa |
| `cmd/<nombre>` | Raíz de composición: crea los adaptadores y los inyecta | Cualquier cosa |

**No es una recomendación, es una prueba.** `pkg/archtest` revisa los imports en cada `go test`; si el dominio importa un adaptador, falla:

```
--- FAIL: TestHexagonalRules
    domain/project/bad.go: la capa "domain" no puede importar
    "github.com/JoseEscajadillo/JAPpi/pkg/eventbus/natsbus" (ver docs/adr/0006)
```

**¿Dónde pongo esto?**

- ¿Es una regla que seguiría siendo cierta aunque cambiáramos de base de datos o de proveedor de Git? → `domain`.
- ¿Orquesta pasos: busca, decide, guarda, publica? → `app`.
- ¿Habla con algo externo o traduce un formato (JSON de GitHub, filas SQL)? → `adapters`.
- ¿Crea objetos concretos o lee variables de entorno? → `cmd`.

## SOLID en Go, con ejemplos de este repo

Go no tiene clases ni herencia, así que SOLID se aplica a su manera. Cada principio con el código donde ya lo aplicamos:

### S: responsabilidad única

Cada tipo tiene un solo motivo para cambiar.

- `app.HandlePush` orquesta un push; no sabe parsear webhooks ni hablar SQL.
- El adaptador `adapters/in/http/webhook.go` de github-integration solo traduce HTTP ↔ caso de uso. La verificación de la firma vive en `domain.VerifySignature` y la publicación en `app.ReceivePush`.
- Regla práctica: si al describir un tipo usas "y" dos veces, divídelo.

### O: abierto/cerrado

Se amplía añadiendo código, no modificando el que ya funciona.

- El detector de monorepos usa **estrategias**: `stack.Detector` es una interfaz, y `NodeDetector`, `GoDetector`, `PythonDetector` y `DockerfileDetector` la implementan. Para soportar Rust se escribe un `RustDetector` y se registra en `DefaultAnalyzer()`; el `Analyzer` no se toca.
- Lo mismo con los frameworks: añadir SolidStart es una fila en `nodeFrontends`, no otro `if`.
- **Señal de alarma:** un `switch` sobre tipos o nombres que crece con cada funcionalidad.

### L: sustitución de Liskov

Toda implementación de un puerto se comporta igual ante quien la usa.

- `pkg/eventbus/bustest` es una **suite de contrato**: el bus en memoria y el de NATS pasan exactamente las mismas pruebas (entrega intacta, filtro por tipo, deduplicación por ID, una copia por consumidor). Por eso las pruebas de los casos de uso pueden usar el bus en memoria y confiar en el resultado.
- Al escribir el adaptador de Postgres, crea una suite de contrato para `ProjectFinder` y `DeploymentSaver` y ejecútala también contra `adapters/out/memory`.

### I: segregación de interfaces

Interfaces pequeñas, declaradas **por quien las consume**.

- `app.HandlePush` pide `ProjectFinder` (1 método), `DeploymentSaver` (1 método) y `EventPublisher` (1 método), no un `Repository` con veinte métodos.
- El consumidor de eventos declara su propio `Subscriber`; github-integration declara su propio `EventPublisher`. Las dos interfaces las satisface `natsbus.Bus` sin saber que existen.
- **Idioma Go:** *acepta interfaces, devuelve structs*. No crees una interfaz hasta que haya un consumidor que la necesite.

### D: inversión de dependencias

La lógica depende de abstracciones; los detalles se inyectan desde fuera.

- `app` depende de interfaces; `cmd/control-plane/main.go` decide que hoy son `memory.Store` y `natsbus.Bus` (o el bus en memoria si falta `NATS_URL`). Cambiar a Postgres toca **una línea** en `main.go`.
- El dominio del detector recibe un `fs.FS`, así que no sabe si los archivos vienen de un clon de git, de un tarball de GitHub o de una carpeta local.
- Sin contenedores de inyección ni reflexión: en Go, la inyección se hace a mano en `main.go`, y así se lee de un vistazo.

### Lo que SOLID **no** significa aquí

- **No** crees interfaces para todo "por si acaso"; eso es Java, no Go.
- **No** hagas capas de un solo método que solo reenvían la llamada.
- **No** escribas getters y setters: los campos exportados están bien en structs de datos.

## Eventos entre servicios

- Los servicios **nunca** se llaman entre sí por HTTP; publican y consumen eventos en NATS JetStream ([ADR-0002](docs/adr/0002-microservicios-orientados-a-eventos-con-nats.md)).
- Contratos: `pkg/contracts/events`. Catálogo y reglas: [`docs/events.md`](docs/events.md).
- **Todo handler es idempotente.** Pregúntate siempre: ¿qué pasa si este evento llega dos veces, o si mi handler falla a mitad de camino? Ejemplo: `HandlePush` usa IDs deterministas (`deployment.IDFor`) y publica con ID fijo (`build-<deploymentID>`).
- Error que no se arregla reintentando → `events.Permanent(err)`.
- Los secretos **jamás** van en un evento.

## Pruebas

| Tipo | Dónde | Cómo |
|------|-------|------|
| Dominio | `internal/domain/**/*_test.go` | Tablas de casos, datos en memoria (`fstest.MapFS`). Deben tardar milisegundos |
| Casos de uso | `internal/app/*_test.go` | Adaptadores en memoria reales, no mocks generados |
| Adaptadores de entrada | `adapters/in/**/*_test.go` | `httptest` + bus en memoria (ver `webhook_test.go`) |
| Contrato | `pkg/**/xxxtest` | Una suite, varias implementaciones |
| Integración | `*_test.go` con `t.Skip` si falta `JAPPI_TEST_*` | Contra NATS, Postgres o un Kubernetes real (la CI levanta kind) |
| Arquitectura | `services/*/arch_test.go` | Obligatoria en todo servicio |

Reglas:

- Todo bug corregido llega con la prueba que lo habría detectado.
- Nombra las pruebas por el comportamiento: `TestRedeliveredPushIsIdempotent`, no `TestHandlePush2`.
- Sin `time.Sleep` para sincronizar, salvo para comprobar que algo **no** ocurre, y con un margen acotado.
- Ejecuta `go test -race ./...` antes de un PR que toque concurrencia.

## Estilo de código Go

- `gofmt` y `go vet` sin avisos (la CI los exige). Recomendado: `golangci-lint run`.
- **Errores:** siempre con contexto, `fmt.Errorf("guardar despliegue %s: %w", id, err)`. Nunca los ignores en silencio; si es a propósito, `_ =` y un comentario.
- **`context.Context`** como primer parámetro de todo lo que haga E/S.
- **Logs:** `log/slog` en JSON, con claves estables (`event`, `id`, `repo`). **Nunca** registres secretos ni cuerpos completos de webhooks.
- **Configuración:** desde variables de entorno, leídas **solo** en `main.go`. Un secreto que falta es un error de arranque, no un valor por defecto.
- **Nombres:**
  - paquetes cortos y en singular (`stack`, `wiring`); `utils`, `helpers` y `common` están prohibidos;
  - interfaces por capacidad (`DeploymentSaver`), sin prefijo `I`.
- **Idioma:** identificadores en inglés; comentarios, mensajes de error, logs y docs en español.
- Comenta el **porqué**, no el qué. Ejemplo bueno, de `handle_push.go`: "Publicamos aunque el despliegue ya existiera: si el intento anterior falló justo después de guardar, el build nunca se pidió".

## Frontend (Next.js Multi-Zones)

El dashboard es un monorepo pnpm + Turborepo en `web/` con tres zonas independientes ([ADR-0005](docs/adr/0005-nextjs-multi-zones-para-microfrontends.md)):

| Zona | Rutas | Qué contiene |
|------|-------|--------------|
| `web/apps/shell` | `/`, `/pricing`, `/docs` | Web pública y login; hace de *router* hacia las demás |
| `web/apps/console` | `/app/*` | Proyectos, despliegues, logs, variables |
| `web/apps/billing` | `/billing/*` | Planes y Stripe |

Reglas:

- **UI compartida** en `web/packages/ui`; **tipos de la API** en `web/packages/api-client`. Ninguna zona importa código de otra.
- **Entre zonas se navega con `<a href>`**, no con `<Link>` (es otra app). Dentro de una zona, `<Link>`.
- **Server Components por defecto.** `"use client"` solo donde haya interactividad.
- **El navegador nunca habla con NATS ni con servicios internos:** solo con el control-plane (HTTP) y con el servicio de logs (WebSocket).
- **Variables `NEXT_PUBLIC_*`** solo para valores públicos. JAPpi se despliega a sí mismo, así que nuestro dashboard es el primer cliente del cableado automático.

## ADR y C4: documentar decisiones

**ADR.** Escribe uno cuando la decisión cumpla los criterios de [`docs/adr/README.md`](docs/adr/README.md): tecnología nueva, contrato incompatible, servicio nuevo, seguridad, costes, o cuando se haya discutido más de 15 minutos.

- Usa la [plantilla](docs/adr/template.md): contexto, decisión, alternativas y consecuencias, **incluidas las negativas**.
- Entra como `Propuesto` en el PR y pasa a `Aceptado` al aprobarse.
- **Un ADR aceptado no se edita:** se reemplaza por otro.

**C4.** [`docs/c4/README.md`](docs/c4/README.md) tiene los niveles 1–4 y el flujo de un push, en Mermaid. Si tu PR añade un contenedor, un sistema externo o una relación, el diagrama se actualiza **en el mismo PR**.

## Flujo de Git y pull requests

- **`main` siempre desplegable y protegida:** nadie hace push directo.
- **Ramas cortas** con el formato `tipo/descripcion-corta`: `feat/deployer-rollback`, `fix/webhook-signature`, `docs/adr-0011-cloudnativepg`.
- **Commits** con [Conventional Commits](https://www.conventionalcommits.org/es/):
  ```
  feat(control-plane): redesplegar solo los servicios afectados por un push
  fix(github-integration): rechazar webhooks sin firma
  docs(adr): 0011 usar CloudNativePG
  ```
  Tipos: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`. El ámbito es el servicio o el área.
- **PR pequeños** (idealmente menos de 400 líneas cambiadas), con descripción de **qué**, **por qué** y **cómo probarlo**, y enlace al ADR si aplica.
- **Revisión:**
  - al menos **1 aprobación** de otra persona, y 2 si toca `pkg/contracts`, la seguridad o el aislamiento de clientes;
  - quien revisa mira la arquitectura, las pruebas y la seguridad, no el estilo (de eso se encarga `gofmt`).
- **Merge:** *squash and merge*; el título del PR es el mensaje final.

## Seguridad

JAPpi ejecuta **código ajeno**. Tratamos cada proyecto de cliente como hostil.

- **Secretos:** nunca en el repo, en eventos, en logs ni en el entorno de build. En Kubernetes, como `Secret`, y en local, en un `.env` (ignorado por git).
- **Webhooks:** se verifica la firma **antes** de leer el contenido (ver `github-integration`). Webhook sin secreto configurado = el servicio no arranca.
- **Aislamiento:** cada proyecto tiene un namespace con `ResourceQuota`, una `NetworkPolicy` que deniega por defecto y gVisor. Nada de `privileged`, `hostPath` ni `hostNetwork` para código de clientes.
- **Dependencias:** `govulncheck ./...` antes de añadir una librería nueva.
- **Si encuentras una vulnerabilidad:** avisa por privado al equipo, no la describas en un issue público.

## Trabajar con Claude Code

El repo trae skills en `.claude/skills/` y un `CLAUDE.md` con el contexto del proyecto:

| Skill | Cuándo |
|-------|--------|
| `systems-design` | Antes de implementar algo que toca un servicio, un evento o varios servicios; también para revisar un diseño |
| `hexagonal-service` | Crear un servicio o un caso de uso con la estructura correcta |
| `new-adr` | Redactar un ADR con el formato del proyecto |

El código generado con IA pasa por la **misma revisión** que el escrito a mano, y quien abre el PR responde por él.

## Definición de terminado

Un PR está listo para revisión cuando:

- [ ] `gofmt -l .` no lista nada; `go vet ./...` y `go test ./...` pasan.
- [ ] La lógica nueva tiene pruebas y los bugs corregidos, su prueba de regresión.
- [ ] Respeta las capas (lo garantiza `TestHexagonalRules`) y no hay llamadas HTTP entre servicios.
- [ ] Los handlers de eventos nuevos son idempotentes y la descripción del PR explica por qué.
- [ ] Ningún secreto en código, logs, eventos ni entorno de build.
- [ ] Si hubo una decisión de arquitectura, está el ADR; si cambió la arquitectura, el C4; si hay un evento nuevo, `docs/events.md`; si cambió la capacidad, un cuello de botella o los costes, `docs/system-design.md`.
- [ ] `docs/roadmap.md` refleja el avance.
- [ ] La descripción del PR explica qué, por qué y cómo probarlo.
