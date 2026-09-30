package domain

// Política de aislamiento de los proyectos (ADR-0012). Son decisiones de
// negocio y de seguridad, por eso viven en el dominio; el adaptador k8s
// solo las traduce a NetworkPolicy y securityContext.

// EgressPorts son los puertos TCP a los que el código del cliente puede
// conectarse fuera del clúster. Sin SMTP (25/465/587): frena el spam. Sin
// puertos arbitrarios: dificulta la minería contra pools en puertos típicos.
var EgressPorts = []int32{80, 443}

// BlockedEgressCIDRs son redes a las que nunca se sale aunque el puerto
// esté permitido: redes privadas (otros proyectos, el plano de control) y
// el servicio de metadatos del proveedor cloud (169.254.169.254), que
// podría filtrar credenciales del nodo.
var BlockedEgressCIDRs = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"100.64.0.0/10",
	"169.254.0.0/16",
}

// RunAsUser es el UID fijo con el que corre todo contenedor de un cliente.
// Fijarlo evita depender del USER de la imagen (que puede ser root o un
// nombre que el kubelet no puede verificar). El builder genera imágenes
// que funcionan con cualquier UID del grupo 0, al estilo de OpenShift.
const RunAsUser int64 = 10001

// RunAsGroup es 0 (grupo root, no usuario root): las imágenes dan permisos
// de grupo a sus carpetas escribibles, así funcionan con cualquier UID.
const RunAsGroup int64 = 0
