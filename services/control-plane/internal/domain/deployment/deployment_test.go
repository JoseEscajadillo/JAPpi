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

func TestIDForIsDeterministic(t *testing.T) {
	if deployment.IDFor("p", "s", "a") != deployment.IDFor("p", "s", "a") {
		t.Error("mismo servicio y commit deben dar el mismo ID")
	}
	if deployment.IDFor("p", "s", "a") == deployment.IDFor("p", "s", "b") {
		t.Error("commits distintos deben dar IDs distintos")
	}
}
