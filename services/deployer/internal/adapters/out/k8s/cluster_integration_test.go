package k8s_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/adapters/out/k8s"
	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/domain"
)

// podinfo: app de ejemplo pública, fijada por digest como exige el dominio.
const podinfo = "ghcr.io/stefanprodan/podinfo:6.9.2@sha256:cab90e04829cdae73199a8402882a4e3ccaf251b8a0d50bdefede0ab3f6371e7"

// Prueba contra un clúster real (la CI levanta uno con kind):
//
//	JAPPI_TEST_KUBECONFIG=$HOME/.kube/config go test ./services/deployer/...
func TestIntegrationDeployToRealCluster(t *testing.T) {
	kubeconfig := os.Getenv("JAPPI_TEST_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("JAPPI_TEST_KUBECONFIG no definida")
	}
	client, err := k8s.NewClient(kubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	cluster := k8s.New(client, k8s.Config{}) // sin gVisor ni ingress class en kind

	projectID := "it-" + time.Now().Format("150405")
	project, workload, err := domain.Plan(domain.Spec{
		DeploymentID: "dep_it", ProjectID: projectID, ServiceID: "svc_web", ServiceName: "web",
		Image: podinfo, Port: 9898, Domain: "web-it.jappi.localhost", Tier: domain.TierHobby,
		Env: []domain.EnvVar{{Name: "PODINFO_UI_MESSAGE", Value: "desplegado por JAPpi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	t.Cleanup(func() {
		_ = client.CoreV1().Namespaces().Delete(context.Background(), project.Namespace, metav1.DeleteOptions{})
	})

	if err := cluster.EnsureProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	if err := cluster.ApplyWorkload(ctx, workload); err != nil {
		t.Fatal(err)
	}

	for {
		status, err := cluster.Rollout(ctx, workload.Namespace, workload.Name)
		if err != nil {
			t.Fatal(err)
		}
		phase, detail := domain.Evaluate(status)
		if phase == domain.Complete {
			t.Logf("rollout completo: %s", detail)
			break
		}
		if phase == domain.Failed {
			t.Fatalf("el rollout falló: %s", detail)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("el rollout no terminó a tiempo: %s", detail)
		case <-time.After(2 * time.Second):
		}
	}

	t.Run("Pod Security rechaza un pod privilegiado", func(t *testing.T) {
		privileged := true
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "intruso", Namespace: project.Namespace},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name: "x", Image: podinfo,
				SecurityContext: &corev1.SecurityContext{Privileged: &privileged},
			}}},
		}
		_, err := client.CoreV1().Pods(project.Namespace).Create(ctx, pod, metav1.CreateOptions{})
		if !apierrors.IsForbidden(err) || !strings.Contains(err.Error(), "PodSecurity") {
			t.Fatalf("el namespace debía rechazar el pod privilegiado, obtuve: %v", err)
		}
	})
}
