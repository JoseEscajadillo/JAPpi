// Package memory implementa los puertos de persistencia en memoria. Sirve
// para desarrollo local y pruebas mientras llega el adaptador de Postgres
// (ver migrations/0001_init.sql), que debe pasar las mismas pruebas.
package memory

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/app"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/deployment"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/project"
)

// Store guarda proyectos y despliegues.
type Store struct {
	mu          sync.RWMutex
	projects    []project.Project
	deployments map[string]deployment.Deployment
}

func NewStore() *Store {
	return &Store{deployments: map[string]deployment.Deployment{}}
}

// AddProject registra un proyecto (en producción lo hará el caso de uso CreateProject).
func (s *Store) AddProject(p project.Project) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projects = append(s.projects, p)
}

func (s *Store) FindByBranch(_ context.Context, repository, branch string) ([]project.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []project.Project
	for _, p := range s.projects {
		if p.Tracks(repository, branch) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *Store) SaveIfAbsent(_ context.Context, d deployment.Deployment) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.deployments[d.ID]; ok {
		return false, nil
	}
	s.deployments[d.ID] = d
	return true, nil
}

// Deployments devuelve una copia de los despliegues guardados.
func (s *Store) Deployments() []deployment.Deployment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]deployment.Deployment, 0, len(s.deployments))
	for _, d := range s.deployments {
		out = append(out, d)
	}
	return out
}

func (s *Store) Get(_ context.Context, projectID string) (project.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.projects {
		if p.ID == projectID {
			return p, nil
		}
	}
	return project.Project{}, fmt.Errorf("proyecto %s: %w", projectID, app.ErrNotFound)
}

func (s *Store) GetDeployment(_ context.Context, id string) (deployment.Deployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.deployments[id]
	if !ok {
		return deployment.Deployment{}, fmt.Errorf("despliegue %s: %w", id, app.ErrNotFound)
	}
	return d, nil
}

func (s *Store) UpdateDeployment(_ context.Context, d deployment.Deployment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.deployments[d.ID]; !ok {
		return fmt.Errorf("despliegue %s: %w", d.ID, app.ErrNotFound)
	}
	s.deployments[d.ID] = d
	return nil
}

// ListByService devuelve los despliegues de un servicio, del más nuevo al más viejo.
func (s *Store) ListByService(_ context.Context, serviceID string) ([]deployment.Deployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []deployment.Deployment
	for _, d := range s.deployments {
		if d.ServiceID == serviceID {
			out = append(out, d)
		}
	}
	slices.SortFunc(out, func(a, b deployment.Deployment) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return out, nil
}
