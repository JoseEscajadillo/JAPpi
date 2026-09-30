package stack

import "strings"

// AffectedServices devuelve los servicios que hay que redesplegar tras un
// push. En un monorepo, tocar apps/web no debe reconstruir apps/api.
//
// La regla es conservadora: un archivo que no pertenece a ningún servicio
// (lockfile de la raíz, paquete compartido, turbo.json...) puede afectar a
// cualquiera, así que redespliega todos. Si la lista de archivos está
// incompleta, también.
func AffectedServices(plan Plan, changed []string, complete bool) []Service {
	if !complete || !plan.Monorepo || len(changed) == 0 {
		return plan.Services
	}
	hit := make(map[string]bool)
	for _, f := range changed {
		owner, ok := ownerOf(plan, f)
		if !ok {
			return plan.Services
		}
		hit[owner] = true
	}
	var out []Service
	for _, s := range plan.Services {
		if hit[s.Name] {
			out = append(out, s)
		}
	}
	return out
}

func ownerOf(plan Plan, file string) (string, bool) {
	for _, s := range plan.Services {
		if strings.HasPrefix(file, s.Path+"/") {
			return s.Name, true
		}
	}
	return "", false
}
