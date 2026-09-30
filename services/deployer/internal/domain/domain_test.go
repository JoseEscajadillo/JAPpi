package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/domain"
)

func validSpec() domain.Spec {
	return domain.Spec{
		DeploymentID: "dep_1", ProjectID: "p_123", ServiceID: "svc_api", ServiceName: "api",
		Image:  "registry.jappi.app/p_123/api@sha256:" + strings.Repeat("a", 64),
		Port:   3000,
		Domain: "api-acme-abc123.jappi.app",
		Tier:   domain.TierHobby,
		Env: []domain.EnvVar{
			{Name: "PORT", Value: "3000"},
			{Name: "DATABASE_URL", Secret: &domain.SecretKey{Name: "jappi-addon-postgres", Key: "uri"}},
		},
	}
}

func TestValidSpecPasses(t *testing.T) {
	if err := validSpec().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidSpecs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.Spec)
		want   string
	}{
		{"imagen sin digest", func(s *domain.Spec) { s.Image = "nginx:latest" }, "digest"},
		{"puerto cero", func(s *domain.Spec) { s.Port = 0 }, "puerto"},
		{"nombre no DNS", func(s *domain.Spec) { s.ServiceName = "Mi_API" }, "etiqueta DNS"},
		{"plan desconocido", func(s *domain.Spec) { s.Tier = "gratis" }, "plan desconocido"},
		{"variable reservada", func(s *domain.Spec) { s.Env = append(s.Env, domain.EnvVar{Name: "JAPPI_TOKEN", Value: "x"}) }, "reservado"},
		{"variable repetida", func(s *domain.Spec) { s.Env = append(s.Env, domain.EnvVar{Name: "PORT", Value: "1"}) }, "repetida"},
		{"secreto con literal", func(s *domain.Spec) { s.Env[1].Value = "postgres://filtrado" }, "referencia a secreto"},
		{"sin dominio", func(s *domain.Spec) { s.Domain = "" }, "dominio"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSpec()
			tt.mutate(&s)
			err := s.Validate()
			if !errors.Is(err, domain.ErrInvalidSpec) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("esperaba ErrInvalidSpec mencionando %q, obtuve %v", tt.want, err)
			}
		})
	}
}

func TestNamespaceIsValidAndStable(t *testing.T) {
	short := domain.Namespace("P_123")
	if short != "prj-p-123" {
		t.Errorf("namespace inesperado: %s", short)
	}
	long := domain.Namespace(strings.Repeat("x", 100))
	if len(long) > 63 || long != domain.Namespace(strings.Repeat("x", 100)) {
		t.Errorf("namespace largo inválido o inestable: %s (%d)", long, len(long))
	}
	if long == domain.Namespace(strings.Repeat("x", 99)) {
		t.Error("IDs distintos recortados no pueden colisionar")
	}
}

// Invariante: un proyecto típico (frontend, backend, Postgres y Redis) tiene
// que poder hacer un rolling update (un pod extra) sin chocar con su cuota.
func TestEveryTierFitsARollingUpdate(t *testing.T) {
	const workloads = 4
	for _, tier := range []domain.Tier{domain.TierTrial, domain.TierHobby, domain.TierPro, domain.TierTeam} {
		l, err := domain.LimitsFor(tier)
		if err != nil {
			t.Fatal(err)
		}
		pods := workloads*int(l.Replicas) + 1
		if pods*l.CPURequestMilli > l.QuotaCPURequestMilli {
			t.Errorf("%s: CPU pedida %dm > cuota %dm", tier, pods*l.CPURequestMilli, l.QuotaCPURequestMilli)
		}
		if pods*l.MemRequestMiB > l.QuotaMemRequestMiB {
			t.Errorf("%s: memoria pedida %dMi > cuota %dMi", tier, pods*l.MemRequestMiB, l.QuotaMemRequestMiB)
		}
		if pods*l.MemLimitMiB > l.QuotaMemLimitMiB {
			t.Errorf("%s: límite de memoria %dMi > cuota %dMi", tier, pods*l.MemLimitMiB, l.QuotaMemLimitMiB)
		}
		if pods > l.QuotaPods {
			t.Errorf("%s: %d pods > cuota %d", tier, pods, l.QuotaPods)
		}
	}
}

func TestPlanUsesStableLabels(t *testing.T) {
	a, b := validSpec(), validSpec()
	b.DeploymentID = "dep_2"
	_, wa, err := domain.Plan(a)
	if err != nil {
		t.Fatal(err)
	}
	_, wb, _ := domain.Plan(b)
	for k, v := range wa.Labels {
		if wb.Labels[k] != v {
			t.Errorf("la label %s cambia entre despliegues: el selector del Deployment se rompería", k)
		}
	}
}

func TestEvaluate(t *testing.T) {
	done := domain.RolloutStatus{Found: true, Generation: 2, ObservedGeneration: 2, Desired: 1, Updated: 1, Available: 1}
	tests := []struct {
		name   string
		status domain.RolloutStatus
		want   domain.Phase
	}{
		{"completo", done, domain.Complete},
		{"aún no existe", domain.RolloutStatus{}, domain.Progressing},
		{"generación vieja", func() domain.RolloutStatus { s := done; s.ObservedGeneration = 1; return s }(), domain.Progressing},
		{"sin disponibles", func() domain.RolloutStatus { s := done; s.Available = 0; return s }(), domain.Progressing},
		{"crash loop", func() domain.RolloutStatus {
			s := done
			s.Available = 0
			s.PodProblems = []string{"CrashLoopBackOff"}
			return s
		}(), domain.Failed},
		{"PSA rechaza", func() domain.RolloutStatus { s := done; s.ReplicaFailure = "forbidden: violates PodSecurity"; return s }(), domain.Failed},
		{"plazo vencido", func() domain.RolloutStatus { s := done; s.DeadlineExceeded = true; return s }(), domain.Failed},
		{"esperando contenedor no es fallo", func() domain.RolloutStatus {
			s := done
			s.Available = 0
			s.PodProblems = []string{"ContainerCreating"}
			return s
		}(), domain.Progressing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, detail := domain.Evaluate(tt.status); got != tt.want {
				t.Errorf("fase %v, quería %v (%s)", got, tt.want, detail)
			}
		})
	}
}
