package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/domain"
)

// Deploy pone una especificación en el clúster e informa del resultado.
//
// Es idempotente: aplicar dos veces el mismo estado deseado no cambia nada
// en Kubernetes, y los eventos de estado usan IDs fijos por despliegue y
// estado, así que una reentrega no genera avisos duplicados.
type Deploy struct {
	Projects  ProjectProvisioner
	Workloads WorkloadApplier
	Rollouts  RolloutReader
	Events    EventPublisher

	// PollInterval es cada cuánto se revisa el rollout.
	PollInterval time.Duration
	// Timeout es el máximo que se espera a que el rollout termine. Debe ser
	// algo mayor que progressDeadlineSeconds del Deployment.
	Timeout time.Duration

	// Locks serializa los despliegues de un mismo servicio. Debe
	// compartirse (puntero) entre todas las copias del caso de uso.
	Locks *KeyedMutex
}

// KeyedMutex es un candado por clave. El valor cero está listo para usarse.
type KeyedMutex struct {
	mu    sync.Mutex
	locks map[string]*keyedLock
}

type keyedLock struct {
	mu      sync.Mutex
	waiters int
}

func (k *KeyedMutex) lock(key string) (unlock func()) {
	if k == nil {
		return func() {}
	}
	k.mu.Lock()
	if k.locks == nil {
		k.locks = map[string]*keyedLock{}
	}
	l, ok := k.locks[key]
	if !ok {
		l = &keyedLock{}
		k.locks[key] = l
	}
	l.waiters++
	k.mu.Unlock()

	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		k.mu.Lock()
		if l.waiters--; l.waiters == 0 {
			delete(k.locks, key) // no acumular una entrada por servicio para siempre
		}
		k.mu.Unlock()
	}
}

// Execute despliega spec. Devuelve error solo si merece reintento (el
// clúster no respondió); los fallos del despliegue en sí se informan con
// un evento de estado y devuelven nil.
func (uc Deploy) Execute(ctx context.Context, spec domain.Spec) error {
	project, workload, err := domain.Plan(spec)
	if err != nil {
		return uc.publish(ctx, spec, events.StatusFailed, err.Error())
	}
	// Dos despliegues del mismo servicio no se aplican a la vez: el segundo
	// espera a que el primero termine. (Dentro de una réplica; con varias
	// réplicas del deployer hará falta un candado distribuido: ver
	// docs/system-design.md, cuello de botella 3.)
	unlock := uc.Locks.lock(workload.Namespace + "/" + workload.Name)
	defer unlock()

	if err := uc.publish(ctx, spec, events.StatusDeploying, "aplicando en el clúster"); err != nil {
		return err
	}
	if err := uc.Projects.EnsureProject(ctx, project); err != nil {
		return fmt.Errorf("preparar el namespace %s: %w", project.Namespace, err)
	}
	if err := uc.Workloads.ApplyWorkload(ctx, workload); err != nil {
		return fmt.Errorf("aplicar %s/%s: %w", workload.Namespace, workload.Name, err)
	}

	phase, detail, err := uc.waitRollout(ctx, workload)
	if err != nil {
		return err
	}
	status := events.StatusHealthy
	if phase == domain.Failed {
		status = events.StatusFailed
	}
	return uc.publish(ctx, spec, status, detail)
}

var errTimeout = errors.New("tiempo de espera agotado")

func (uc Deploy) waitRollout(ctx context.Context, w domain.Workload) (domain.Phase, string, error) {
	deadline := time.Now().Add(uc.Timeout)
	ticker := time.NewTicker(uc.PollInterval)
	defer ticker.Stop()

	for {
		status, err := uc.Rollouts.Rollout(ctx, w.Namespace, w.Name)
		if err != nil {
			return 0, "", fmt.Errorf("leer el rollout de %s/%s: %w", w.Namespace, w.Name, err)
		}
		phase, detail := domain.Evaluate(status)
		if phase != domain.Progressing {
			return phase, detail, nil
		}
		if time.Now().After(deadline) {
			return domain.Failed, fmt.Sprintf("%v tras %s: %s", errTimeout, uc.Timeout, detail), nil
		}
		select {
		case <-ctx.Done():
			return 0, "", ctx.Err()
		case <-ticker.C:
		}
	}
}

func (uc Deploy) publish(ctx context.Context, spec domain.Spec, status, detail string) error {
	e, err := events.NewWithID("status-"+spec.DeploymentID+"-"+status, events.DeploymentStatusChanged, spec.ProjectID,
		events.DeploymentStatusChangedPayload{
			DeploymentID: spec.DeploymentID,
			ServiceID:    spec.ServiceID,
			Status:       status,
			Detail:       detail,
		})
	if err != nil {
		return err
	}
	if err := uc.Events.Publish(ctx, e); err != nil {
		return fmt.Errorf("publicar estado %s de %s: %w", status, spec.DeploymentID, err)
	}
	return nil
}
