package stack

import (
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Límites del escaneo para que un repo enorme no bloquee el análisis.
const (
	maxScannedFiles = 3000
	maxFileSize     = 256 << 10
)

var envFiles = []string{".env.example", ".env.sample", ".env.template", ".env.local.example", ".env.development.example"}

var skipDirs = []string{
	"node_modules", ".git", "dist", "build", "out", ".next", ".nuxt", ".svelte-kit",
	".turbo", "coverage", "vendor", "venv", ".venv", "__pycache__",
}

var sourceExts = []string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".vue", ".svelte", ".astro", ".go", ".py"}

// Patrones de lectura de variables en cada lenguaje soportado.
var envRefPatterns = []*regexp.Regexp{
	regexp.MustCompile(`process\.env\.([A-Z][A-Z0-9_]*)`),
	regexp.MustCompile(`process\.env\[\s*['"]([A-Z][A-Z0-9_]*)['"]\s*\]`),
	regexp.MustCompile(`import\.meta\.env\.([A-Z][A-Z0-9_]*)`),
	regexp.MustCompile(`os\.(?:Getenv|LookupEnv)\(\s*"([A-Z][A-Z0-9_]*)"\s*\)`),
	regexp.MustCompile(`os\.(?:getenv|environ\.get)\(\s*['"]([A-Z][A-Z0-9_]*)['"]`),
	regexp.MustCompile(`os\.environ\[\s*['"]([A-Z][A-Z0-9_]*)['"]\s*\]`),
}

var svelteImport = regexp.MustCompile(`import\s*\{([^}]*)\}\s*from\s*['"]\$env/(?:static|dynamic)/(?:public|private)['"]`)

// collectEnvHints devuelve, ordenadas y sin repetir, las variables que el
// servicio en dir declara en sus .env de ejemplo o lee desde su código.
func collectEnvHints(fsys fs.FS, dir string) ([]string, error) {
	found := map[string]bool{}

	for _, name := range envFiles {
		if raw, ok := readFile(fsys, path.Join(dir, name)); ok {
			for _, k := range parseEnvKeys(raw) {
				found[k] = true
			}
		}
	}

	scanned := 0
	err := fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // un archivo ilegible no invalida el análisis
		}
		if d.IsDir() {
			if p != dir && slices.Contains(skipDirs, d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if scanned >= maxScannedFiles {
			return fs.SkipAll
		}
		if !slices.Contains(sourceExts, path.Ext(p)) {
			return nil
		}
		if info, err := d.Info(); err != nil || info.Size() > maxFileSize {
			return nil
		}
		scanned++
		src, ok := readFile(fsys, p)
		if !ok {
			return nil
		}
		for _, re := range envRefPatterns {
			for _, m := range re.FindAllStringSubmatch(src, -1) {
				if len(m) > 1 {
					found[m[1]] = true
				}
			}
		}
		for _, m := range svelteImport.FindAllStringSubmatch(src, -1) {
			for _, name := range strings.Split(m[1], ",") {
				if name = strings.TrimSpace(name); name != "" {
					found[name] = true
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(found))
	for k := range found {
		out = append(out, k)
	}
	slices.Sort(out)
	return out, nil
}

// parseEnvKeys lee las claves de un archivo .env (KEY=valor, "export KEY=...").
func parseEnvKeys(raw string) []string {
	var keys []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export "))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, _, ok := strings.Cut(line, "="); ok && k != "" {
			keys = append(keys, strings.TrimSpace(k))
		}
	}
	return keys
}
