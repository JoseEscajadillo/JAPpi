package stack_test

import (
	"slices"
	"testing"
	"testing/fstest"

	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/stack"
)

func file(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

func analyze(t *testing.T, fsys fstest.MapFS) stack.Plan {
	t.Helper()
	plan, err := stack.DefaultAnalyzer().Analyze(fsys)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func service(t *testing.T, plan stack.Plan, name string) stack.Service {
	t.Helper()
	for _, s := range plan.Services {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no se detectó el servicio %q; plan: %+v", name, plan.Services)
	return stack.Service{}
}

func TestSingleNextApp(t *testing.T) {
	plan := analyze(t, fstest.MapFS{
		"package.json":   file(`{"dependencies":{"next":"15.0.0","react":"19.0.0"}}`),
		"app/page.tsx":   file(`fetch(process.env.NEXT_PUBLIC_BACKEND_URL + "/items")`),
		"pnpm-lock.yaml": file(``),
	})
	if plan.Monorepo || len(plan.Services) != 1 {
		t.Fatalf("esperaba un solo servicio, obtuve %+v", plan)
	}
	web := service(t, plan, "web")
	if web.Role != stack.RoleFrontend || web.Framework != "next" || web.PublicEnvPrefix != "NEXT_PUBLIC_" {
		t.Errorf("clasificación incorrecta: %+v", web)
	}
	if !slices.Contains(web.EnvHints, "NEXT_PUBLIC_BACKEND_URL") {
		t.Errorf("no leyó la variable del código: %v", web.EnvHints)
	}
}

func TestPnpmTurboMonorepo(t *testing.T) {
	plan := analyze(t, fstest.MapFS{
		"package.json":        file(`{"name":"acme","packageManager":"pnpm@9.0.0"}`),
		"pnpm-workspace.yaml": file("packages:\n  - \"apps/*\"\n  - 'packages/*'\n"),
		"turbo.json":          file(`{}`),
		// Frontend Vite que lee la URL con un nombre propio.
		"apps/web/package.json": file(`{"dependencies":{"react":"19"},"devDependencies":{"vite":"6"}}`),
		"apps/web/src/api.ts":   file(`const base = import.meta.env.VITE_SERVER_BASE_URL`),
		// Backend Express con Prisma (nombre de variable propio) y BullMQ.
		"apps/api/package.json":         file(`{"scripts":{"start":"node dist"},"dependencies":{"express":"5","@prisma/client":"6","bullmq":"5"},"devDependencies":{"vitest":"3"}}`),
		"apps/api/prisma/schema.prisma": file("datasource db {\n  provider = \"postgresql\"\n  url      = env(\"PRISMA_DB\")\n}\n"),
		"apps/api/.env.example":         file("PRISMA_DB=\nREDIS_URL=\nCORS_ORIGIN=http://localhost:5173\n"),
		// Librería compartida: no se despliega.
		"packages/ui/package.json": file(`{"name":"@acme/ui","dependencies":{"react":"19"}}`),
	})

	if !plan.Monorepo || len(plan.Services) != 2 {
		t.Fatalf("esperaba 2 servicios en monorepo, obtuve %+v", plan.Services)
	}
	if !slices.Contains(plan.Tools, "pnpm-workspaces") || !slices.Contains(plan.Tools, "turbo") {
		t.Errorf("herramientas no detectadas: %v", plan.Tools)
	}

	web := service(t, plan, "web")
	if web.Role != stack.RoleFrontend || web.Framework != "vite" || web.PublicEnvPrefix != "VITE_" {
		t.Errorf("web mal clasificado: %+v", web)
	}

	api := service(t, plan, "api")
	if api.Role != stack.RoleBackend || api.Framework != "express" {
		t.Errorf("api mal clasificado: %+v", api)
	}
	if api.DatabaseURLEnv != "PRISMA_DB" {
		t.Errorf("no leyó la variable de Prisma: %q", api.DatabaseURLEnv)
	}
	if want := []stack.Addon{stack.AddonPostgres, stack.AddonRedis}; !slices.Equal(plan.Addons(), want) {
		t.Errorf("add-ons = %v, quería %v", plan.Addons(), want)
	}
}

func TestConventionalFoldersGoAndNext(t *testing.T) {
	plan := analyze(t, fstest.MapFS{
		"frontend/package.json":      file(`{"dependencies":{"next":"15"}}`),
		"backend/go.mod":             file("module acme\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.10.0\n\tgithub.com/jackc/pgx/v5 v5.7.0\n)\n"),
		"backend/cmd/server/main.go": file("package main\n\nfunc main() {}\n"),
		"backend/internal/db.go":     file(`package db; import "os"; var _ = os.Getenv("PG_DSN_URL")`),
	})
	if !plan.Monorepo {
		t.Fatal("carpetas convencionales deberían contar como monorepo")
	}
	be := service(t, plan, "backend")
	if be.Runtime != "go" || be.Framework != "gin" || be.Port != 8080 || !slices.Equal(be.Needs, []stack.Addon{stack.AddonPostgres}) {
		t.Errorf("backend mal clasificado: %+v", be)
	}
	service(t, plan, "frontend")
}

func TestGoLibraryIsNotDeployed(t *testing.T) {
	plan := analyze(t, fstest.MapFS{
		"go.mod": file("module lib\n"),
		"lib.go": file("package lib\n"),
	})
	if len(plan.Services) != 0 {
		t.Fatalf("una librería no es desplegable: %+v", plan.Services)
	}
}

func TestDockerfileFallbackReadsExpose(t *testing.T) {
	plan := analyze(t, fstest.MapFS{
		"Dockerfile": file("FROM rust:1\nEXPOSE 7000\n"),
	})
	if len(plan.Services) != 1 || plan.Services[0].Port != 7000 || plan.Services[0].Runtime != "docker" {
		t.Fatalf("fallback Dockerfile incorrecto: %+v", plan.Services)
	}
}

func TestAffectedServices(t *testing.T) {
	plan := stack.Plan{Monorepo: true, Services: []stack.Service{
		{Name: "web", Path: "apps/web"},
		{Name: "api", Path: "apps/api"},
	}}
	names := func(svcs []stack.Service) []string {
		var out []string
		for _, s := range svcs {
			out = append(out, s.Name)
		}
		return out
	}

	tests := []struct {
		name     string
		changed  []string
		complete bool
		want     []string
	}{
		{"solo el frontend", []string{"apps/web/src/App.tsx"}, true, []string{"web"}},
		{"ambos", []string{"apps/web/a.ts", "apps/api/b.ts"}, true, []string{"web", "api"}},
		{"archivo compartido redespliega todo", []string{"pnpm-lock.yaml"}, true, []string{"web", "api"}},
		{"lista truncada redespliega todo", []string{"apps/web/a.ts"}, false, []string{"web", "api"}},
		{"prefijo parecido no cuenta", []string{"apps/web-old/a.ts"}, true, []string{"web", "api"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := names(stack.AffectedServices(plan, tt.changed, tt.complete))
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
