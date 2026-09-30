package app

import (
	"context"
	"fmt"

	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/domains"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/stack"
	"github.com/JoseEscajadillo/JAPpi/services/control-plane/internal/domain/wiring"
)

// AnalyzeRepository muestra, antes de crear un proyecto, qué servicios
// detectó JAPpi, qué dominios les tocarían y qué variables se cablearían.
// Es lo que ve el usuario en la pantalla "Importar repositorio".
type AnalyzeRepository struct {
	Source     RepoSource
	Analyzer   *stack.Analyzer
	BaseDomain string
}

// Analysis es la vista previa del proyecto.
type Analysis struct {
	Plan    stack.Plan        `json:"plan"`
	Domains map[string]string `json:"domains"`
	Wiring  wiring.Result     `json:"wiring"`
}

// Execute analiza repository en ref. projectID/projectSlug son los que
// tendrá el proyecto si el usuario lo confirma.
func (uc AnalyzeRepository) Execute(ctx context.Context, repository, ref, projectID, projectSlug string) (Analysis, error) {
	fsys, err := uc.Source.Open(ctx, repository, ref)
	if err != nil {
		return Analysis{}, fmt.Errorf("abrir %s@%s: %w", repository, ref, err)
	}
	plan, err := uc.Analyzer.Analyze(fsys)
	if err != nil {
		return Analysis{}, fmt.Errorf("analizar %s: %w", repository, err)
	}
	doms := make(map[string]string, len(plan.Services))
	for _, s := range plan.Services {
		doms[s.Name] = domains.ForService(s.Name, projectSlug, projectID, uc.BaseDomain)
	}
	return Analysis{
		Plan:    plan,
		Domains: doms,
		Wiring:  wiring.Resolve(wiring.Input{Plan: plan, Domains: doms}),
	}, nil
}
