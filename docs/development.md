# Entorno de desarrollo

Todo se maneja con `./scripts/dev.sh`, que funciona en Linux, macOS y **Git Bash en Windows**. Ejecuta `./scripts/dev.sh help` para ver los comandos.

## 1. Herramientas

| Herramienta | Para qué | Windows | macOS / Linux |
|-------------|----------|---------|---------------|
| **Go** (versión de `go.mod`) | Servicios | `winget install GoLang.Go` o el zip de go.dev | `brew install go` / paquete oficial |
| **Docker** | NATS, Postgres, clúster local | Docker Desktop (con WSL2) | Docker Desktop / Docker Engine |
| **k3d** | Clúster K3s local | `winget install k3d` | `brew install k3d` / script oficial |
| **kubectl** | Ver el clúster | Viene con Docker Desktop | `brew install kubectl` |
| Git Bash | Ejecutar `scripts/dev.sh` | Viene con Git for Windows | No hace falta |

> **Windows con VirtualBox:** Docker Desktop (WSL2) y VirtualBox no rinden a la vez. Si usas VMs, alterna con `bcdedit /set hypervisorlaunchtype auto|off` y reinicia.

## 2. Niveles de entorno

No siempre hace falta levantarlo todo:

| Nivel | Qué levanta | Para qué | Comando |
|-------|-------------|----------|---------|
| **0 · Solo Go** | Nada | El 90 % del trabajo: dominio, casos de uso, adaptadores con fakes | `./scripts/dev.sh check` |
| **1 · Infraestructura** | NATS + Postgres en Docker | Probar natsbus, el adaptador de Postgres y los servicios conectados | `./scripts/dev.sh infra-up` |
| **2 · Clúster** | K3s en Docker (k3d) + registry | Probar el deployer de verdad y los despliegues completos | `./scripts/dev.sh cluster-up` |

```bash
./scripts/dev.sh infra-up
./scripts/dev.sh cluster-up
./scripts/dev.sh test-all     # unitarias + integración contra NATS y el clúster
```

## 3. Levantar los servicios en local

Con el nivel 1 (y el 2 para el deployer), en terminales separadas:

```bash
NATS_URL=nats://localhost:4222 go run ./services/control-plane/cmd/control-plane
NATS_URL=nats://localhost:4222 GITHUB_WEBHOOK_SECRET=dev-secret go run ./services/github-integration/cmd/github-integration
NATS_URL=nats://localhost:4222 KUBECONFIG=$HOME/.kube/config go run ./services/deployer/cmd/deployer
```

Para ver los eventos pasar por el bus, usa la [CLI de NATS](https://github.com/nats-io/natscli):

```bash
nats sub 'jappi.>'
```

## 4. Variables de entorno por servicio

| Servicio | Variable | Por defecto | Notas |
|----------|----------|-------------|-------|
| todos | `NATS_URL` | `nats://localhost:4222` | En el control-plane, si está vacía usa el bus en memoria |
| todos | `HTTP_ADDR` | control-plane `:8081`, github-integration `:8082`, deployer `:8083` | `/healthz` en todos |
| github-integration | `GITHUB_WEBHOOK_SECRET` | **obligatoria** | Sin ella el servicio no arranca |
| deployer | `KUBECONFIG` | vacío = dentro del clúster | Ruta a un kubeconfig para desarrollo |
| deployer | `RUNTIME_CLASS` | vacío | `gvisor` en producción |
| deployer | `INGRESS_CLASS` | `traefik` | |
| deployer | `INGRESS_NAMESPACE` | `kube-system` | Único origen de tráfico externo permitido hacia los pods |

## 5. Pruebas

| Tipo | Se activan con | Qué necesitan |
|------|----------------|---------------|
| Unitarias y de arquitectura | Siempre | Nada |
| Contrato del bus sobre NATS | `JAPPI_TEST_NATS_URL=nats://localhost:4222` | Nivel 1 |
| Deployer contra Kubernetes | `JAPPI_TEST_KUBECONFIG=$HOME/.kube/config` | Nivel 2 (o cualquier clúster) |

La CI las ejecuta todas en cada PR:
- **job `go`:** gofmt, vet, pruebas con `-race`, govulncheck y build de todos los servicios;
- **job `kubernetes`:** clúster kind + prueba de integración del deployer;
- **job `images`:** build de las imágenes Docker.

> En Windows, `go test -race` necesita un compilador de C (cgo). Si no lo tienes, la CI lo ejecuta por ti en Linux.

## 6. Imágenes

```bash
./scripts/dev.sh images   # localhost:5001/jappi-<servicio>:dev
docker push localhost:5001/jappi-deployer:dev
```

Un solo `Dockerfile` construye cualquier servicio (`--build-arg SERVICE=<nombre>`): binario estático sobre distroless, sin shell y sin root.

## 7. Problemas frecuentes

| Síntoma | Causa | Solución |
|---------|-------|----------|
| `failed to connect to the docker API` | Docker Desktop apagado | Ábrelo y espera a que diga *Running* |
| `k3d: command not found` en Git Bash | Git Bash no ve el PATH nuevo | Cierra y abre la terminal |
| `TestHexagonalRules` falla | Una capa importa algo prohibido | Mueve el código a la capa correcta (ver CONTRIBUTING); no desactives la prueba |
| El rollout falla con `CreateContainerConfigError` | Falta el Secret de un add-on | En local, créalo a mano: `kubectl -n prj-<id> create secret generic jappi-addon-postgres --from-literal=uri=...` |
