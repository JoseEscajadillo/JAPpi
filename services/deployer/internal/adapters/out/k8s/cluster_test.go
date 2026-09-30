package k8s_test

import (
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/adapters/out/k8s"
	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/domain"
)

var ctx = context.Background()

func plan(t *testing.T, deploymentID string) (domain.Project, domain.Workload) {
	t.Helper()
	p, w, err := domain.Plan(domain.Spec{
		DeploymentID: deploymentID, ProjectID: "p1", ServiceID: "svc_api", ServiceName: "api",
		Image: "registry.jappi.app/p1/api@sha256:" + strings.Repeat("c", 64), Port: 3000,
		Domain: "api-acme-abc123.jappi.app", Tier: domain.TierHobby,
		Env: []domain.EnvVar{
			{Name: "PORT", Value: "3000"},
			{Name: "DATABASE_URL", Secret: &domain.SecretKey{Name: "jappi-addon-postgres", Key: "uri"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return p, w
}

func setup(t *testing.T) (*k8s.Cluster, *fake.Clientset) {
	t.Helper()
	client := fake.NewClientset()
	return k8s.New(client, k8s.Config{RuntimeClass: "gvisor", IngressClass: "traefik"}), client
}

func TestEnsureProjectIsolatesTheNamespace(t *testing.T) {
	c, client := setup(t)
	p, _ := plan(t, "dep_1")
	for range 2 { // idempotente
		if err := c.EnsureProject(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	ns, err := client.CoreV1().Namespaces().Get(ctx, "prj-p1", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if ns.Labels["pod-security.kubernetes.io/enforce"] != "restricted" {
		t.Errorf("el namespace debe exigir Pod Security restricted: %v", ns.Labels)
	}

	q, err := client.CoreV1().ResourceQuotas("prj-p1").Get(ctx, "jappi-quota", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Spec.Hard[corev1.ResourceRequestsCPU]; got.MilliValue() != 500 {
		t.Errorf("cuota de CPU del plan hobby = %s", got.String())
	}

	nps, err := client.NetworkingV1().NetworkPolicies("prj-p1").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, np := range nps.Items {
		names[np.Name] = true
		if np.Name == "allow-internet" {
			peer := np.Spec.Egress[0].To[0].IPBlock
			if peer == nil || !contains(peer.Except, "169.254.0.0/16") {
				t.Errorf("la salida a internet debe excluir el servicio de metadatos: %+v", peer)
			}
			for _, port := range np.Spec.Egress[0].Ports {
				if p := port.Port.IntValue(); p == 25 || p == 587 {
					t.Errorf("SMTP no debe estar permitido")
				}
			}
		}
	}
	for _, want := range []string{"default-deny", "allow-same-project", "allow-ingress-controller", "allow-dns", "allow-internet"} {
		if !names[want] {
			t.Errorf("falta la NetworkPolicy %s", want)
		}
	}
}

func TestApplyWorkloadHardensThePod(t *testing.T) {
	c, client := setup(t)
	_, w := plan(t, "dep_1")
	if err := c.ApplyWorkload(ctx, w); err != nil {
		t.Fatal(err)
	}
	d, err := client.AppsV1().Deployments("prj-p1").Get(ctx, "api", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pod := d.Spec.Template.Spec
	ctr := pod.Containers[0]
	sc := ctr.SecurityContext

	switch {
	case pod.RuntimeClassName == nil || *pod.RuntimeClassName != "gvisor":
		t.Error("el pod debe correr con gVisor")
	case pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken:
		t.Error("el pod no debe montar el token de la API")
	case sc == nil || sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation:
		t.Error("allowPrivilegeEscalation debe ser false")
	case sc.RunAsUser == nil || *sc.RunAsUser == 0:
		t.Error("el contenedor no puede correr como root")
	case len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL":
		t.Error("hay que quitar todas las capabilities")
	case d.Spec.Strategy.RollingUpdate.MaxUnavailable.IntValue() != 0:
		t.Error("el rolling update no debe dejar el servicio sin pods")
	case ctr.Resources.Limits.Memory().Value() != 384<<20:
		t.Errorf("límite de memoria = %s", ctr.Resources.Limits.Memory())
	}

	var db *corev1.EnvVar
	for i := range ctr.Env {
		if ctr.Env[i].Name == "DATABASE_URL" {
			db = &ctr.Env[i]
		}
	}
	if db == nil || db.Value != "" || db.ValueFrom == nil || db.ValueFrom.SecretKeyRef.Name != "jappi-addon-postgres" {
		t.Errorf("DATABASE_URL debe salir de un Secret, no de un literal: %+v", db)
	}

	ing, err := client.NetworkingV1().Ingresses("prj-p1").Get(ctx, "api", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if ing.Spec.Rules[0].Host != "api-acme-abc123.jappi.app" || len(ing.Spec.TLS) != 1 {
		t.Errorf("ingress mal formado: %+v", ing.Spec)
	}
}

func TestRedeployKeepsSelectorAndChangesTemplate(t *testing.T) {
	c, client := setup(t)
	_, w1 := plan(t, "dep_1")
	_, w2 := plan(t, "dep_2")
	if err := c.ApplyWorkload(ctx, w1); err != nil {
		t.Fatal(err)
	}
	before, _ := client.AppsV1().Deployments("prj-p1").Get(ctx, "api", metav1.GetOptions{})
	if err := c.ApplyWorkload(ctx, w2); err != nil {
		t.Fatal(err)
	}
	after, _ := client.AppsV1().Deployments("prj-p1").Get(ctx, "api", metav1.GetOptions{})

	if !equalMaps(before.Spec.Selector.MatchLabels, after.Spec.Selector.MatchLabels) {
		t.Error("el selector cambió entre despliegues; Kubernetes lo rechazaría")
	}
	if after.Spec.Template.Annotations["jappi.dev/deployment-id"] != "dep_2" {
		t.Error("un redeploy debe cambiar la plantilla para reiniciar los pods")
	}
}

func TestRolloutReportsOnlyProblemsOfTheNewVersion(t *testing.T) {
	c, client := setup(t)
	_, w := plan(t, "dep_2")
	if err := c.ApplyWorkload(ctx, w); err != nil {
		t.Fatal(err)
	}

	d, _ := client.AppsV1().Deployments("prj-p1").Get(ctx, "api", metav1.GetOptions{})
	d.Status = appsv1.DeploymentStatus{ObservedGeneration: d.Generation, UpdatedReplicas: 1}
	if _, err := client.AppsV1().Deployments("prj-p1").UpdateStatus(ctx, d, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	addPod(t, client, "old", "dep_1", "CrashLoopBackOff")

	s, err := c.Rollout(ctx, "prj-p1", "api")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.PodProblems) != 0 {
		t.Fatalf("el pod viejo no debe afectar al despliegue nuevo: %v", s.PodProblems)
	}

	addPod(t, client, "new", "dep_2", "ImagePullBackOff")
	s, _ = c.Rollout(ctx, "prj-p1", "api")
	if phase, _ := domain.Evaluate(s); phase != domain.Failed {
		t.Fatalf("ImagePullBackOff en el pod nuevo debe fallar el despliegue: %+v", s)
	}
}

func TestRolloutOfMissingDeployment(t *testing.T) {
	c, _ := setup(t)
	s, err := c.Rollout(ctx, "prj-p1", "nada")
	if err != nil || s.Found {
		t.Fatalf("un Deployment inexistente es 'aún no creado', no un error: %+v %v", s, err)
	}
}

func addPod(t *testing.T, client *fake.Clientset, name, deploymentID, reason string) {
	t.Helper()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "prj-p1",
			Labels:      map[string]string{domain.LabelProject: "p1", domain.LabelService: "svc_api"},
			Annotations: map[string]string{"jappi.dev/deployment-id": deploymentID},
		},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
			{Name: "app", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason}}},
		}},
	}
	if _, err := client.CoreV1().Pods("prj-p1").Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func equalMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
