// Package domain contiene las reglas del deployer: qué se considera una
// especificación válida, cómo se llaman las cosas en el clúster, cuántos
// recursos da cada plan y cuándo un despliegue terminó bien o mal.
//
// No sabe nada de Kubernetes: el adaptador out/k8s traduce estas decisiones
// a objetos de la API.
package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrInvalidSpec indica una especificación que no se puede desplegar.
// Reintentar no la arregla.
var ErrInvalidSpec = errors.New("especificación inválida")

// Spec es lo que tiene que quedar corriendo para un servicio.
type Spec struct {
	DeploymentID string
	ProjectID    string
	ServiceID    string
	ServiceName  string
	Image        string
	Port         int
	Domain       string
	Tier         Tier
	Env          []EnvVar
}

// EnvVar es un literal o una referencia a un Secret del namespace.
type EnvVar struct {
	Name   string
	Value  string
	Secret *SecretKey
}

// SecretKey es una clave dentro de un Secret.
type SecretKey struct {
	Name string
	Key  string
}

var (
	envNameRe  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	dnsLabelRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
)

// reservedEnvPrefix queda para variables que inyecte la propia plataforma.
const reservedEnvPrefix = "JAPPI_"

// Validate comprueba la especificación antes de tocar el clúster.
func (s Spec) Validate() error {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	if s.DeploymentID == "" || s.ProjectID == "" || s.ServiceID == "" {
		add("faltan identificadores (deployment, proyecto o servicio)")
	}
	if !dnsLabelRe.MatchString(s.ServiceName) {
		add("nombre de servicio %q no es una etiqueta DNS válida", s.ServiceName)
	}
	// Solo imágenes fijadas por digest: una etiqueta como :latest puede
	// cambiar por debajo y rompería el rollback (volver a "la misma" imagen).
	if !strings.Contains(s.Image, "@sha256:") {
		add("la imagen %q debe estar fijada por digest (@sha256:...)", s.Image)
	}
	if s.Port < 1 || s.Port > 65535 {
		add("puerto %d fuera de rango", s.Port)
	}
	if s.Domain == "" {
		add("falta el dominio")
	}
	if _, err := LimitsFor(s.Tier); err != nil {
		add("%v", err)
	}
	seen := map[string]bool{}
	for _, e := range s.Env {
		switch {
		case !envNameRe.MatchString(e.Name):
			add("variable %q con nombre inválido", e.Name)
		case strings.HasPrefix(e.Name, reservedEnvPrefix):
			add("la variable %q usa el prefijo reservado %s", e.Name, reservedEnvPrefix)
		case seen[e.Name]:
			add("variable %q repetida", e.Name)
		case e.Secret != nil && (e.Secret.Name == "" || e.Secret.Key == "" || e.Value != ""):
			add("variable %q: una referencia a secreto necesita nombre y clave, y ningún valor literal", e.Name)
		}
		seen[e.Name] = true
	}

	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidSpec, strings.Join(problems, "; "))
	}
	return nil
}
