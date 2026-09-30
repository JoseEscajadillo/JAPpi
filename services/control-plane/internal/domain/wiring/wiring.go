// Package wiring decide qué variables de entorno inyectar en cada servicio
// para que frontend, backend y add-ons se conecten sin configuración
// (ADR-0008).
//
// Regla de oro: preferimos el nombre que el código del cliente ya usa
// (EnvHints) y solo inventamos uno cuando no hay pistas. Las variables que el
// usuario definió a mano nunca se sobrescriben.
package wiring

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/domains"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/stack"
)

// Var es una variable que JAPpi inyecta.
type Var struct {
	Name string `json:"name"`
	// Value es un valor literal (URLs públicas, puertos). Vacío si Ref != nil.
	Value string `json:"value,omitempty"`
	// Ref apunta a un secreto de un add-on; el deployer lo resuelve a un
	// Secret de Kubernetes. Los secretos nunca pasan por aquí en claro.
	Ref *Ref `json:"ref,omitempty"`
	// BuildTime indica que la variable debe existir al construir la imagen
	// (el framework la incrusta en el bundle del navegador).
	BuildTime bool `json:"build_time"`
	// Reason explica en el dashboard por qué existe la variable.
	Reason string `json:"reason"`
}

// Ref referencia una credencial de un add-on.
type Ref struct {
	Addon stack.Addon `json:"addon"`
	Key   string      `json:"key"` // "uri"
}

// Conflict es una variable que habríamos inyectado, pero el usuario ya la definió.
type Conflict struct {
	Service string `json:"service"`
	Name    string `json:"name"`
}

// Input es todo lo que el cableado necesita.
type Input struct {
	Plan stack.Plan
	// Domains asigna a cada servicio (por nombre) su dominio público reservado.
	Domains map[string]string
	// UserVars son las variables definidas por el usuario, por servicio.
	UserVars map[string]map[string]string
}

// Result son las variables por servicio (en orden estable) y los conflictos.
type Result struct {
	Services  map[string][]Var `json:"services"`
	Conflicts []Conflict       `json:"conflicts,omitempty"`
}

// Resolve calcula el cableado del plan.
func Resolve(in Input) Result {
	res := Result{Services: map[string][]Var{}}
	frontends := in.Plan.ByRole(stack.RoleFrontend)
	backends := in.Plan.ByRole(stack.RoleBackend)

	add := func(svc string, v Var) {
		if _, taken := in.UserVars[svc][v.Name]; taken {
			res.Conflicts = append(res.Conflicts, Conflict{Service: svc, Name: v.Name})
			return
		}
		res.Services[svc] = append(res.Services[svc], v)
	}

	for _, b := range backends {
		add(b.Name, Var{Name: "PORT", Value: strconv.Itoa(b.Port), Reason: "Puerto en el que JAPpi espera tráfico"})
		if slices.Contains(b.Needs, stack.AddonPostgres) {
			name := b.DatabaseURLEnv
			if name == "" {
				name = pick(b.EnvHints, databaseNames, databasePattern, "DATABASE_URL")
			}
			add(b.Name, Var{Name: name, Ref: &Ref{Addon: stack.AddonPostgres, Key: "uri"}, Reason: "Conexión al PostgreSQL del proyecto"})
		}
		if slices.Contains(b.Needs, stack.AddonRedis) {
			name := pick(b.EnvHints, redisNames, redisPattern, "REDIS_URL")
			add(b.Name, Var{Name: name, Ref: &Ref{Addon: stack.AddonRedis, Key: "uri"}, Reason: "Conexión al Redis del proyecto"})
		}
		if len(frontends) > 0 {
			origins := make([]string, 0, len(frontends))
			for _, f := range frontends {
				origins = append(origins, domains.URL(in.Domains[f.Name]))
			}
			name := pick(b.EnvHints, originNames, originPattern, "FRONTEND_URL")
			add(b.Name, Var{Name: name, Value: strings.Join(origins, ","), Reason: "Origen permitido para CORS: la URL pública del frontend"})
		}
	}

	for _, f := range frontends {
		add(f.Name, Var{Name: "PORT", Value: strconv.Itoa(f.Port), Reason: "Puerto en el que JAPpi espera tráfico"})
		for _, b := range backends {
			add(f.Name, Var{
				Name:      apiVarName(f, b, len(backends)),
				Value:     domains.URL(in.Domains[b.Name]),
				BuildTime: f.PublicEnvPrefix != "",
				Reason:    "URL pública del backend " + b.Name,
			})
		}
	}
	return res
}

var (
	databaseNames   = []string{"DATABASE_URL", "POSTGRES_URL", "POSTGRESQL_URL", "PG_URL", "DB_URL", "DATABASE_URI", "SQLALCHEMY_DATABASE_URI"}
	databasePattern = regexp.MustCompile(`(^|_)(DATABASE|POSTGRES|POSTGRESQL|PG|DB)(_[A-Z0-9]+)*_(URL|URI|DSN)$`)
	redisNames      = []string{"REDIS_URL", "REDIS_URI", "REDIS_DSN"}
	redisPattern    = regexp.MustCompile(`REDIS(_[A-Z0-9]+)*_(URL|URI|DSN)$`)
	originNames     = []string{"FRONTEND_URL", "CLIENT_URL", "WEB_URL", "APP_URL", "CORS_ORIGIN", "CORS_ORIGINS", "ALLOWED_ORIGINS", "ORIGIN"}
	originPattern   = regexp.MustCompile(`^(FRONTEND|CLIENT|WEB|CORS|ALLOWED)(_[A-Z0-9]+)*_(URL|ORIGIN|ORIGINS)$`)
	apiPattern      = regexp.MustCompile(`(API|BACKEND|SERVER)(_[A-Z0-9]+)*_(URL|URI|HOST|ENDPOINT|BASE|BASE_URL)$`)
)

// pick elige el nombre de variable: primero un nombre conocido que el
// código ya use, luego cualquier pista que encaje con el patrón, y si no, fallback.
func pick(hints, known []string, pattern *regexp.Regexp, fallback string) string {
	for _, k := range known {
		if slices.Contains(hints, k) {
			return k
		}
	}
	for _, h := range hints {
		if pattern.MatchString(h) {
			return h
		}
	}
	return fallback
}

// apiVarName elige cómo llama el frontend f a la URL del backend b.
func apiVarName(f, b stack.Service, totalBackends int) string {
	backendTag := strings.ToUpper(strings.ReplaceAll(b.Name, "-", "_"))
	var candidates []string
	for _, h := range f.EnvHints {
		if strings.HasPrefix(h, f.PublicEnvPrefix) && apiPattern.MatchString(h) {
			candidates = append(candidates, h)
		}
	}
	if totalBackends == 1 && len(candidates) > 0 {
		return candidates[0]
	}
	for _, c := range candidates {
		if strings.Contains(c, backendTag) {
			return c
		}
	}
	if totalBackends == 1 {
		return f.PublicEnvPrefix + "API_URL"
	}
	return f.PublicEnvPrefix + backendTag + "_URL"
}
