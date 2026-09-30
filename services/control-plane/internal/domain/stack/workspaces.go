package stack

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

type packageJSON struct {
	Name            string            `json:"name"`
	PackageManager  string            `json:"packageManager"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Workspaces      json.RawMessage   `json:"workspaces"`
}

func readPackageJSON(fsys fs.FS, name string) (packageJSON, bool, error) {
	raw, ok := readFile(fsys, name)
	if !ok {
		return packageJSON{}, false, nil
	}
	var p packageJSON
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return packageJSON{}, false, fmt.Errorf("stack: %s inválido: %w", name, err)
	}
	return p, true, nil
}

// workspaces acepta las dos formas de npm/yarn: ["apps/*"] o {"packages": ["apps/*"]}.
func (p packageJSON) workspaces() []string {
	if len(p.Workspaces) == 0 {
		return nil
	}
	var list []string
	if json.Unmarshal(p.Workspaces, &list) == nil {
		return list
	}
	var obj struct {
		Packages []string `json:"packages"`
	}
	if json.Unmarshal(p.Workspaces, &obj) == nil {
		return obj.Packages
	}
	return nil
}

// workspacePatterns lee las declaraciones de workspace del repo.
func workspacePatterns(fsys fs.FS) (patterns, tools []string, err error) {
	if raw, ok := readFile(fsys, "pnpm-workspace.yaml"); ok {
		patterns = append(patterns, parsePnpmWorkspace(raw)...)
		tools = append(tools, "pnpm-workspaces")
	}
	root, ok, err := readPackageJSON(fsys, "package.json")
	if err != nil {
		return nil, nil, err
	}
	if ok {
		if ws := root.workspaces(); len(ws) > 0 {
			patterns = append(patterns, ws...)
			tools = append(tools, packageManager(fsys)+"-workspaces")
		}
	}
	if raw, ok := readFile(fsys, "go.work"); ok {
		patterns = append(patterns, parseGoWork(raw)...)
		tools = append(tools, "go-workspace")
	}
	if exists(fsys, "turbo.json") {
		tools = append(tools, "turbo")
	}
	if exists(fsys, "nx.json") {
		tools = append(tools, "nx")
	}
	return patterns, tools, nil
}

// parsePnpmWorkspace extrae la lista "packages:" de pnpm-workspace.yaml.
// El formato real es siempre una lista simple; no hace falta un parser YAML
// completo (y así el dominio no depende de librerías externas).
func parsePnpmWorkspace(raw string) []string {
	var out []string
	inPackages := false
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(trimmed, "-") {
			inPackages = strings.HasPrefix(trimmed, "packages:")
			continue
		}
		if inPackages && strings.HasPrefix(trimmed, "-") {
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			out = append(out, strings.Trim(item, `"'`))
		}
	}
	return out
}

// parseGoWork extrae los directorios de las directivas "use" de go.work.
func parseGoWork(raw string) []string {
	var out []string
	inBlock := false
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.Split(line, "//")[0])
		switch {
		case line == "use (":
			inBlock = true
		case inBlock && line == ")":
			inBlock = false
		case inBlock && line != "":
			out = append(out, line)
		case strings.HasPrefix(line, "use "):
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "use ")))
		}
	}
	return out
}

// packageManager deduce el gestor de paquetes de Node a partir de la raíz.
func packageManager(fsys fs.FS) string {
	if root, ok, _ := readPackageJSON(fsys, "package.json"); ok && root.PackageManager != "" {
		return strings.SplitN(root.PackageManager, "@", 2)[0]
	}
	switch {
	case exists(fsys, "pnpm-lock.yaml"):
		return "pnpm"
	case exists(fsys, "yarn.lock"):
		return "yarn"
	case exists(fsys, "bun.lockb"), exists(fsys, "bun.lock"):
		return "bun"
	default:
		return "npm"
	}
}
