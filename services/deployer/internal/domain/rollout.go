package domain

import "fmt"

// RolloutStatus es lo que el deployer observa de un servicio en el clúster.
type RolloutStatus struct {
	Found              bool
	Generation         int64
	ObservedGeneration int64
	Desired            int32
	Updated            int32
	Available          int32
	// DeadlineExceeded: Kubernetes marcó ProgressDeadlineExceeded.
	DeadlineExceeded bool
	// ReplicaFailure: el controlador no pudo crear pods (cuota agotada,
	// Pod Security Admission los rechazó...). Contiene el mensaje.
	ReplicaFailure string
	// PodProblems son motivos de espera de los contenedores de la versión
	// nueva (CrashLoopBackOff, ImagePullBackOff...).
	PodProblems []string
}

// Phase es la conclusión sobre un rollout.
type Phase int

const (
	Progressing Phase = iota
	Complete
	Failed
)

// fatalWaitReasons son estados de un contenedor que no se arreglan esperando.
var fatalWaitReasons = map[string]string{
	"CrashLoopBackOff":           "la aplicación se cierra al arrancar (revisa los logs)",
	"ImagePullBackOff":           "no se pudo descargar la imagen",
	"ErrImagePull":               "no se pudo descargar la imagen",
	"InvalidImageName":           "nombre de imagen inválido",
	"CreateContainerConfigError": "configuración inválida (¿falta un Secret referenciado?)",
	"CreateContainerError":       "no se pudo crear el contenedor",
}

// Evaluate decide si el rollout terminó. El detalle se muestra al usuario.
func Evaluate(s RolloutStatus) (Phase, string) {
	switch {
	case !s.Found:
		return Progressing, "esperando a que se cree el Deployment"
	case s.ReplicaFailure != "":
		return Failed, "Kubernetes no pudo crear los pods: " + s.ReplicaFailure
	case s.DeadlineExceeded:
		return Failed, "el despliegue superó el tiempo máximo sin progresar"
	}
	for _, reason := range s.PodProblems {
		if human, fatal := fatalWaitReasons[reason]; fatal {
			return Failed, fmt.Sprintf("%s: %s", reason, human)
		}
	}
	if s.ObservedGeneration >= s.Generation && s.Updated == s.Desired && s.Available >= s.Desired && s.Desired > 0 {
		return Complete, fmt.Sprintf("%d/%d réplicas disponibles", s.Available, s.Desired)
	}
	return Progressing, fmt.Sprintf("%d/%d réplicas actualizadas, %d disponibles", s.Updated, s.Desired, s.Available)
}
