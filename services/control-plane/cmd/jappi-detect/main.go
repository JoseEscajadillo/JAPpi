// Command jappi-detect analiza una carpeta local y muestra lo que JAPpi haría
// con ella: servicios detectados, dominios y variables cableadas. Es otro
// adaptador de entrada (CLI) sobre el mismo caso de uso que usará el dashboard.
//
//	go run ./services/control-plane/cmd/jappi-detect D:\ruta\a\un\repo
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/adapters/out/repofs"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/app"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/stack"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "uso: jappi-detect <carpeta-del-repo>")
		os.Exit(2)
	}
	dir := os.Args[1]
	uc := app.AnalyzeRepository{
		Source:     repofs.Local{Dir: dir},
		Analyzer:   stack.DefaultAnalyzer(),
		BaseDomain: "jappi.localhost",
	}
	slug := filepath.Base(filepath.Clean(dir))
	analysis, err := uc.Execute(context.Background(), slug, "HEAD", "local-"+slug, slug)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(analysis)
}
