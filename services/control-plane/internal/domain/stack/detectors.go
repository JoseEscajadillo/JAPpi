package stack

import (
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type framework struct {
	dep, name string
	role      Role
	port      int
	prefix    string
}

// Frameworks de frontend en orden de prioridad. Los "full-stack" (Next,
// Nuxt, SvelteKit, Astro) cuentan como frontend: tienen su propio servidor
// pero su código de cliente necesita la URL pública del backend.
var nodeFrontends = []framework{
	{"next", "next", RoleFrontend, 3000, "NEXT_PUBLIC_"},
	{"nuxt", "nuxt", RoleFrontend, 3000, "NUXT_PUBLIC_"},
	{"@sveltejs/kit", "sveltekit", RoleFrontend, 3000, "PUBLIC_"},
	{"astro", "astro", RoleFrontend, 4321, "PUBLIC_"},
	{"@angular/core", "angular", RoleFrontend, 4200, ""},
	{"react-scripts", "create-react-app", RoleFrontend, 3000, "REACT_APP_"},
}

var nodeBackends = []framework{
	{"@nestjs/core", "nestjs", RoleBackend, 3000, ""},
	{"fastify", "fastify", RoleBackend, 3000, ""},
	{"express", "express", RoleBackend, 3000, ""},
	{"hono", "hono", RoleBackend, 3000, ""},
	{"koa", "koa", RoleBackend, 3000, ""},
	{"@hapi/hapi", "hapi", RoleBackend, 3000, ""},
	{"elysia", "elysia", RoleBackend, 3000, ""},
}

// Vite va al final: aparece también en backends que solo usan vitest.
var viteFramework = framework{"vite", "vite", RoleFrontend, 5173, "VITE_"}

var (
	nodePostgresDrivers = []string{"pg", "postgres", "pg-promise", "slonik", "@mikro-orm/postgresql"}
	nodeRedisClients    = []string{"redis", "ioredis", "bullmq", "bull", "@keyv/redis", "connect-redis"}
)

// NodeDetector reconoce aplicaciones con package.json.
type NodeDetector struct{}

func (NodeDetector) Detect(fsys fs.FS, dir string, _ RepoInfo) (Service, bool, error) {
	pkg, ok, err := readPackageJSON(fsys, path.Join(dir, "package.json"))
	if err != nil || !ok {
		return Service{}, false, err
	}
	all := merge(pkg.Dependencies, pkg.DevDependencies)

	fw, found := match(nodeFrontends, all)
	if !found {
		fw, found = match(nodeBackends, pkg.Dependencies)
	}
	if !found {
		fw, found = match([]framework{viteFramework}, all)
	}
	if !found {
		if pkg.Scripts["start"] == "" {
			return Service{}, false, nil // librería del monorepo, no se despliega
		}
		fw = framework{name: "node", role: RoleBackend, port: 3000}
	}

	svc := Service{Role: fw.role, Runtime: "node", Framework: fw.name, Port: fw.port, PublicEnvPrefix: fw.prefix}
	if hasAny(all, nodePostgresDrivers) {
		svc.Needs = append(svc.Needs, AddonPostgres)
	}
	if schema, ok := readFile(fsys, path.Join(dir, "prisma", "schema.prisma")); ok {
		if provider, env := parsePrisma(schema); provider == "postgresql" || provider == "postgres" {
			if !hasAny(all, nodePostgresDrivers) {
				svc.Needs = append(svc.Needs, AddonPostgres)
			}
			svc.DatabaseURLEnv = env
		}
	}
	if hasAny(all, nodeRedisClients) {
		svc.Needs = append(svc.Needs, AddonRedis)
	}
	return svc, true, nil
}

var (
	prismaProvider = regexp.MustCompile(`provider\s*=\s*"([a-z]+)"`)
	prismaURLEnv   = regexp.MustCompile(`url\s*=\s*env\(\s*"([A-Za-z_][A-Za-z0-9_]*)"\s*\)`)
)

// parsePrisma devuelve el provider y la variable de conexión del bloque datasource.
func parsePrisma(schema string) (provider, urlEnv string) {
	i := strings.Index(schema, "datasource")
	if i < 0 {
		return "", ""
	}
	block := schema[i:]
	if end := strings.Index(block, "}"); end >= 0 {
		block = block[:end]
	}
	if m := prismaProvider.FindStringSubmatch(block); m != nil {
		provider = m[1]
	}
	if m := prismaURLEnv.FindStringSubmatch(block); m != nil {
		urlEnv = m[1]
	}
	return provider, urlEnv
}

var (
	goPostgresModules = []string{"github.com/jackc/pgx", "github.com/lib/pq", "gorm.io/driver/postgres", "github.com/uptrace/bun/driver/pgdriver"}
	goRedisModules    = []string{"github.com/redis/go-redis", "github.com/go-redis/redis", "github.com/gomodule/redigo", "github.com/redis/rueidis"}
	goFrameworks      = []struct{ module, name string }{
		{"github.com/gin-gonic/gin", "gin"},
		{"github.com/labstack/echo", "echo"},
		{"github.com/gofiber/fiber", "fiber"},
		{"github.com/go-chi/chi", "chi"},
	}
)

// GoDetector reconoce módulos Go con un paquete main.
type GoDetector struct{}

func (GoDetector) Detect(fsys fs.FS, dir string, _ RepoInfo) (Service, bool, error) {
	mod, ok := readFile(fsys, path.Join(dir, "go.mod"))
	if !ok || !hasMainPackage(fsys, dir) {
		return Service{}, false, nil
	}
	svc := Service{Role: RoleBackend, Runtime: "go", Framework: "net/http", Port: 8080}
	for _, f := range goFrameworks {
		if strings.Contains(mod, f.module) {
			svc.Framework = f.name
			break
		}
	}
	if containsAny(mod, goPostgresModules) {
		svc.Needs = append(svc.Needs, AddonPostgres)
	}
	if containsAny(mod, goRedisModules) {
		svc.Needs = append(svc.Needs, AddonRedis)
	}
	return svc, true, nil
}

func hasMainPackage(fsys fs.FS, dir string) bool {
	candidates, _ := fs.Glob(fsys, path.Join(dir, "*.go"))
	more, _ := fs.Glob(fsys, path.Join(dir, "cmd", "*", "*.go"))
	for _, f := range append(candidates, more...) {
		if src, ok := readFile(fsys, f); ok && mainPackageRe.MatchString(src) {
			return true
		}
	}
	return false
}

// PythonDetector reconoce proyectos Django, FastAPI y Flask.
type PythonDetector struct{}

func (PythonDetector) Detect(fsys fs.FS, dir string, _ RepoInfo) (Service, bool, error) {
	var deps string
	for _, f := range []string{"requirements.txt", "pyproject.toml", "Pipfile"} {
		if raw, ok := readFile(fsys, path.Join(dir, f)); ok {
			deps += strings.ToLower(raw) + "\n"
		}
	}
	if deps == "" {
		return Service{}, false, nil
	}
	svc := Service{Role: RoleBackend, Runtime: "python", Port: 8000}
	for _, fw := range []string{"django", "fastapi", "flask", "litestar"} {
		if strings.Contains(deps, fw) {
			svc.Framework = fw
			break
		}
	}
	if svc.Framework == "" {
		return Service{}, false, nil
	}
	if containsAny(deps, []string{"psycopg", "asyncpg", "pg8000"}) {
		svc.Needs = append(svc.Needs, AddonPostgres)
	}
	if strings.Contains(deps, "redis") {
		svc.Needs = append(svc.Needs, AddonRedis)
	}
	return svc, true, nil
}

var (
	exposeRe      = regexp.MustCompile(`(?mi)^\s*EXPOSE\s+(\d+)`)
	mainPackageRe = regexp.MustCompile(`(?m)^package main\b`)
)

// DockerfileDetector es el último recurso: cualquier carpeta con Dockerfile.
type DockerfileDetector struct{}

func (DockerfileDetector) Detect(fsys fs.FS, dir string, _ RepoInfo) (Service, bool, error) {
	df, ok := readFile(fsys, path.Join(dir, "Dockerfile"))
	if !ok {
		return Service{}, false, nil
	}
	svc := Service{Role: RoleBackend, Runtime: "docker", Framework: "dockerfile", Port: 8080}
	if m := exposeRe.FindStringSubmatch(df); m != nil {
		if p, err := strconv.Atoi(m[1]); err == nil {
			svc.Port = p
		}
	}
	return svc, true, nil
}

func match(fws []framework, deps map[string]string) (framework, bool) {
	for _, fw := range fws {
		if _, ok := deps[fw.dep]; ok {
			return fw, true
		}
	}
	return framework{}, false
}

func merge(a, b map[string]string) map[string]string {
	out := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func hasAny(deps map[string]string, names []string) bool {
	for _, n := range names {
		if _, ok := deps[n]; ok {
			return true
		}
	}
	return false
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
