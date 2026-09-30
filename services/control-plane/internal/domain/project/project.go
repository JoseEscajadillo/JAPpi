// Package project modela un proyecto de JAPpi: un repositorio conectado,
// su plan de servicios y los dominios reservados para cada uno.
package project

import "github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/stack"

// Project es la raíz del agregado.
type Project struct {
	ID         string
	AccountID  string
	Slug       string
	Repository string // "owner/name"
	Branch     string // rama que se despliega en producción
	Plan       stack.Plan
	// ServiceIDs asigna a cada nombre de servicio del plan su ID persistente.
	ServiceIDs map[string]string
	// Domains asigna a cada servicio su dominio reservado (ADR-0009).
	Domains map[string]string
	// UserVars son las variables que el usuario definió a mano, por servicio.
	UserVars map[string]map[string]string
}

// Tracks indica si un push a (repo, rama) debe desplegar este proyecto.
func (p Project) Tracks(repository, branch string) bool {
	return p.Repository == repository && p.Branch == branch
}
