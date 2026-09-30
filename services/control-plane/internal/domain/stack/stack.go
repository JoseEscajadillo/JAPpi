// Package stack analiza el repositorio de un cliente y deduce qué servicios
// contiene (frontend, backend), con qué framework, y qué add-ons necesitan.
// Es el corazón del diferencial de JAPpi (ADR-0008).
//
// El dominio no sabe de dónde vienen los archivos: recibe un fs.FS, que es
// el puerto. Un adaptador lo llena con un clon de git, un tarball de la API
// de GitHub o una carpeta local.
package stack

import (
	"io/fs"
	"path"
	"slices"
	"strings"
)

// Role es la función de un servicio dentro del proyecto.
type Role string

const (
	RoleFrontend Role = "frontend"
	RoleBackend  Role = "backend"
)

// Addon es un servicio gestionado que JAPpi aprovisiona.
type Addon string

const (
	AddonPostgres Addon = "postgres"
	AddonRedis    Addon = "redis"
)

// Service es un servicio desplegable detectado en el repo.
type Service struct {
	Name      string `json:"name"`
	Path      string `json:"path"` // "." o "apps/web"
	Role      Role   `json:"role"`
	Runtime   string `json:"runtime"`   // node, go, python, docker
	Framework string `json:"framework"` // next, vite, express, gin...
	Port      int    `json:"port"`
	// PublicEnvPrefix es el prefijo de las variables que el framework
	// incrusta en el bundle al construir (NEXT_PUBLIC_, VITE_...).
	PublicEnvPrefix string  `json:"public_env_prefix,omitempty"`
	Needs           []Addon `json:"needs,omitempty"`
	// EnvHints son las variables que el código del servicio espera, sacadas
	// de .env.example y del código fuente. El cableado las usa para elegir
	// nombres en vez de imponer los nuestros.
	EnvHints []string `json:"env_hints,omitempty"`
	// DatabaseURLEnv es el nombre exacto de la variable de conexión cuando el
	// ORM lo declara (p. ej. `url = env("X")` en schema.prisma).
	DatabaseURLEnv string `json:"database_url_env,omitempty"`
	HasDockerfile  bool   `json:"has_dockerfile"`
}

// Plan es el resultado del análisis de un repositorio.
type Plan struct {
	Monorepo bool      `json:"monorepo"`
	Tools    []string  `json:"tools,omitempty"` // pnpm-workspaces, turbo, nx...
	Services []Service `json:"services"`
}

// Addons devuelve la unión ordenada de add-ons que necesita el plan.
func (p Plan) Addons() []Addon {
	var out []Addon
	for _, s := range p.Services {
		for _, a := range s.Needs {
			if !slices.Contains(out, a) {
				out = append(out, a)
			}
		}
	}
	slices.Sort(out)
	return out
}

// ByRole devuelve los servicios con ese rol.
func (p Plan) ByRole(r Role) []Service {
	var out []Service
	for _, s := range p.Services {
		if s.Role == r {
			out = append(out, s)
		}
	}
	return out
}

// Detector reconoce un tipo de servicio en un directorio. Añadir soporte
// para un lenguaje nuevo es añadir un Detector, sin tocar el Analyzer
// (principio abierto/cerrado).
type Detector interface {
	// Detect devuelve ok=false si el directorio no es un servicio que este
	// detector reconozca (o si es una librería, no algo desplegable).
	Detect(fsys fs.FS, dir string, repo RepoInfo) (svc Service, ok bool, err error)
}

// RepoInfo es información de la raíz del repo que los detectores comparten.
type RepoInfo struct {
	PackageManager string // npm, pnpm, yarn, bun
}

// Analyzer recorre el repo y aplica los detectores en orden.
type Analyzer struct {
	detectors []Detector
}

// NewAnalyzer crea un Analyzer con los detectores dados, en orden de prioridad.
func NewAnalyzer(detectors ...Detector) *Analyzer { return &Analyzer{detectors: detectors} }

// DefaultAnalyzer usa todos los detectores incluidos en JAPpi.
func DefaultAnalyzer() *Analyzer {
	return NewAnalyzer(NodeDetector{}, GoDetector{}, PythonDetector{}, DockerfileDetector{})
}

// conventionalDirs son las carpetas que se revisan cuando el repo no declara
// workspaces pero reparte el código en subcarpetas.
var conventionalDirs = []string{
	"apps/*", "services/*",
	"frontend", "backend", "web", "api", "client", "server", "www",
}

// Analyze devuelve el plan de despliegue del repo.
func (a *Analyzer) Analyze(fsys fs.FS) (Plan, error) {
	repo := RepoInfo{PackageManager: packageManager(fsys)}
	var plan Plan

	patterns, tools, err := workspacePatterns(fsys)
	if err != nil {
		return Plan{}, err
	}
	plan.Tools = tools
	if len(patterns) == 0 {
		patterns = conventionalDirs
	}

	dirs, err := expand(fsys, patterns)
	if err != nil {
		return Plan{}, err
	}
	for _, dir := range dirs {
		svc, ok, err := a.detect(fsys, dir, repo)
		if err != nil {
			return Plan{}, err
		}
		if ok {
			plan.Services = append(plan.Services, svc)
		}
	}

	if len(plan.Services) > 0 {
		plan.Monorepo = true
	} else {
		// Repo de un solo servicio: la raíz es la aplicación.
		svc, ok, err := a.detect(fsys, ".", repo)
		if err != nil {
			return Plan{}, err
		}
		if ok {
			plan.Services = []Service{svc}
		}
	}
	assignNames(plan.Services)
	return plan, nil
}

func (a *Analyzer) detect(fsys fs.FS, dir string, repo RepoInfo) (Service, bool, error) {
	for _, d := range a.detectors {
		svc, ok, err := d.Detect(fsys, dir, repo)
		if err != nil || !ok {
			if err != nil {
				return Service{}, false, err
			}
			continue
		}
		svc.Path = dir
		svc.HasDockerfile = exists(fsys, path.Join(dir, "Dockerfile"))
		hints, err := collectEnvHints(fsys, dir)
		if err != nil {
			return Service{}, false, err
		}
		svc.EnvHints = hints
		return svc, true, nil
	}
	return Service{}, false, nil
}

// expand resuelve patrones tipo "apps/*" a directorios existentes, sin repetir.
func expand(fsys fs.FS, patterns []string) ([]string, error) {
	var dirs []string
	for _, p := range patterns {
		if strings.HasPrefix(p, "!") {
			continue
		}
		p = strings.TrimSuffix(strings.ReplaceAll(strings.TrimPrefix(p, "./"), "**", "*"), "/")
		matches, err := fs.Glob(fsys, p)
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			if st, err := fs.Stat(fsys, m); err == nil && st.IsDir() && !slices.Contains(dirs, m) {
				dirs = append(dirs, m)
			}
		}
	}
	slices.Sort(dirs)
	return dirs, nil
}

// assignNames da a cada servicio un nombre corto, único y válido como etiqueta DNS.
func assignNames(svcs []Service) {
	used := map[string]int{}
	for i := range svcs {
		name := path.Base(svcs[i].Path)
		if svcs[i].Path == "." {
			name = "web"
			if svcs[i].Role == RoleBackend {
				name = "api"
			}
		}
		name = dnsLabel(name)
		used[name]++
		if n := used[name]; n > 1 {
			name = name + "-" + string(rune('0'+n))
		}
		svcs[i].Name = name
	}
}

func dnsLabel(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "svc"
	}
	return out
}

func exists(fsys fs.FS, name string) bool {
	_, err := fs.Stat(fsys, name)
	return err == nil
}

func readFile(fsys fs.FS, name string) (string, bool) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", false
	}
	return string(b), true
}
