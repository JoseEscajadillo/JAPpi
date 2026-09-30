---
name: hexagonal-service
description: Crea el esqueleto de un microservicio Go nuevo de JAPpi (domain/app/adapters/cmd, arch test, main con apagado ordenado, conexión a NATS) o añade un caso de uso o adaptador a uno existente con la estructura hexagonal del repo. Úsala cuando haya que crear builder, deployer, addons, billing, logs u otro servicio, o un caso de uso nuevo.
---

# Servicio hexagonal en JAPpi

Antes de crear un servicio, confirma con la skill `systems-design` que de verdad hace falta uno nuevo y que existe su ADR. Toma como referencia `services/github-integration`, el servicio completo más pequeño.

## Estructura

```
services/<nombre>/
├── arch_test.go                         # obligatorio
├── cmd/<nombre>/main.go                 # raíz de composición
└── internal/
    ├── domain/                          # Go puro
    ├── app/                             # casos de uso + puertos
    └── adapters/
        ├── in/{http,events}/            # lo que llama al servicio
        └── out/<tecnología>/            # lo que el servicio llama
```

## Pasos

1. **`arch_test.go`** (package `<nombre sin guiones>_test`):
   ```go
   func TestHexagonalRules(t *testing.T) {
       archtest.CheckHexagon(t, ".", "github.com/JoseEscajadillo/JAPpi/services/<nombre>")
   }
   ```
2. **Dominio y pruebas.** Empieza por las reglas y sus tablas de casos, sin infraestructura.
3. **Caso de uso** en `app/<verbo_sustantivo>.go`: un struct cuyos campos son puertos, `Execute(ctx, ...)`, y los errores envueltos con contexto (`fmt.Errorf("...: %w", err)`).
4. **Puertos** en `app/ports.go`: interfaces pequeñas, las que `Execute` usa y nada más.
5. **Consumidor de eventos** en `adapters/in/events`:
   - consumidor durable con el nombre del servicio;
   - decodifica el payload y, si falla, devuelve `events.Permanent(err)`;
   - llama al caso de uso;
   - registra con `slog` el ID del evento.
6. **Adaptadores de salida.** Si un puerto va a tener dos implementaciones (memoria y Postgres), escribe una suite de contrato como `pkg/eventbus/bustest` y ejecútala contra ambas.
7. **`main.go`:**
   - `slog` en JSON con `service=<nombre>`;
   - `signal.NotifyContext` para SIGINT y SIGTERM;
   - configuración desde variables de entorno (valor por defecto solo si es seguro; los secretos, obligatorios);
   - conectar el bus e inyectar los adaptadores;
   - `http.Server` con `ReadHeaderTimeout` y `Shutdown` al cancelar el contexto;
   - `GET /healthz`.
8. **Registrar:**
   - evento nuevo → `docs/events.md`;
   - contenedor nuevo → C4 nivel 2;
   - tareas → `docs/roadmap.md`;
   - binario nuevo → matriz de la CI en `.github/workflows/ci.yml`.
9. **Verificar:** `gofmt -l . && go vet ./... && go test ./...`

## Convenciones de nombres

| Qué | Cómo |
|-----|------|
| Casos de uso | `VerboSustantivo` (`HandlePush`, `RequestRollback`) |
| Puertos | Por su capacidad (`DeploymentSaver`, `ImagePusher`), sin prefijo `I` ni sufijo `Interface` |
| Adaptadores | Por tecnología (`postgres.Store`, `natsbus.Bus`, `k8s.Applier`) |
| Paquetes | Minúsculas, sin guiones bajos ni plurales genéricos (`util`, `helpers`, `common` están prohibidos) |
