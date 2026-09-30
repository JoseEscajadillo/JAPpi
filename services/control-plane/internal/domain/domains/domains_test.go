package domains_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/domains"
)

var validLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func TestForService(t *testing.T) {
	a := domains.ForService("web", "Mi Tienda!", "p1", "jappi.app")
	if a != domains.ForService("web", "Mi Tienda!", "p1", "jappi.app") {
		t.Error("debe ser determinista")
	}
	if a == domains.ForService("web", "Mi Tienda!", "p2", "jappi.app") {
		t.Error("dos proyectos con el mismo nombre no pueden compartir dominio")
	}
	if !strings.HasPrefix(a, "web-mi-tienda-") || !strings.HasSuffix(a, ".jappi.app") {
		t.Errorf("formato inesperado: %s", a)
	}
}

func TestLabelIsAlwaysValidDNS(t *testing.T) {
	for _, slug := range []string{strings.Repeat("x", 200), "---", "ÑANDÚ_app", ""} {
		d := domains.ForService("frontend", slug, "id", "jappi.app")
		label := strings.TrimSuffix(d, ".jappi.app")
		if !validLabel.MatchString(label) {
			t.Errorf("etiqueta DNS inválida para %q: %q", slug, label)
		}
	}
}
