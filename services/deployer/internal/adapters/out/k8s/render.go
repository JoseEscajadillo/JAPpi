// Package k8s implementa los puertos del deployer sobre la API de
// Kubernetes con server-side apply: se declara el estado completo de cada
// objeto y el API server calcula la diferencia, así que aplicar dos veces es
// inofensivo.
//
// Este archivo solo traduce decisiones del dominio a objetos; no decide
// nada (los límites, los puertos permitidos o el UID vienen de domain).
package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsv1ac "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	netv1ac "k8s.io/client-go/applyconfigurations/networking/v1"

	"github.com/JoseEscajadillo/JAPpi/services/deployer/internal/domain"
)

// Config son los detalles del clúster concreto.
type Config struct {
	// RuntimeClass para el código de los clientes: "gvisor" en producción,
	// vacío en clústeres de desarrollo que no lo tienen.
	RuntimeClass string
	// IngressClass del controlador de ingress ("traefik" en K3s).
	IngressClass string
	// IngressNamespace es donde corre el controlador de ingress; es el único
	// origen externo que puede llegar a los pods ("kube-system" en K3s).
	IngressNamespace string
	// FieldManager identifica a JAPpi como dueño de los campos que aplica.
	FieldManager string
}

const (
	quotaName            = "jappi-quota"
	limitRangeName       = "jappi-defaults"
	containerName        = "app"
	portName             = "http"
	annotationDeployment = "jappi.dev/deployment-id"
	progressDeadline     = 300
	dnsNamespace         = "kube-system"
)

func mi(n int) resource.Quantity    { return *resource.NewQuantity(int64(n)<<20, resource.BinarySI) }
func gi(n int) resource.Quantity    { return *resource.NewQuantity(int64(n)<<30, resource.BinarySI) }
func milli(n int) resource.Quantity { return *resource.NewMilliQuantity(int64(n), resource.DecimalSI) }

func renderNamespace(p domain.Project) *corev1ac.NamespaceApplyConfiguration {
	labels := clone(p.Labels)
	// Pod Security Admission en modo restricted: el API server rechaza
	// cualquier pod privilegiado, con root o con capacidades extra, aunque
	// alguien se salte el deployer.
	for _, mode := range []string{"enforce", "audit", "warn"} {
		labels["pod-security.kubernetes.io/"+mode] = "restricted"
		labels["pod-security.kubernetes.io/"+mode+"-version"] = "latest"
	}
	return corev1ac.Namespace(p.Namespace).WithLabels(labels)
}

func renderQuota(p domain.Project) *corev1ac.ResourceQuotaApplyConfiguration {
	l := p.Limits
	return corev1ac.ResourceQuota(quotaName, p.Namespace).WithLabels(p.Labels).
		WithSpec(corev1ac.ResourceQuotaSpec().WithHard(corev1.ResourceList{
			corev1.ResourceRequestsCPU:     milli(l.QuotaCPURequestMilli),
			corev1.ResourceRequestsMemory:  mi(l.QuotaMemRequestMiB),
			corev1.ResourceLimitsMemory:    mi(l.QuotaMemLimitMiB),
			corev1.ResourcePods:            *resource.NewQuantity(int64(l.QuotaPods), resource.DecimalSI),
			corev1.ResourceRequestsStorage: gi(l.QuotaStorageGiB),
		}))
}

// renderLimitRange da recursos por defecto a los pods que no los declaran
// (p. ej. los de los add-ons); sin ellos la cuota los rechazaría.
func renderLimitRange(p domain.Project) *corev1ac.LimitRangeApplyConfiguration {
	l := p.Limits
	return corev1ac.LimitRange(limitRangeName, p.Namespace).WithLabels(p.Labels).
		WithSpec(corev1ac.LimitRangeSpec().WithLimits(corev1ac.LimitRangeItem().
			WithType(corev1.LimitTypeContainer).
			WithDefault(corev1.ResourceList{corev1.ResourceCPU: milli(l.CPULimitMilli), corev1.ResourceMemory: mi(l.MemLimitMiB)}).
			WithDefaultRequest(corev1.ResourceList{corev1.ResourceCPU: milli(l.CPURequestMilli), corev1.ResourceMemory: mi(l.MemRequestMiB)})))
}

// renderNetworkPolicies: todo denegado salvo lo explícito. Las políticas
// de red se suman, así que cada una abre un camino concreto.
func renderNetworkPolicies(p domain.Project, cfg Config) []*netv1ac.NetworkPolicyApplyConfiguration {
	all := func() *metav1ac.LabelSelectorApplyConfiguration { return metav1ac.LabelSelector() }
	nsSelector := func(name string) *metav1ac.LabelSelectorApplyConfiguration {
		return metav1ac.LabelSelector().WithMatchLabels(map[string]string{"kubernetes.io/metadata.name": name})
	}
	policy := func(name string) *netv1ac.NetworkPolicyApplyConfiguration {
		return netv1ac.NetworkPolicy(name, p.Namespace).WithLabels(p.Labels)
	}
	port := func(proto corev1.Protocol, n int32) *netv1ac.NetworkPolicyPortApplyConfiguration {
		return netv1ac.NetworkPolicyPort().WithProtocol(proto).WithPort(intstr.FromInt32(n))
	}

	internetPorts := make([]*netv1ac.NetworkPolicyPortApplyConfiguration, 0, len(domain.EgressPorts))
	for _, n := range domain.EgressPorts {
		internetPorts = append(internetPorts, port(corev1.ProtocolTCP, n))
	}

	return []*netv1ac.NetworkPolicyApplyConfiguration{
		policy("default-deny").WithSpec(netv1ac.NetworkPolicySpec().
			WithPodSelector(all()).
			WithPolicyTypes(networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress)),

		// Los servicios del mismo proyecto se hablan entre sí (api ↔ postgres).
		policy("allow-same-project").WithSpec(netv1ac.NetworkPolicySpec().
			WithPodSelector(all()).
			WithPolicyTypes(networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress).
			WithIngress(netv1ac.NetworkPolicyIngressRule().WithFrom(netv1ac.NetworkPolicyPeer().WithPodSelector(all()))).
			WithEgress(netv1ac.NetworkPolicyEgressRule().WithTo(netv1ac.NetworkPolicyPeer().WithPodSelector(all())))),

		// El tráfico público solo entra a través del ingress controller.
		policy("allow-ingress-controller").WithSpec(netv1ac.NetworkPolicySpec().
			WithPodSelector(all()).
			WithPolicyTypes(networkingv1.PolicyTypeIngress).
			WithIngress(netv1ac.NetworkPolicyIngressRule().WithFrom(
				netv1ac.NetworkPolicyPeer().WithNamespaceSelector(nsSelector(cfg.IngressNamespace))))),

		policy("allow-dns").WithSpec(netv1ac.NetworkPolicySpec().
			WithPodSelector(all()).
			WithPolicyTypes(networkingv1.PolicyTypeEgress).
			WithEgress(netv1ac.NetworkPolicyEgressRule().
				WithTo(netv1ac.NetworkPolicyPeer().WithNamespaceSelector(nsSelector(dnsNamespace))).
				WithPorts(port(corev1.ProtocolUDP, 53), port(corev1.ProtocolTCP, 53)))),

		policy("allow-internet").WithSpec(netv1ac.NetworkPolicySpec().
			WithPodSelector(all()).
			WithPolicyTypes(networkingv1.PolicyTypeEgress).
			WithEgress(netv1ac.NetworkPolicyEgressRule().
				WithTo(netv1ac.NetworkPolicyPeer().WithIPBlock(
					netv1ac.IPBlock().WithCIDR("0.0.0.0/0").WithExcept(domain.BlockedEgressCIDRs...))).
				WithPorts(internetPorts...))),
	}
}

// selectorLabels son las labels que identifican los pods de un servicio.
// Nunca cambian entre despliegues: el selector de un Deployment es inmutable.
func selectorLabels(w domain.Workload) map[string]string {
	return map[string]string{
		domain.LabelProject: w.Labels[domain.LabelProject],
		domain.LabelService: w.Labels[domain.LabelService],
	}
}

func renderDeployment(w domain.Workload, cfg Config) *appsv1ac.DeploymentApplyConfiguration {
	l := w.Limits
	annotations := map[string]string{annotationDeployment: w.DeploymentID}

	env := make([]*corev1ac.EnvVarApplyConfiguration, 0, len(w.Env))
	for _, e := range w.Env {
		v := corev1ac.EnvVar().WithName(e.Name)
		if e.Secret != nil {
			v.WithValueFrom(corev1ac.EnvVarSource().WithSecretKeyRef(
				corev1ac.SecretKeySelector().WithName(e.Secret.Name).WithKey(e.Secret.Key)))
		} else {
			v.WithValue(e.Value)
		}
		env = append(env, v)
	}

	container := corev1ac.Container().
		WithName(containerName).
		WithImage(w.Image).
		WithPorts(corev1ac.ContainerPort().WithName(portName).WithContainerPort(int32(w.Port)).WithProtocol(corev1.ProtocolTCP)).
		WithEnv(env...).
		WithResources(corev1ac.ResourceRequirements().
			WithRequests(corev1.ResourceList{corev1.ResourceCPU: milli(l.CPURequestMilli), corev1.ResourceMemory: mi(l.MemRequestMiB)}).
			WithLimits(corev1.ResourceList{corev1.ResourceCPU: milli(l.CPULimitMilli), corev1.ResourceMemory: mi(l.MemLimitMiB)})).
		// TCP y no HTTP: no sabemos qué ruta responde en la app del cliente.
		WithReadinessProbe(corev1ac.Probe().
			WithTCPSocket(corev1ac.TCPSocketAction().WithPort(intstr.FromString(portName))).
			WithPeriodSeconds(5).WithFailureThreshold(3)).
		WithSecurityContext(corev1ac.SecurityContext().
			WithAllowPrivilegeEscalation(false).
			WithRunAsNonRoot(true).
			WithRunAsUser(domain.RunAsUser).
			WithRunAsGroup(domain.RunAsGroup).
			WithCapabilities(corev1ac.Capabilities().WithDrop("ALL")).
			WithSeccompProfile(corev1ac.SeccompProfile().WithType(corev1.SeccompProfileTypeRuntimeDefault))).
		WithVolumeMounts(corev1ac.VolumeMount().WithName("tmp").WithMountPath("/tmp"))

	pod := corev1ac.PodSpec().
		WithContainers(container).
		// El código del cliente no necesita hablar con la API de Kubernetes.
		WithAutomountServiceAccountToken(false).
		// Evita inyectar variables de todos los Services del namespace.
		WithEnableServiceLinks(false).
		WithSecurityContext(corev1ac.PodSecurityContext().
			WithRunAsNonRoot(true).
			WithSeccompProfile(corev1ac.SeccompProfile().WithType(corev1.SeccompProfileTypeRuntimeDefault))).
		WithVolumes(corev1ac.Volume().WithName("tmp").
			WithEmptyDir(corev1ac.EmptyDirVolumeSource().WithSizeLimit(mi(256))))
	if cfg.RuntimeClass != "" {
		pod.WithRuntimeClassName(cfg.RuntimeClass)
	}

	return appsv1ac.Deployment(w.Name, w.Namespace).
		WithLabels(w.Labels).
		WithAnnotations(annotations).
		WithSpec(appsv1ac.DeploymentSpec().
			WithReplicas(l.Replicas).
			WithSelector(metav1ac.LabelSelector().WithMatchLabels(selectorLabels(w))).
			WithRevisionHistoryLimit(5).
			WithProgressDeadlineSeconds(progressDeadline).
			// Sin caída: el pod viejo sirve hasta que el nuevo está listo.
			WithStrategy(appsv1ac.DeploymentStrategy().
				WithType(appsv1.RollingUpdateDeploymentStrategyType).
				WithRollingUpdate(appsv1ac.RollingUpdateDeployment().
					WithMaxUnavailable(intstr.FromInt32(0)).
					WithMaxSurge(intstr.FromInt32(1)))).
			// La anotación cambia en cada despliegue: un redeploy de la misma
			// imagen también reinicia los pods.
			WithTemplate(corev1ac.PodTemplateSpec().
				WithLabels(w.Labels).
				WithAnnotations(annotations).
				WithSpec(pod)))
}

func renderService(w domain.Workload) *corev1ac.ServiceApplyConfiguration {
	return corev1ac.Service(w.Name, w.Namespace).WithLabels(w.Labels).
		WithSpec(corev1ac.ServiceSpec().
			WithSelector(selectorLabels(w)).
			WithPorts(corev1ac.ServicePort().
				WithName(portName).WithPort(80).
				WithTargetPort(intstr.FromString(portName)).
				WithProtocol(corev1.ProtocolTCP)))
}

// renderIngress publica el servicio en su dominio. El TLS no lleva
// secretName: el ingress controller usa el certificado wildcard por
// defecto (ADR-0009), así un dominio nuevo no espera a que se emita uno.
func renderIngress(w domain.Workload, cfg Config) *netv1ac.IngressApplyConfiguration {
	spec := netv1ac.IngressSpec().
		WithTLS(netv1ac.IngressTLS().WithHosts(w.Host)).
		WithRules(netv1ac.IngressRule().WithHost(w.Host).WithHTTP(
			netv1ac.HTTPIngressRuleValue().WithPaths(netv1ac.HTTPIngressPath().
				WithPath("/").
				WithPathType(networkingv1.PathTypePrefix).
				WithBackend(netv1ac.IngressBackend().WithService(
					netv1ac.IngressServiceBackend().WithName(w.Name).
						WithPort(netv1ac.ServiceBackendPort().WithName(portName)))))))
	if cfg.IngressClass != "" {
		spec.WithIngressClassName(cfg.IngressClass)
	}
	return netv1ac.Ingress(w.Name, w.Namespace).WithLabels(w.Labels).WithSpec(spec)
}

func clone(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
