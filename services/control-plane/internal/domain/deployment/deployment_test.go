package deployment_test

import (
	"errors"
	"testing"
	"time"

	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/deployment"
)

func TestHappyPath(t *testing.T) {
	d := deployment.New("d1", "p", "s", "sha", time.Now())
	for _, s := range []deployment.Status{deployment.Building, deployment.Deploying, deployment.Healthy, deployment.Superseded} {
		if err := d.TransitionTo(s, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if !d.Terminal() {
		t.Error("superseded es terminal")
	}
}

func TestCannotSkipTheBuild(t *testing.T) {
	d := deployment.New("d1", "p", "s", "sha", time.Now())
	if err := d.TransitionTo(deployment.Healthy, time.Now()); !errors.Is(err, deployment.ErrInvalidTransition) {
		t.Fatalf("queued → healthy debería fallar, obtuve %v", err)
	}
}

func TestAdvanceToToleratesOutOfOrderEvents(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		from        deployment.Status
		target      deployment.Status
		wantStatus  deployment.Status
		wantChanged bool
	}{
		{"salta pasos intermedios", deployment.Queued, deployment.Deploying, deployment.Deploying, true},
		{"healthy antes que deploying", deployment.Building, deployment.Healthy, deployment.Healthy, true},
		{"deploying llega tarde", deployment.Healthy, deployment.Deploying, deployment.Healthy, false},
		{"evento repetido", deployment.Deploying, deployment.Deploying, deployment.Deploying, false},
		{"falla desde building", deployment.Building, deployment.Failed, deployment.Failed, true},
		{"fallo tardío no deshace healthy", deployment.Healthy, deployment.Failed, deployment.Healthy, false},
		{"terminal no se toca", deployment.Superseded, deployment.Healthy, deployment.Superseded, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := deployment.Deployment{Status: tt.from}
			changed, err := d.AdvanceTo(tt.target, now)
			if err != nil {
				t.Fatal(err)
			}
			if d.Status != tt.wantStatus || changed != tt.wantChanged {
				t.Errorf("estado %s (changed=%v), quería %s (changed=%v)", d.Status, changed, tt.wantStatus, tt.wantChanged)
			}
		})
	}
}

func TestIDForIsDeterministic(t *testing.T) {
	id := deployment.IDFor("p", "s", "a")
	if got := deployment.IDFor("p", "s", "a"); got != id {
		t.Error("mismo servicio y commit deben dar el mismo ID")
	}
	if got := deployment.IDFor("p", "s", "b"); got == id {
		t.Error("commits distintos deben dar IDs distintos")
	}
}
