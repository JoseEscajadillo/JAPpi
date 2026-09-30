// Package deployment modela el ciclo de vida de un despliegue como una
// máquina de estados. Toda transición pasa por TransitionTo, así que es
// imposible, por ejemplo, marcar como "healthy" algo que nunca se construyó.
package deployment

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"time"
)

type Status string

const (
	Queued     Status = "queued"
	Building   Status = "building"
	Deploying  Status = "deploying"
	Healthy    Status = "healthy"
	Failed     Status = "failed"
	Superseded Status = "superseded" // lo reemplazó un deploy más nuevo
	RolledBack Status = "rolled_back"
)

var transitions = map[Status][]Status{
	Queued:    {Building, Failed, Superseded},
	Building:  {Deploying, Failed, Superseded},
	Deploying: {Healthy, Failed},
	Healthy:   {Superseded, RolledBack},
}

// ErrInvalidTransition indica un salto de estado no permitido.
var ErrInvalidTransition = errors.New("transición de estado inválida")

// Deployment es un intento de poner en producción un commit de un servicio.
type Deployment struct {
	ID        string
	ProjectID string
	ServiceID string
	CommitSHA string
	Image     string // se conoce al terminar el build
	Status    Status
	Detail    string // último mensaje para el usuario ("CrashLoopBackOff: ...")
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New crea un despliegue en cola.
func New(id, projectID, serviceID, commitSHA string, now time.Time) Deployment {
	return Deployment{
		ID: id, ProjectID: projectID, ServiceID: serviceID, CommitSHA: commitSHA,
		Status: Queued, CreatedAt: now, UpdatedAt: now,
	}
}

// IDFor devuelve un ID determinista para (servicio, commit). Si el evento que
// origina el deploy se reentrega, se obtiene el mismo ID y no se duplica.
func IDFor(projectID, serviceID, commitSHA string) string {
	sum := sha256.Sum256([]byte(projectID + "/" + serviceID + "@" + commitSHA))
	return "dep_" + hex.EncodeToString(sum[:])[:20]
}

// TransitionTo cambia el estado si la transición es válida.
func (d *Deployment) TransitionTo(s Status, now time.Time) error {
	if !slices.Contains(transitions[d.Status], s) {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, d.Status, s)
	}
	d.Status = s
	d.UpdatedAt = now
	return nil
}

// happyPath es el orden normal de estados de un despliegue.
var happyPath = []Status{Queued, Building, Deploying, Healthy}

// AdvanceTo lleva el despliegue hasta target recorriendo los pasos
// intermedios del camino normal. Los eventos pueden llegar desordenados o
// repetidos (un "healthy" antes que su "deploying"), así que:
//   - si el despliegue ya pasó por target, o ya terminó, no hace nada;
//   - si target es Failed, falla desde cualquier estado no terminal previo a Healthy.
//
// Devuelve changed=false cuando el evento era viejo o repetido.
func (d *Deployment) AdvanceTo(target Status, now time.Time) (changed bool, err error) {
	if d.Terminal() || d.Status == target {
		return false, nil
	}
	if target == Failed {
		if d.Status == Healthy {
			return false, nil // un fallo tardío no deshace un despliegue que ya sirvió
		}
		return true, d.TransitionTo(Failed, now)
	}
	from, to := slices.Index(happyPath, d.Status), slices.Index(happyPath, target)
	if to < 0 {
		return false, fmt.Errorf("%w: AdvanceTo solo admite estados del camino normal o failed, no %s", ErrInvalidTransition, target)
	}
	if from >= to {
		return false, nil
	}
	for _, s := range happyPath[from+1 : to+1] {
		if err := d.TransitionTo(s, now); err != nil {
			return false, err
		}
	}
	return true, nil
}

// Terminal indica si el despliegue ya no cambiará de estado por sí solo.
func (d Deployment) Terminal() bool {
	return d.Status == Failed || d.Status == Superseded || d.Status == RolledBack
}
