package domain

import "fmt"

// Tier es el plan de suscripción del dueño del proyecto.
type Tier string

const (
	TierTrial Tier = "trial" // 30 días con los límites de hobby (ADR-0010)
	TierHobby Tier = "hobby" // $5
	TierPro   Tier = "pro"   // $12
	TierTeam  Tier = "team"  // $20
)

// Limits traduce un plan a recursos (ADR-0012). Las cuotas de CPU van sobre
// requests, no sobre limits: así los contenedores pueden usar CPU ociosa
// del nodo, pero el reparto garantizado queda acotado. La memoria sí se
// acota por límites, porque no se puede "devolver" una vez usada.
type Limits struct {
	// Cuota total del namespace del proyecto.
	QuotaCPURequestMilli int
	QuotaMemRequestMiB   int
	QuotaMemLimitMiB     int
	QuotaPods            int
	QuotaStorageGiB      int

	// Por contenedor de la aplicación.
	CPURequestMilli int
	CPULimitMilli   int
	MemRequestMiB   int
	MemLimitMiB     int

	Replicas int32
}

// Valores iniciales, pensados para nodos Hetzner CX22/CX32. Se recalibran
// con datos reales de uso (ver ADR-0012).
var tierLimits = map[Tier]Limits{
	TierHobby: {
		QuotaCPURequestMilli: 500, QuotaMemRequestMiB: 1024, QuotaMemLimitMiB: 2048, QuotaPods: 10, QuotaStorageGiB: 5,
		CPURequestMilli: 100, CPULimitMilli: 1000, MemRequestMiB: 192, MemLimitMiB: 384,
		Replicas: 1,
	},
	TierPro: {
		QuotaCPURequestMilli: 1000, QuotaMemRequestMiB: 2048, QuotaMemLimitMiB: 4096, QuotaPods: 20, QuotaStorageGiB: 20,
		CPURequestMilli: 200, CPULimitMilli: 2000, MemRequestMiB: 384, MemLimitMiB: 768,
		Replicas: 1,
	},
	TierTeam: {
		QuotaCPURequestMilli: 2000, QuotaMemRequestMiB: 4096, QuotaMemLimitMiB: 8192, QuotaPods: 40, QuotaStorageGiB: 50,
		CPURequestMilli: 200, CPULimitMilli: 2000, MemRequestMiB: 384, MemLimitMiB: 768,
		Replicas: 2, // alta disponibilidad: sobrevive a la caída de un pod
	},
}

func init() { tierLimits[TierTrial] = tierLimits[TierHobby] }

// LimitsFor devuelve los recursos de un plan.
func LimitsFor(t Tier) (Limits, error) {
	l, ok := tierLimits[t]
	if !ok {
		return Limits{}, fmt.Errorf("plan desconocido %q", t)
	}
	return l, nil
}
