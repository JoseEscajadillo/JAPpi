package k8s

import (
	"context"
	"fmt"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/domain"
)

// Cluster implementa ProjectProvisioner, WorkloadApplier y RolloutReader.
type Cluster struct {
	client kubernetes.Interface
	cfg    Config
}

// New crea el adaptador. Rellena los valores por defecto de K3s.
func New(client kubernetes.Interface, cfg Config) *Cluster {
	if cfg.FieldManager == "" {
		cfg.FieldManager = "jappi-deployer"
	}
	if cfg.IngressNamespace == "" {
		cfg.IngressNamespace = "kube-system"
	}
	return &Cluster{client: client, cfg: cfg}
}

// NewClient conecta con el clúster: con el kubeconfig indicado o, si está
// vacío, con la cuenta de servicio del pod (dentro del clúster).
func NewClient(kubeconfig string) (kubernetes.Interface, error) {
	var (
		conf *rest.Config
		err  error
	)
	if kubeconfig == "" {
		conf, err = rest.InClusterConfig()
	} else {
		conf, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if err != nil {
		return nil, fmt.Errorf("k8s: configuración del cliente: %w", err)
	}
	return kubernetes.NewForConfig(conf)
}

func (c *Cluster) apply() metav1.ApplyOptions {
	// Force: JAPpi es la dueña de estos objetos; si alguien los editó a
	// mano, el estado declarado gana.
	return metav1.ApplyOptions{FieldManager: c.cfg.FieldManager, Force: true}
}

// EnsureProject aplica namespace, cuota, límites por defecto y políticas de red.
func (c *Cluster) EnsureProject(ctx context.Context, p domain.Project) error {
	core := c.client.CoreV1()
	if _, err := core.Namespaces().Apply(ctx, renderNamespace(p), c.apply()); err != nil {
		return fmt.Errorf("namespace %s: %w", p.Namespace, err)
	}
	if _, err := core.ResourceQuotas(p.Namespace).Apply(ctx, renderQuota(p), c.apply()); err != nil {
		return fmt.Errorf("cuota de %s: %w", p.Namespace, err)
	}
	if _, err := core.LimitRanges(p.Namespace).Apply(ctx, renderLimitRange(p), c.apply()); err != nil {
		return fmt.Errorf("límites de %s: %w", p.Namespace, err)
	}
	for _, np := range renderNetworkPolicies(p, c.cfg) {
		if _, err := c.client.NetworkingV1().NetworkPolicies(p.Namespace).Apply(ctx, np, c.apply()); err != nil {
			return fmt.Errorf("política de red %s en %s: %w", *np.Name, p.Namespace, err)
		}
	}
	return nil
}

// ApplyWorkload aplica Deployment, Service e Ingress del servicio.
func (c *Cluster) ApplyWorkload(ctx context.Context, w domain.Workload) error {
	if _, err := c.client.AppsV1().Deployments(w.Namespace).Apply(ctx, renderDeployment(w, c.cfg), c.apply()); err != nil {
		return fmt.Errorf("deployment: %w", err)
	}
	if _, err := c.client.CoreV1().Services(w.Namespace).Apply(ctx, renderService(w), c.apply()); err != nil {
		return fmt.Errorf("service: %w", err)
	}
	if _, err := c.client.NetworkingV1().Ingresses(w.Namespace).Apply(ctx, renderIngress(w, c.cfg), c.apply()); err != nil {
		return fmt.Errorf("ingress: %w", err)
	}
	return nil
}

// Rollout traduce el estado del Deployment y de sus pods nuevos.
func (c *Cluster) Rollout(ctx context.Context, namespace, name string) (domain.RolloutStatus, error) {
	d, err := c.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return domain.RolloutStatus{}, nil
	}
	if err != nil {
		return domain.RolloutStatus{}, err
	}

	s := domain.RolloutStatus{
		Found:              true,
		Generation:         d.Generation,
		ObservedGeneration: d.Status.ObservedGeneration,
		Updated:            d.Status.UpdatedReplicas,
		Available:          d.Status.AvailableReplicas,
		Desired:            1,
	}
	if d.Spec.Replicas != nil {
		s.Desired = *d.Spec.Replicas
	}
	for _, cond := range d.Status.Conditions {
		switch {
		case cond.Type == appsv1.DeploymentReplicaFailure && cond.Status == corev1.ConditionTrue:
			s.ReplicaFailure = cond.Message
		case cond.Type == appsv1.DeploymentProgressing && cond.Status == corev1.ConditionFalse && cond.Reason == "ProgressDeadlineExceeded":
			s.DeadlineExceeded = true
		}
	}

	problems, err := c.podProblems(ctx, d)
	if err != nil {
		return domain.RolloutStatus{}, err
	}
	s.PodProblems = problems
	return s, nil
}

// podProblems mira solo los pods de la versión que se está desplegando: un
// pod viejo en CrashLoopBackOff no debe hacer fallar el despliegue nuevo.
func (c *Cluster) podProblems(ctx context.Context, d *appsv1.Deployment) ([]string, error) {
	selector := labels.SelectorFromSet(d.Spec.Selector.MatchLabels).String()
	pods, err := c.client.CoreV1().Pods(d.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("listar pods: %w", err)
	}
	current := d.Spec.Template.Annotations[annotationDeployment]
	var out []string
	for _, pod := range pods.Items {
		if pod.Annotations[annotationDeployment] != current {
			continue
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if w := cs.State.Waiting; w != nil && w.Reason != "" && !slices.Contains(out, w.Reason) {
				out = append(out, w.Reason)
			}
		}
	}
	return out, nil
}
