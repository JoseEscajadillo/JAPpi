// Package events es el adaptador de entrada que traduce eventos del bus a
// llamadas a los casos de uso. No contiene lógica de negocio.
package events

import (
	"context"
	"fmt"
	"log/slog"

	contracts "github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/app"
)

// ConsumerName es el consumidor durable del control-plane en el bus.
const ConsumerName = "control-plane"

// Subscriber es lo que este adaptador necesita del bus.
type Subscriber interface {
	Subscribe(ctx context.Context, consumer string, types []contracts.Type, h contracts.Handler) error
}

// Consumer conecta el bus con los casos de uso.
type Consumer struct {
	Bus                    Subscriber
	HandlePush             app.HandlePush
	HandleBuildSucceeded   app.HandleBuildSucceeded
	HandleBuildFailed      app.HandleBuildFailed
	HandleDeploymentStatus app.HandleDeploymentStatus
}

// Start se suscribe y devuelve; el consumo sigue hasta que ctx se cancela.
func (c Consumer) Start(ctx context.Context) error {
	return c.Bus.Subscribe(ctx, ConsumerName, []contracts.Type{
		contracts.RepoPushed,
		contracts.BuildSucceeded,
		contracts.BuildFailed,
		contracts.DeploymentStatusChanged,
	}, c.dispatch)
}

func (c Consumer) dispatch(ctx context.Context, e contracts.Envelope) error {
	switch e.Type {
	case contracts.RepoPushed:
		return handle(ctx, e, func(ctx context.Context, p contracts.RepoPushedPayload) error {
			n, err := c.HandlePush.Execute(ctx, p)
			if err == nil {
				slog.Info("push procesado", "event", e.ID, "repo", p.Repository, "branch", p.Branch, "commit", p.CommitSHA, "builds", n)
			}
			return err
		})
	case contracts.BuildSucceeded:
		return handle(ctx, e, c.HandleBuildSucceeded.Execute)
	case contracts.BuildFailed:
		return handle(ctx, e, c.HandleBuildFailed.Execute)
	case contracts.DeploymentStatusChanged:
		return handle(ctx, e, c.HandleDeploymentStatus.Execute)
	default:
		return contracts.Permanent(fmt.Errorf("tipo de evento no esperado: %s", e.Type))
	}
}

// handle decodifica el payload del tipo P y llama al caso de uso.
func handle[P any](ctx context.Context, e contracts.Envelope, execute func(context.Context, P) error) error {
	var p P
	if err := e.Decode(&p); err != nil {
		return contracts.Permanent(err)
	}
	return execute(ctx, p)
}
