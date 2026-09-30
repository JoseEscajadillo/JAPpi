package app_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
	busmem "github.com/JoseEscajadillo/JAPpi/pkg/eventbus/memory"
	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/app"
	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/domain"
)

// fakeCluster simula el clúster: devuelve una secuencia de estados de rollout.
type fakeCluster struct {
	mu        sync.Mutex
	projects  []domain.Project
	workloads []domain.Workload
	statuses  []domain.RolloutStatus
	applyErr  error
}

func (c *fakeCluster) EnsureProject(_ context.Context, p domain.Project) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.projects = append(c.projects, p)
	return nil
}

func (c *fakeCluster) ApplyWorkload(_ context.Context, w domain.Workload) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.applyErr != nil {
		return c.applyErr
	}
	c.workloads = append(c.workloads, w)
	return nil
}

func (c *fakeCluster) Rollout(context.Context, string, string) (domain.RolloutStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.statuses[0]
	if len(c.statuses) > 1 {
		c.statuses = c.statuses[1:]
	}
	return s, nil
}

var (
	progressing = domain.RolloutStatus{Found: true, Generation: 1, ObservedGeneration: 1, Desired: 1, Updated: 1}
	complete    = domain.RolloutStatus{Found: true, Generation: 1, ObservedGeneration: 1, Desired: 1, Updated: 1, Available: 1}
	crashing    = domain.RolloutStatus{Found: true, Generation: 1, ObservedGeneration: 1, Desired: 1, Updated: 1, PodProblems: []string{"CrashLoopBackOff"}}
)

func spec() domain.Spec {
	return domain.Spec{
		DeploymentID: "dep_1", ProjectID: "p1", ServiceID: "svc_api", ServiceName: "api",
		Image: "r/api@sha256:" + strings.Repeat("b", 64), Port: 8080, Domain: "api.jappi.app", Tier: domain.TierHobby,
	}
}

func setup(t *testing.T, c *fakeCluster) (app.Deploy, *[]events.DeploymentStatusChangedPayload) {
	t.Helper()
	bus := busmem.New()
	var got []events.DeploymentStatusChangedPayload
	_ = bus.Subscribe(context.Background(), "test", []events.Type{events.DeploymentStatusChanged}, func(_ context.Context, e events.Envelope) error {
		var p events.DeploymentStatusChangedPayload
		_ = e.Decode(&p)
		got = append(got, p)
		return nil
	})
	return app.Deploy{
		Projects: c, Workloads: c, Rollouts: c, Events: bus,
		PollInterval: time.Millisecond, Timeout: time.Second,
	}, &got
}

func statuses(got []events.DeploymentStatusChangedPayload) string {
	var s []string
	for _, p := range got {
		s = append(s, p.Status)
	}
	return strings.Join(s, ",")
}

func TestHealthyRollout(t *testing.T) {
	c := &fakeCluster{statuses: []domain.RolloutStatus{{}, progressing, complete}}
	uc, got := setup(t, c)
	if err := uc.Execute(context.Background(), spec()); err != nil {
		t.Fatal(err)
	}
	if s := statuses(*got); s != "deploying,healthy" {
		t.Fatalf("estados = %s", s)
	}
	if len(c.projects) != 1 || c.projects[0].Namespace != "prj-p1" {
		t.Errorf("no preparó el namespace del proyecto: %+v", c.projects)
	}
}

func TestCrashLoopFailsFast(t *testing.T) {
	c := &fakeCluster{statuses: []domain.RolloutStatus{progressing, crashing}}
	uc, got := setup(t, c)
	if err := uc.Execute(context.Background(), spec()); err != nil {
		t.Fatal(err)
	}
	if s := statuses(*got); s != "deploying,failed" || !strings.Contains((*got)[1].Detail, "CrashLoopBackOff") {
		t.Fatalf("estados = %s, detalle = %+v", s, *got)
	}
}

func TestTimeoutIsReportedAsFailure(t *testing.T) {
	c := &fakeCluster{statuses: []domain.RolloutStatus{progressing}}
	uc, got := setup(t, c)
	uc.Timeout = 20 * time.Millisecond
	if err := uc.Execute(context.Background(), spec()); err != nil {
		t.Fatal(err)
	}
	if s := statuses(*got); s != "deploying,failed" {
		t.Fatalf("estados = %s", s)
	}
}

func TestInvalidSpecIsReportedWithoutTouchingTheCluster(t *testing.T) {
	c := &fakeCluster{statuses: []domain.RolloutStatus{complete}}
	uc, got := setup(t, c)
	bad := spec()
	bad.Image = "nginx:latest"
	if err := uc.Execute(context.Background(), bad); err != nil {
		t.Fatal(err)
	}
	if s := statuses(*got); s != "failed" || len(c.projects) != 0 {
		t.Fatalf("estados = %s, proyectos tocados = %d", s, len(c.projects))
	}
}

func TestClusterErrorIsRetried(t *testing.T) {
	c := &fakeCluster{statuses: []domain.RolloutStatus{complete}, applyErr: errors.New("connection refused")}
	uc, _ := setup(t, c)
	if err := uc.Execute(context.Background(), spec()); err == nil {
		t.Fatal("un error del clúster debe devolverse para que el bus reintente")
	}
}

func TestRedeliveryDoesNotDuplicateStatusEvents(t *testing.T) {
	c := &fakeCluster{statuses: []domain.RolloutStatus{complete}}
	uc, got := setup(t, c)
	for range 2 {
		if err := uc.Execute(context.Background(), spec()); err != nil {
			t.Fatal(err)
		}
	}
	if s := statuses(*got); s != "deploying,healthy" {
		t.Fatalf("la reentrega duplicó avisos: %s", s)
	}
}

// slowCluster cuenta cuántos rollouts del mismo servicio corren a la vez.
type slowCluster struct {
	fakeCluster
	maxMu          sync.Mutex
	inFlight, peak int
}

func (c *slowCluster) ApplyWorkload(ctx context.Context, w domain.Workload) error {
	c.maxMu.Lock()
	c.inFlight++
	c.peak = max(c.peak, c.inFlight)
	c.maxMu.Unlock()
	return nil
}

func (c *slowCluster) Rollout(context.Context, string, string) (domain.RolloutStatus, error) {
	time.Sleep(20 * time.Millisecond)
	c.maxMu.Lock()
	c.inFlight--
	c.maxMu.Unlock()
	return complete, nil
}

func TestSameServiceDeploysAreSerialized(t *testing.T) {
	c := &slowCluster{}
	uc, _ := setup(t, &c.fakeCluster)
	uc.Workloads, uc.Rollouts, uc.Locks = c, c, &app.KeyedMutex{}

	var wg sync.WaitGroup
	for i := range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := spec()
			s.DeploymentID = fmt.Sprintf("dep_%d", i)
			if err := uc.Execute(context.Background(), s); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if c.peak != 1 {
		t.Fatalf("hubo %d despliegues del mismo servicio a la vez", c.peak)
	}
}
