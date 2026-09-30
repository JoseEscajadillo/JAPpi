// Package archtest convierte las reglas de la arquitectura hexagonal en
// pruebas: si alguien importa un adaptador desde el dominio, `go test` falla.
//
// Reglas (ver ADR-0006):
//   - internal/domain  -> solo la librería estándar y el propio dominio.
//   - internal/app     -> librería estándar, dominio y pkg/contracts.
//   - internal/adapters y cmd -> sin restricciones (son el borde del hexágono).
package archtest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/JoseEscajadillo/JAPpi"

// CheckHexagon revisa los imports de los archivos no-test bajo dir/internal.
// servicePkg es la ruta de importación del servicio, p. ej.
// "github.com/JoseEscajadillo/JAPpi/services/control-plane".
func CheckHexagon(t *testing.T, dir, servicePkg string) {
	t.Helper()
	root := filepath.Join(dir, "internal")
	domain := servicePkg + "/internal/domain"
	app := servicePkg + "/internal/app"

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		layer := strings.Split(filepath.ToSlash(rel), "/")[0]

		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			var ok bool
			switch layer {
			case "domain":
				ok = isStdlib(p) || under(p, domain)
			case "app":
				ok = isStdlib(p) || under(p, domain) || under(p, app) || under(p, module+"/pkg/contracts")
			default:
				ok = true
			}
			if !ok {
				t.Errorf("%s: la capa %q no puede importar %q (ver docs/adr/0006)", filepath.ToSlash(rel), layer, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func isStdlib(p string) bool { return !strings.Contains(strings.Split(p, "/")[0], ".") }

func under(p, prefix string) bool { return p == prefix || strings.HasPrefix(p, prefix+"/") }
