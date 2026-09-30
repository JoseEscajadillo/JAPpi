// Package domains asigna el dominio público de cada servicio.
//
// El dominio se reserva al crear el proyecto, antes del primer deploy
// (ADR-0009). Así el frontend conoce la URL del backend en su primer build,
// aunque el backend todavía no exista.
package domains

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// maxLabel es el límite de una etiqueta DNS (RFC 1035).
const maxLabel = 63

// ForService devuelve un dominio estable y único para un servicio:
// <servicio>-<proyecto>-<hash6>.<base>. El hash evita colisiones entre
// proyectos con el mismo nombre y hace el dominio difícil de adivinar.
func ForService(service, projectSlug, projectID, base string) string {
	sum := sha256.Sum256([]byte(projectID + "/" + service))
	suffix := "-" + hex.EncodeToString(sum[:])[:6]

	label := sanitize(service + "-" + projectSlug)
	if len(label) > maxLabel-len(suffix) {
		label = strings.TrimRight(label[:maxLabel-len(suffix)], "-")
	}
	return label + suffix + "." + base
}

// URL devuelve la URL pública (siempre HTTPS) de un dominio.
func URL(domain string) string { return "https://" + domain }

func sanitize(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "app"
	}
	return out
}
