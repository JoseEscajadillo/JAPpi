package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Project es el entorno aislado de un proyecto en el clúster.
type Project struct {
	ProjectID string
	Namespace string
	Labels    map[string]string
	Limits    Limits
}

// Workload es todo lo que el clúster necesita para correr un servicio.
type Workload struct {
	Namespace    string
	Name         string
	DeploymentID string
	Image        string
	Port         int
	Host         string
	Env          []EnvVar
	Labels       map[string]string // identifican el servicio: estables entre despliegues
	Limits       Limits
}

// Labels comunes a todo lo que crea JAPpi. Los selectores usan solo las
// estables (proyecto y servicio), nunca el ID del despliegue.
const (
	LabelManagedBy = "app.kubernetes.io/managed-by"
	LabelName      = "app.kubernetes.io/name"
	LabelProject   = "jappi.dev/project"
	LabelService   = "jappi.dev/service"
	ManagedBy      = "jappi"
)

// Plan convierte una especificación válida en el proyecto y la carga de
// trabajo que el adaptador debe aplicar.
func Plan(s Spec) (Project, Workload, error) {
	if err := s.Validate(); err != nil {
		return Project{}, Workload{}, err
	}
	limits, _ := LimitsFor(s.Tier) // Validate ya comprobó el plan

	ns := Namespace(s.ProjectID)
	project := Project{
		ProjectID: s.ProjectID,
		Namespace: ns,
		Labels:    map[string]string{LabelManagedBy: ManagedBy, LabelProject: labelValue(s.ProjectID)},
		Limits:    limits,
	}
	workload := Workload{
		Namespace:    ns,
		Name:         s.ServiceName,
		DeploymentID: s.DeploymentID,
		Image:        s.Image,
		Port:         s.Port,
		Host:         s.Domain,
		Env:          s.Env,
		Labels: map[string]string{
			LabelManagedBy: ManagedBy,
			LabelName:      s.ServiceName,
			LabelProject:   labelValue(s.ProjectID),
			LabelService:   labelValue(s.ServiceID),
		},
		Limits: limits,
	}
	return project, workload, nil
}

// Namespace devuelve el namespace de un proyecto: "prj-<id>". Si el ID es
// demasiado largo se recorta y se añade un hash para no colisionar.
func Namespace(projectID string) string {
	name := "prj-" + sanitize(projectID)
	if len(name) <= 63 {
		return name
	}
	sum := sha256.Sum256([]byte(projectID))
	return strings.TrimRight(name[:63-9], "-") + "-" + hex.EncodeToString(sum[:])[:8]
}

// labelValue ajusta un valor al formato de las labels de Kubernetes (63
// caracteres, alfanuméricos, '-', '_' y '.').
func labelValue(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := b.String()
	if len(out) > 63 {
		out = out[:63]
	}
	return strings.Trim(out, "-_.")
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
