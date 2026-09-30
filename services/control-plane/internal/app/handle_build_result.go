package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/deployment"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/project"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/stack"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/wiring"
)

// HandleBuildSucceeded pasa un despliegue a "deploying" y le pide al
// deployer que ponga la imagen en producción con la especificación completa.
type HandleBuildSucceeded struct {
	Projects    ProjectGetter
	Deployments DeploymentStore
	Events      EventPublisher
	Now         func() time.Time
}

func (uc HandleBuildSucceeded) Execute(ctx context.Context, b events.BuildSucceededPayload) error {
	d, err := uc.Deployments.GetDeployment(ctx, b.DeploymentID)
	if err != nil {
		return notFoundIsPermanent(err)
	}
	if d.Terminal() {
		return nil // otro push lo reemplazó mientras se construía
	}
	p, err := uc.Projects.Get(ctx, d.ProjectID)
	if err != nil {
		return notFoundIsPermanent(err)
	}
	svc, ok := p.ServiceByID(d.ServiceID)
	if !ok {
		return events.Permanent(fmt.Errorf("el servicio %s ya no existe en el proyecto %s", d.ServiceID, p.ID))
	}

	// Un build viejo que termina tarde no puede pisar a uno más nuevo que
	// ya está desplegándose o sirviendo.
	newer, err := uc.newerInProduction(ctx, d)
	if err != nil {
		return err
	}
	if newer {
		if err := d.TransitionTo(deployment.Superseded, uc.Now()); err != nil {
			return events.Permanent(err)
		}
		d.Detail = "reemplazado por un despliegue más reciente antes de publicarse"
		return uc.Deployments.UpdateDeployment(ctx, d)
	}

	if _, err := d.AdvanceTo(deployment.Deploying, uc.Now()); err != nil {
		return events.Permanent(err)
	}
	d.Image = b.Image
	if err := uc.Deployments.UpdateDeployment(ctx, d); err != nil {
		return fmt.Errorf("guardar despliegue %s: %w", d.ID, err)
	}
	if d.Status != deployment.Deploying {
		return nil // ya llegó más lejos (reentrega tardía)
	}

	// ID fijo: si esto se reintenta, el bus descarta la repetición.
	e, err := events.NewWithID("deploy-"+d.ID, events.DeployRequested, p.ID, DeploySpec(p, svc, d))
	if err != nil {
		return err
	}
	if err := uc.Events.Publish(ctx, e); err != nil {
		return fmt.Errorf("pedir despliegue de %s: %w", d.ID, err)
	}
	return nil
}

func (uc HandleBuildSucceeded) newerInProduction(ctx context.Context, d deployment.Deployment) (bool, error) {
	all, err := uc.Deployments.ListByService(ctx, d.ServiceID)
	if err != nil {
		return false, err
	}
	for _, other := range all {
		if other.ID != d.ID && other.CreatedAt.After(d.CreatedAt) &&
			(other.Status == deployment.Deploying || other.Status == deployment.Healthy) {
			return true, nil
		}
	}
	return false, nil
}

// DeploySpec arma la especificación que recibe el deployer. Lleva las
// variables cableadas: literales tal cual y secretos como referencias.
//
// TODO(equipo): las variables definidas por el usuario no viajan todavía,
// porque pueden ser secretas y no deben ir en un evento (ADR-0011).
func DeploySpec(p project.Project, svc stack.Service, d deployment.Deployment) events.DeployRequestedPayload {
	vars := wiring.Resolve(wiring.Input{Plan: p.Plan, Domains: p.Domains, UserVars: p.UserVars}).Services[svc.Name]
	env := make([]events.EnvVar, 0, len(vars))
	for _, v := range vars {
		if v.Ref != nil {
			env = append(env, events.EnvVar{Name: v.Name, SecretRef: &events.SecretRef{
				Name: events.AddonSecretName(string(v.Ref.Addon)),
				Key:  v.Ref.Key,
			}})
			continue
		}
		env = append(env, events.EnvVar{Name: v.Name, Value: v.Value})
	}
	return events.DeployRequestedPayload{
		DeploymentID: d.ID,
		ProjectID:    p.ID,
		ServiceID:    d.ServiceID,
		ServiceName:  svc.Name,
		Image:        d.Image,
		Port:         svc.Port,
		Domain:       p.Domains[svc.Name],
		Tier:         p.TierOrDefault(),
		Env:          env,
	}
}

// HandleBuildFailed marca el despliegue como fallido.
type HandleBuildFailed struct {
	Deployments DeploymentStore
	Now         func() time.Time
}

func (uc HandleBuildFailed) Execute(ctx context.Context, b events.BuildFailedPayload) error {
	d, err := uc.Deployments.GetDeployment(ctx, b.DeploymentID)
	if err != nil {
		return notFoundIsPermanent(err)
	}
	changed, err := d.AdvanceTo(deployment.Failed, uc.Now())
	if err != nil || !changed {
		return err
	}
	d.Detail = "el build falló: " + b.Reason
	return uc.Deployments.UpdateDeployment(ctx, d)
}

// HandleDeploymentStatus registra lo que informa el deployer. Cuando un
// despliegue queda sano, el que servía antes pasa a "superseded": así el
// historial siempre tiene un único despliegue activo por servicio, que es
// el punto de partida del rollback.
type HandleDeploymentStatus struct {
	Deployments DeploymentStore
	Now         func() time.Time
}

var deployerStatuses = map[string]deployment.Status{
	events.StatusDeploying: deployment.Deploying,
	events.StatusHealthy:   deployment.Healthy,
	events.StatusFailed:    deployment.Failed,
}

func (uc HandleDeploymentStatus) Execute(ctx context.Context, s events.DeploymentStatusChangedPayload) error {
	target, ok := deployerStatuses[s.Status]
	if !ok {
		return events.Permanent(fmt.Errorf("estado desconocido %q", s.Status))
	}
	d, err := uc.Deployments.GetDeployment(ctx, s.DeploymentID)
	if err != nil {
		return notFoundIsPermanent(err)
	}
	changed, err := d.AdvanceTo(target, uc.Now())
	if err != nil {
		return events.Permanent(err)
	}
	if !changed {
		return nil
	}
	d.Detail = s.Detail
	if err := uc.Deployments.UpdateDeployment(ctx, d); err != nil {
		return err
	}
	if d.Status != deployment.Healthy {
		return nil
	}

	previous, err := uc.Deployments.ListByService(ctx, d.ServiceID)
	if err != nil {
		return err
	}
	for _, old := range previous {
		if old.ID == d.ID || old.Status != deployment.Healthy {
			continue
		}
		if err := old.TransitionTo(deployment.Superseded, uc.Now()); err != nil {
			return events.Permanent(err)
		}
		if err := uc.Deployments.UpdateDeployment(ctx, old); err != nil {
			return err
		}
	}
	return nil
}

func notFoundIsPermanent(err error) error {
	if errors.Is(err, ErrNotFound) {
		return events.Permanent(err)
	}
	return err
}
