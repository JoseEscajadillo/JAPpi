// Package events traduce deploy.requested al caso de uso Deploy.
package events

import (
	"context"
	"log/slog"

	contracts "github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/app"
	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/domain"
)

// ConsumerName es el consumidor durable del deployer en el bus.
const ConsumerName = "deployer"

// Subscriber es lo que este adaptador necesita del bus.
type Subscriber interface {
	Subscribe(ctx context.Context, consumer string, types []contracts.Type, h contracts.Handler) error
}

// Consumer conecta el bus con el caso de uso.
type Consumer struct {
	Bus    Subscriber
	Deploy app.Deploy
}

// Start se suscribe; el consumo sigue hasta que ctx se cancela.
func (c Consumer) Start(ctx context.Context) error {
	return c.Bus.Subscribe(ctx, ConsumerName, []contracts.Type{contracts.DeployRequested}, c.onDeployRequested)
}

func (c Consumer) onDeployRequested(ctx context.Context, e contracts.Envelope) error {
	var p contracts.DeployRequestedPayload
	if err := e.Decode(&p); err != nil {
		return contracts.Permanent(err)
	}
	slog.Info("desplegando", "event", e.ID, "deployment", p.DeploymentID, "service", p.ServiceName, "image", p.Image)
	return c.Deploy.Execute(ctx, toSpec(p))
}

func toSpec(p contracts.DeployRequestedPayload) domain.Spec {
	env := make([]domain.EnvVar, 0, len(p.Env))
	for _, v := range p.Env {
		ev := domain.EnvVar{Name: v.Name, Value: v.Value}
		if v.SecretRef != nil {
			ev.Secret = &domain.SecretKey{Name: v.SecretRef.Name, Key: v.SecretRef.Key}
		}
		env = append(env, ev)
	}
	return domain.Spec{
		DeploymentID: p.DeploymentID,
		ProjectID:    p.ProjectID,
		ServiceID:    p.ServiceID,
		ServiceName:  p.ServiceName,
		Image:        p.Image,
		Port:         p.Port,
		Domain:       p.Domain,
		Tier:         domain.Tier(p.Tier),
		Env:          env,
	}
}
