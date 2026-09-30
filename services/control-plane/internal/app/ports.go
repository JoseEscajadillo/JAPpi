// Package app contiene los casos de uso del control-plane.
//
// Los puertos (interfaces) se declaran aquí, en el lado que los consume, y
// son pequeños: cada caso de uso pide solo lo que usa (segregación de
// interfaces). Los adaptadores de internal/adapters los implementan y
// cmd/control-plane los conecta (inversión de dependencias).
package app

import (
	"context"
	"errors"
	"io/fs"

	"github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/deployment"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/project"
)

// ErrNotFound lo devuelven los adaptadores cuando no existe lo que se busca.
var ErrNotFound = errors.New("no encontrado")

// ProjectFinder encuentra los proyectos que siguen una rama de un repo.
type ProjectFinder interface {
	FindByBranch(ctx context.Context, repository, branch string) ([]project.Project, error)
}

// DeploymentSaver guarda un despliegue nuevo. Devuelve created=false si ya
// existía uno con ese ID (reentrega del mismo evento).
type DeploymentSaver interface {
	SaveIfAbsent(ctx context.Context, d deployment.Deployment) (created bool, err error)
}

// EventPublisher publica eventos en el bus.
type EventPublisher interface {
	Publish(ctx context.Context, e events.Envelope) error
}

// RepoSource entrega los archivos de un repo en un commit o rama.
type RepoSource interface {
	Open(ctx context.Context, repository, ref string) (fs.FS, error)
}

// ProjectGetter obtiene un proyecto por su ID (o ErrNotFound).
type ProjectGetter interface {
	Get(ctx context.Context, projectID string) (project.Project, error)
}

// DeploymentStore lee y actualiza despliegues existentes.
type DeploymentStore interface {
	GetDeployment(ctx context.Context, id string) (deployment.Deployment, error)
	UpdateDeployment(ctx context.Context, d deployment.Deployment) error
	ListByService(ctx context.Context, serviceID string) ([]deployment.Deployment, error)
}
