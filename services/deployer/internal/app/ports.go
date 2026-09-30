// Package app contiene el caso de uso del deployer: llevar una
// especificación al clúster y vigilar el rollout hasta que termine.
package app

import (
	"context"

	"github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/domain"
)

// ProjectProvisioner asegura que el entorno aislado del proyecto existe
// (namespace, cuotas, políticas de red). Debe ser idempotente.
type ProjectProvisioner interface {
	EnsureProject(ctx context.Context, p domain.Project) error
}

// WorkloadApplier aplica el estado deseado de un servicio. Idempotente.
type WorkloadApplier interface {
	ApplyWorkload(ctx context.Context, w domain.Workload) error
}

// RolloutReader observa el progreso de un servicio.
type RolloutReader interface {
	Rollout(ctx context.Context, namespace, name string) (domain.RolloutStatus, error)
}

// EventPublisher publica eventos en el bus.
type EventPublisher interface {
	Publish(ctx context.Context, e events.Envelope) error
}
