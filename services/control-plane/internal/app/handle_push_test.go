package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
	busmem "github.com/JoseEscajadillo/JAPpi/pkg/eventbus/memory"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/adapters/out/memory"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/app"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/project"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/stack"
)

func setup(t *testing.T) (app.HandlePush, *memory.Store, *[]events.BuildRequestedPayload) {
	t.Helper()
	store := memory.NewStore()
	store.AddProject(project.Project{
		ID: "p1", Slug: "acme", Repository: "acme/shop", Branch: "main",
		Plan: stack.Plan{Monorepo: true, Services: []stack.Service{
			{Name: "web", Path: "apps/web", Role: stack.RoleFrontend, PublicEnvPrefix: "VITE_"},
			{Name: "api", Path: "apps/api", Role: stack.RoleBackend, Needs: []stack.Addon{stack.AddonPostgres}},
		}},
		ServiceIDs: map[string]string{"web": "svc_web", "api": "svc_api"},
		Domains:    map[string]string{"web": "web.jappi.app", "api": "api.jappi.app"},
	})

	bus := busmem.New()
	var builds []events.BuildRequestedPayload
	_ = bus.Subscribe(context.Background(), "builder", []events.Type{events.BuildRequested}, func(_ context.Context, e events.Envelope) error {
		var p events.BuildRequestedPayload
		if err := e.Decode(&p); err != nil {
			return err
		}
		builds = append(builds, p)
		return nil
	})

	uc := app.HandlePush{Projects: store, Deployments: store, Events: bus, Now: time.Now}
	return uc, store, &builds
}

func TestPushOnlyRebuildsAffectedService(t *testing.T) {
	uc, store, builds := setup(t)
	push := events.RepoPushedPayload{
		Repository: "acme/shop", Branch: "main", CommitSHA: "abc123",
		ChangedFiles: []string{"apps/web/src/App.tsx"}, ChangedFilesComplete: true,
	}
	n, err := uc.Execute(context.Background(), push)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(*builds) != 1 || (*builds)[0].ServiceID != "svc_web" {
		t.Fatalf("solo debía construirse web; builds=%+v", *builds)
	}
	if got := (*builds)[0].BuildEnv["VITE_API_URL"]; got != "https://api.jappi.app" {
		t.Errorf("el build del frontend debe llevar la URL del backend, obtuve %q", got)
	}
	if len(store.Deployments()) != 1 {
		t.Errorf("esperaba 1 despliegue, hay %d", len(store.Deployments()))
	}
}

func TestRedeliveredPushIsIdempotent(t *testing.T) {
	uc, store, builds := setup(t)
	push := events.RepoPushedPayload{Repository: "acme/shop", Branch: "main", CommitSHA: "abc123"}

	for range 2 {
		if _, err := uc.Execute(context.Background(), push); err != nil {
			t.Fatal(err)
		}
	}
	if len(store.Deployments()) != 2 || len(*builds) != 2 {
		t.Fatalf("la reentrega duplicó trabajo: %d despliegues, %d builds", len(store.Deployments()), len(*builds))
	}
}

func TestSecretsNeverReachTheBuild(t *testing.T) {
	uc, _, builds := setup(t)
	push := events.RepoPushedPayload{Repository: "acme/shop", Branch: "main", CommitSHA: "def456"}
	if _, err := uc.Execute(context.Background(), push); err != nil {
		t.Fatal(err)
	}
	for _, b := range *builds {
		if _, leaked := b.BuildEnv["DATABASE_URL"]; leaked {
			t.Fatalf("DATABASE_URL no puede viajar en un BuildRequested: %+v", b)
		}
	}
}

func TestOtherBranchIsIgnored(t *testing.T) {
	uc, _, builds := setup(t)
	n, err := uc.Execute(context.Background(), events.RepoPushedPayload{Repository: "acme/shop", Branch: "feature/x", CommitSHA: "1"})
	if err != nil || n != 0 || len(*builds) != 0 {
		t.Fatalf("una rama no seguida no despliega: n=%d err=%v", n, err)
	}
}
