package wiring_test

import (
	"testing"

	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/stack"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/wiring"
)

var doms = map[string]string{"web": "web-acme-aaaaaa.jappi.app", "api": "api-acme-bbbbbb.jappi.app"}

func find(t *testing.T, vars []wiring.Var, name string) wiring.Var {
	t.Helper()
	for _, v := range vars {
		if v.Name == name {
			return v
		}
	}
	t.Fatalf("falta la variable %s en %+v", name, vars)
	return wiring.Var{}
}

func TestDefaultsWithoutHints(t *testing.T) {
	plan := stack.Plan{Services: []stack.Service{
		{Name: "web", Role: stack.RoleFrontend, Port: 3000, PublicEnvPrefix: "NEXT_PUBLIC_"},
		{Name: "api", Role: stack.RoleBackend, Port: 8080, Needs: []stack.Addon{stack.AddonPostgres, stack.AddonRedis}},
	}}
	res := wiring.Resolve(wiring.Input{Plan: plan, Domains: doms})

	apiURL := find(t, res.Services["web"], "NEXT_PUBLIC_API_URL")
	if apiURL.Value != "https://api-acme-bbbbbb.jappi.app" || !apiURL.BuildTime {
		t.Errorf("URL del backend incorrecta: %+v", apiURL)
	}
	if db := find(t, res.Services["api"], "DATABASE_URL"); db.Ref == nil || db.Ref.Addon != stack.AddonPostgres || db.Value != "" {
		t.Errorf("DATABASE_URL debe ser una referencia a secreto, no un literal: %+v", db)
	}
	find(t, res.Services["api"], "REDIS_URL")
	if cors := find(t, res.Services["api"], "FRONTEND_URL"); cors.Value != "https://web-acme-aaaaaa.jappi.app" {
		t.Errorf("origen CORS incorrecto: %+v", cors)
	}
}

func TestRespectsNamesTheCodeAlreadyUses(t *testing.T) {
	plan := stack.Plan{Services: []stack.Service{
		{Name: "web", Role: stack.RoleFrontend, PublicEnvPrefix: "VITE_", EnvHints: []string{"VITE_SERVER_BASE_URL", "VITE_SENTRY_DSN"}},
		{Name: "api", Role: stack.RoleBackend, Needs: []stack.Addon{stack.AddonPostgres}, DatabaseURLEnv: "PRISMA_DB", EnvHints: []string{"CORS_ORIGIN"}},
	}}
	res := wiring.Resolve(wiring.Input{Plan: plan, Domains: doms})

	find(t, res.Services["web"], "VITE_SERVER_BASE_URL")
	find(t, res.Services["api"], "PRISMA_DB")
	find(t, res.Services["api"], "CORS_ORIGIN")
}

func TestMultipleBackendsGetOneVarEach(t *testing.T) {
	plan := stack.Plan{Services: []stack.Service{
		{Name: "web", Role: stack.RoleFrontend, PublicEnvPrefix: "NEXT_PUBLIC_", EnvHints: []string{"NEXT_PUBLIC_AUTH_API_URL"}},
		{Name: "auth-api", Role: stack.RoleBackend},
		{Name: "shop", Role: stack.RoleBackend},
	}}
	res := wiring.Resolve(wiring.Input{Plan: plan, Domains: map[string]string{"auth-api": "a.jappi.app", "shop": "s.jappi.app"}})

	if v := find(t, res.Services["web"], "NEXT_PUBLIC_AUTH_API_URL"); v.Value != "https://a.jappi.app" {
		t.Errorf("auth-api mal cableado: %+v", v)
	}
	if v := find(t, res.Services["web"], "NEXT_PUBLIC_SHOP_URL"); v.Value != "https://s.jappi.app" {
		t.Errorf("shop mal cableado: %+v", v)
	}
}

func TestNeverOverwritesUserVars(t *testing.T) {
	plan := stack.Plan{Services: []stack.Service{
		{Name: "api", Role: stack.RoleBackend, Needs: []stack.Addon{stack.AddonPostgres}},
	}}
	res := wiring.Resolve(wiring.Input{
		Plan:     plan,
		Domains:  doms,
		UserVars: map[string]map[string]string{"api": {"DATABASE_URL": "postgres://externo"}},
	})
	for _, v := range res.Services["api"] {
		if v.Name == "DATABASE_URL" {
			t.Fatal("sobrescribió una variable del usuario")
		}
	}
	if len(res.Conflicts) != 1 || res.Conflicts[0].Name != "DATABASE_URL" {
		t.Errorf("debería reportar el conflicto: %+v", res.Conflicts)
	}
}
