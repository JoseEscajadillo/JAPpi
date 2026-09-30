// Package events es el adaptador de entrada que traduce eventos del bus a
// llamadas a los casos de uso. No contiene lógica de negocio.
package events

import (
	"context"
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
	Bus        Subscriber
	HandlePush app.HandlePush
}

// Start se suscribe y devuelve; el consumo sigue hasta que ctx se cancela.
func (c Consumer) Start(ctx context.Context) error {
	return c.Bus.Subscribe(ctx, ConsumerName, []contracts.Type{contracts.RepoPushed}, c.onRepoPushed)
}

func (c Consumer) onRepoPushed(ctx context.Context, e contracts.Envelope) error {
	var push contracts.RepoPushedPayload
	if err := e.Decode(&push); err != nil {
		return contracts.Permanent(err)
	}
	n, err := c.HandlePush.Execute(ctx, push)
	if err != nil {
		return err
	}
	slog.Info("push procesado", "repo", push.Repository, "branch", push.Branch, "commit", push.CommitSHA, "builds", n)
	return nil
}
