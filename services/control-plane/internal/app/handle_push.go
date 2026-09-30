package app

import (
	"context"
	"fmt"
	"time"

	"github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/deployment"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/stack"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/wiring"
)

// HandlePush convierte un push en despliegues: por cada proyecto que sigue
// la rama, crea un despliegue por servicio afectado y pide su build.
//
// Es idempotente: si el evento se reentrega, los IDs deterministas hacen que
// no se dupliquen ni los despliegues ni las peticiones de build.
type HandlePush struct {
	Projects    ProjectFinder
	Deployments DeploymentSaver
	Events      EventPublisher
	Now         func() time.Time
}

// Execute devuelve cuántos builds se pidieron.
func (uc HandlePush) Execute(ctx context.Context, push events.RepoPushedPayload) (int, error) {
	projects, err := uc.Projects.FindByBranch(ctx, push.Repository, push.Branch)
	if err != nil {
		return 0, fmt.Errorf("buscar proyectos de %s@%s: %w", push.Repository, push.Branch, err)
	}

	requested := 0
	for _, p := range projects {
		vars := wiring.Resolve(wiring.Input{Plan: p.Plan, Domains: p.Domains, UserVars: p.UserVars})

		for _, svc := range stack.AffectedServices(p.Plan, push.ChangedFiles, push.ChangedFilesComplete) {
			serviceID := p.ServiceIDs[svc.Name]
			d := deployment.New(deployment.IDFor(p.ID, serviceID, push.CommitSHA), p.ID, serviceID, push.CommitSHA, uc.Now())
			if _, err := uc.Deployments.SaveIfAbsent(ctx, d); err != nil {
				return requested, fmt.Errorf("guardar despliegue %s: %w", d.ID, err)
			}

			// Publicamos aunque el despliegue ya existiera: si el intento
			// anterior falló justo después de guardar, el build nunca se pidió.
			// El ID fijo del evento hace que el bus descarte la repetición.
			e, err := events.NewWithID("build-"+d.ID, events.BuildRequested, p.ID, events.BuildRequestedPayload{
				DeploymentID: d.ID,
				ServiceID:    serviceID,
				Repository:   push.Repository,
				CommitSHA:    push.CommitSHA,
				ServicePath:  svc.Path,
				BuildEnv:     buildEnv(vars.Services[svc.Name]),
			})
			if err != nil {
				return requested, err
			}
			if err := uc.Events.Publish(ctx, e); err != nil {
				return requested, fmt.Errorf("pedir build de %s: %w", d.ID, err)
			}
			requested++
		}
	}
	return requested, nil
}

// buildEnv reúne las variables que el framework necesita en el build. Solo
// entran literales: una referencia a un secreto nunca debe acabar incrustada
// en un bundle que se sirve al navegador.
//
// TODO(equipo): las variables del usuario marcadas como "build" también deben
// entrar aquí; hoy UserVars no guarda esa marca.
func buildEnv(injected []wiring.Var) map[string]string {
	env := map[string]string{}
	for _, v := range injected {
		if v.BuildTime && v.Ref == nil {
			env[v.Name] = v.Value
		}
	}
	return env
}
