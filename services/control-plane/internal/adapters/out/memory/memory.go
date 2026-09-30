// Package memory implementa los puertos de persistencia en memoria. Sirve
// para desarrollo local y pruebas mientras llega el adaptador de Postgres
// (ver migrations/0001_init.sql), que debe pasar las mismas pruebas.
package memory

import (
	"context"
	"sync"

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
