// Command deployer consume deploy.requested, aplica el estado deseado en
// K3s y publica deployment.status_changed.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JoseEscajadillo/JAPpi/pkg/eventbus/natsbus"
	eventsin "github.com/JoseEscajadillo/JAPpi/services/deployer/internal/adapters/in/events"
	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/adapters/out/k8s"
	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/app"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "deployer"))
	if err := run(); err != nil {
		slog.Error("el servicio terminó con error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// KUBECONFIG vacío = dentro del clúster, con la ServiceAccount del pod.
	client, err := k8s.NewClient(os.Getenv("KUBECONFIG"))
	if err != nil {
		return err
	}
	cluster := k8s.New(client, k8s.Config{
		RuntimeClass:     os.Getenv("RUNTIME_CLASS"), // "gvisor" en producción
		IngressClass:     env("INGRESS_CLASS", "traefik"),
		IngressNamespace: env("INGRESS_NAMESPACE", "kube-system"),
	})

	bus, err := natsbus.Connect(ctx, env("NATS_URL", "nats://localhost:4222"), "deployer")
	if err != nil {
		return err
	}
	defer bus.Close()
	// Cada despliegue espera su rollout (hasta 6 min): se atienden varios a la vez.
	bus.Workers = 16

	consumer := eventsin.Consumer{
		Bus: bus,
		Deploy: app.Deploy{
			Projects: cluster, Workloads: cluster, Rollouts: cluster, Events: bus,
			PollInterval: 3 * time.Second,
			Timeout:      6 * time.Minute, // algo más que progressDeadlineSeconds (5 min)
			Locks:        &app.KeyedMutex{},
		},
	}
	if err := consumer.Start(ctx); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv := &http.Server{Addr: env("HTTP_ADDR", ":8083"), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	slog.Info("escuchando", "addr", srv.Addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
