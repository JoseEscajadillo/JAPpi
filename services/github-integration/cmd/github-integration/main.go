// Command github-integration recibe los webhooks de la GitHub App y los
// publica como eventos RepoPushed.
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
	httpin "github.com/JoseEscajadillo/JAPpi/services/github-integration/internal/adapters/in/http"
	"github.com/JoseEscajadillo/JAPpi/services/github-integration/internal/app"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "github-integration"))
	if err := run(); err != nil {
		slog.Error("el servicio terminó con error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	secret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	if secret == "" {
		return errors.New("GITHUB_WEBHOOK_SECRET es obligatoria: sin ella cualquiera podría disparar despliegues")
	}
	bus, err := natsbus.Connect(ctx, env("NATS_URL", "nats://localhost:4222"), "github-integration")
	if err != nil {
		return err
	}
	defer bus.Close()

	handler := httpin.NewHandler(httpin.Webhook{
		Secret:      []byte(secret),
		ReceivePush: app.ReceivePush{Events: bus},
	})
	srv := &http.Server{Addr: env("HTTP_ADDR", ":8082"), Handler: handler, ReadHeaderTimeout: 5 * time.Second}
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
