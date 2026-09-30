#!/usr/bin/env bash
# Tareas de desarrollo de JAPpi. Funciona en Linux, macOS y Git Bash (Windows).
#   ./scripts/dev.sh help
set -euo pipefail
cd "$(dirname "$0")/.."

CLUSTER=jappi-dev
COMPOSE="docker compose -f deploy/docker-compose.dev.yml"

need() { command -v "$1" >/dev/null 2>&1 || { echo "falta '$1': ver docs/development.md" >&2; exit 1; }; }

cmd_help() {
	cat <<'EOF'
uso: ./scripts/dev.sh <comando>

  infra-up       NATS y Postgres en Docker
  infra-down     los detiene (los datos se conservan en volúmenes)
  cluster-up     clúster K3s local con k3d + registry en localhost:5001
  cluster-down   borra el clúster local
  check          gofmt + go vet + pruebas unitarias (lo mismo que la CI)
  test-all       check + pruebas de integración contra NATS y el clúster local
  detect DIR     muestra lo que JAPpi detectaría en un repo local
  images         construye las imágenes de todos los servicios
  status         qué está levantado
EOF
}

cmd_infra-up()   { need docker; $COMPOSE up -d; echo "NATS: nats://localhost:4222 (monitor http://localhost:8222) · Postgres: localhost:5432"; }
cmd_infra-down() { need docker; $COMPOSE down; }

cmd_cluster-up() {
	need docker; need k3d
	if k3d cluster list "$CLUSTER" >/dev/null 2>&1; then
		echo "el clúster $CLUSTER ya existe"
	else
		k3d cluster create --config deploy/k3d/jappi-dev.yaml
	fi
	kubectl config use-context "k3d-$CLUSTER" >/dev/null
	kubectl get nodes
}
cmd_cluster-down() { need k3d; k3d cluster delete "$CLUSTER"; }

cmd_check() {
	need go
	local unformatted
	unformatted=$(gofmt -l .)
	if [ -n "$unformatted" ]; then echo "sin gofmt:"; echo "$unformatted"; exit 1; fi
	go vet ./...
	go test ./...
}

cmd_test-all() {
	cmd_check
	JAPPI_TEST_NATS_URL=nats://localhost:4222 \
	JAPPI_TEST_KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}" \
		go test -count=1 ./pkg/eventbus/... ./services/deployer/...
}

cmd_detect() {
	[ $# -eq 1 ] || { echo "uso: ./scripts/dev.sh detect <carpeta>" >&2; exit 2; }
	go run ./services/control-plane/cmd/jappi-detect "$1"
}

cmd_images() {
	need docker
	for dir in services/*/cmd/*/; do
		svc=$(basename "$dir")
		[ "$svc" = "jappi-detect" ] && continue
		echo "==> $svc"
		docker build --build-arg SERVICE="$svc" -t "localhost:5001/jappi-$svc:dev" .
	done
}

cmd_status() {
	echo "--- infraestructura (docker compose)"; $COMPOSE ps 2>/dev/null || echo "docker no disponible"
	echo "--- clústeres k3d"; k3d cluster list 2>/dev/null || echo "k3d no instalado"
}

command="${1:-help}"; shift || true
if declare -F "cmd_$command" >/dev/null; then
	"cmd_$command" "$@"
else
	echo "comando desconocido: $command" >&2; cmd_help; exit 2
fi
