// Command control-plane es la raíz de composición del servicio: lee la
// configuración, crea los adaptadores y se los inyecta a los casos de uso.
// Es el único sitio que conoce las implementaciones concretas.
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

	busmem "github.com/JoseEscajadillo/JAPpi/pkg/eventbus/memory"
	"github.com/JoseEscajadillo/JAPpi/pkg/eventbus/natsbus"
	eventsin "github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/adapters/in/events"
	httpin "github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/adapters/in/http"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/adapters/out/memory"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/app"
)

// bus es la unión de lo que necesitan los adaptadores de este servicio.
type bus interface {
	app.EventPublisher
	eventsin.Subscriber
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "control-plane"))
	if err := run(); err != nil {
		slog.Error("el servicio terminó con error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	b, closeBus, err := connectBus(ctx)
	if err != nil {
		return err
	}
	defer closeBus()

	// TODO(equipo): sustituir por el adaptador de Postgres cuando exista.
	store := memory.NewStore()

	consumer := eventsin.Consumer{
		Bus:        b,
		HandlePush: app.HandlePush{Projects: store, Deployments: store, Events: b, Now: time.Now},
	}
	if err := consumer.Start(ctx); err != nil {
		return err
	}

	srv := &http.Server{Addr: env("HTTP_ADDR", ":8081"), Handler: httpin.NewHandler(), ReadHeaderTimeout: 5 * time.Second}
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

// connectBus usa NATS si NATS_URL está definida y, si no, un bus en memoria
// (útil para levantar el servicio suelto mientras se desarrolla).
func connectBus(ctx context.Context) (bus, func(), error) {
	url := os.Getenv("NATS_URL")
	if url == "" {
		slog.Warn("NATS_URL vacía: usando bus en memoria (los eventos no salen de este proceso)")
		return busmem.New(), func() {}, nil
	}
	nb, err := natsbus.Connect(ctx, url, "control-plane")
	if err != nil {
		return nil, nil, err
	}
	return nb, func() { _ = nb.Close() }, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Ambas implementaciones del bus deben satisfacer lo que este servicio necesita.
var (
	_ bus = (*natsbus.Bus)(nil)
	_ bus = (*busmem.Bus)(nil)
)
