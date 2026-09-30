package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/app"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/deployment"
)

var digest = "@sha256:" + strings.Repeat("d", 64)

// deployFlow reúne los casos de uso que recorren un despliegue completo.
type deployFlow struct {
	push    app.HandlePush
	built   app.HandleBuildSucceeded
	failed  app.HandleBuildFailed
	status  app.HandleDeploymentStatus
	deploys *[]events.DeployRequestedPayload
}

func newFlow(t *testing.T) (deployFlow, func(id string) deployment.Deployment) {
	t.Helper()
	uc, store, _ := setup(t)
	bus := uc.Events.(interface {
		Subscribe(context.Context, string, []events.Type, events.Handler) error
	})
	var deploys []events.DeployRequestedPayload
	_ = bus.Subscribe(context.Background(), "deployer", []events.Type{events.DeployRequested}, func(_ context.Context, e events.Envelope) error {
		var p events.DeployRequestedPayload
		_ = e.Decode(&p)
		deploys = append(deploys, p)
		return nil
	})
	f := deployFlow{
		push:    uc,
		built:   app.HandleBuildSucceeded{Projects: store, Deployments: store, Events: uc.Events, Now: time.Now},
		failed:  app.HandleBuildFailed{Deployments: store, Now: time.Now},
		status:  app.HandleDeploymentStatus{Deployments: store, Now: time.Now},
		deploys: &deploys,
	}
	get := func(id string) deployment.Deployment {
		d, err := store.GetDeployment(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	return f, get
}

func pushWeb(t *testing.T, f deployFlow, commit string) string {
	t.Helper()
	ctx := context.Background()
	if _, err := f.push.Execute(ctx, events.RepoPushedPayload{
		Repository: "acme/shop", Branch: "main", CommitSHA: commit,
		ChangedFiles: []string{"apps/api/index.ts"}, ChangedFilesComplete: true,
	}); err != nil {
		t.Fatal(err)
	}
	return deployment.IDFor("p1", "svc_api", commit)
}

func TestBuildSucceededRequestsDeployWithWiredEnv(t *testing.T) {
	f, get := newFlow(t)
	id := pushWeb(t, f, "c1")

	if err := f.built.Execute(context.Background(), events.BuildSucceededPayload{DeploymentID: id, ServiceID: "svc_api", Image: "r/api" + digest}); err != nil {
		t.Fatal(err)
	}
	if d := get(id); d.Status != deployment.Deploying || d.Image != "r/api"+digest {
		t.Fatalf("despliegue = %+v", d)
	}
	if len(*f.deploys) != 1 {
		t.Fatalf("esperaba 1 deploy.requested, hubo %d", len(*f.deploys))
	}
	spec := (*f.deploys)[0]
	if spec.Domain != "api.jappi.app" || spec.Tier != "trial" || spec.ServiceName != "api" {
		t.Errorf("spec incompleta: %+v", spec)
	}
	var db *events.EnvVar
	for i := range spec.Env {
		if spec.Env[i].Name == "DATABASE_URL" {
			db = &spec.Env[i]
		}
	}
	if db == nil || db.Value != "" || db.SecretRef == nil || db.SecretRef.Name != "jappi-addon-postgres" {
		t.Errorf("DATABASE_URL debe viajar como referencia al Secret del add-on: %+v", db)
	}
}

func TestHealthySupersedesThePreviousDeployment(t *testing.T) {
	f, get := newFlow(t)
	ctx := context.Background()
	first := pushWeb(t, f, "c1")
	_ = f.built.Execute(ctx, events.BuildSucceededPayload{DeploymentID: first, Image: "r/api" + digest})
	_ = f.status.Execute(ctx, events.DeploymentStatusChangedPayload{DeploymentID: first, Status: events.StatusHealthy})

	second := pushWeb(t, f, "c2")
	_ = f.built.Execute(ctx, events.BuildSucceededPayload{DeploymentID: second, Image: "r/api" + digest})
	if err := f.status.Execute(ctx, events.DeploymentStatusChangedPayload{DeploymentID: second, Status: events.StatusHealthy}); err != nil {
		t.Fatal(err)
	}

	if get(first).Status != deployment.Superseded || get(second).Status != deployment.Healthy {
		t.Fatalf("primero=%s segundo=%s", get(first).Status, get(second).Status)
	}
}

func TestOldBuildFinishingLateDoesNotOverwriteNewerDeploy(t *testing.T) {
	f, get := newFlow(t)
	ctx := context.Background()
	old := pushWeb(t, f, "c1")
	time.Sleep(time.Millisecond) // CreatedAt estrictamente posterior
	newer := pushWeb(t, f, "c2")

	// El build nuevo termina primero y queda sano.
	_ = f.built.Execute(ctx, events.BuildSucceededPayload{DeploymentID: newer, Image: "r/api@sha256:" + strings.Repeat("2", 64)})
	_ = f.status.Execute(ctx, events.DeploymentStatusChangedPayload{DeploymentID: newer, Status: events.StatusHealthy})
	// Después termina el viejo.
	if err := f.built.Execute(ctx, events.BuildSucceededPayload{DeploymentID: old, Image: "r/api@sha256:" + strings.Repeat("1", 64)}); err != nil {
		t.Fatal(err)
	}

	if get(old).Status != deployment.Superseded {
		t.Fatalf("el build viejo debía quedar reemplazado, está en %s", get(old).Status)
	}
	if len(*f.deploys) != 1 || (*f.deploys)[0].DeploymentID != newer {
		t.Fatalf("solo el despliegue nuevo debía llegar al deployer: %+v", *f.deploys)
	}
}

func TestLateDeployingStatusDoesNotRegress(t *testing.T) {
	f, get := newFlow(t)
	ctx := context.Background()
	id := pushWeb(t, f, "c1")
	_ = f.built.Execute(ctx, events.BuildSucceededPayload{DeploymentID: id, Image: "r/api" + digest})
	_ = f.status.Execute(ctx, events.DeploymentStatusChangedPayload{DeploymentID: id, Status: events.StatusHealthy})
	_ = f.status.Execute(ctx, events.DeploymentStatusChangedPayload{DeploymentID: id, Status: events.StatusDeploying})

	if s := get(id).Status; s != deployment.Healthy {
		t.Fatalf("un evento viejo hizo retroceder el estado a %s", s)
	}
}

func TestBuildFailedMarksDeploymentFailed(t *testing.T) {
	f, get := newFlow(t)
	id := pushWeb(t, f, "c1")
	if err := f.failed.Execute(context.Background(), events.BuildFailedPayload{DeploymentID: id, Reason: "npm ERR! missing script: build"}); err != nil {
		t.Fatal(err)
	}
	d := get(id)
	if d.Status != deployment.Failed || !strings.Contains(d.Detail, "missing script") {
		t.Fatalf("despliegue = %+v", d)
	}
	if len(*f.deploys) != 0 {
		t.Fatal("un build fallido no puede pedir un despliegue")
	}
}

func TestUnknownDeploymentIsPermanentError(t *testing.T) {
	f, _ := newFlow(t)
	err := f.built.Execute(context.Background(), events.BuildSucceededPayload{DeploymentID: "no-existe"})
	if !errors.Is(err, events.ErrPermanent) {
		t.Fatalf("un despliegue inexistente no se arregla reintentando: %v", err)
	}
}
