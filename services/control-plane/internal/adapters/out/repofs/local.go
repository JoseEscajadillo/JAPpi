// Package repofs implementa el puerto RepoSource.
package repofs

import (
	"context"
	"fmt"
	"io/fs"
	"os"
)

// Local sirve una carpeta del disco como si fuera el repo. Ignora repository
// y ref: es para desarrollo y para la CLI jappi-detect. El adaptador de
// producción descargará el tarball del commit desde la API de GitHub.
type Local struct {
	Dir string
}

func (l Local) Open(_ context.Context, _, _ string) (fs.FS, error) {
	st, err := os.Stat(l.Dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s no es una carpeta", l.Dir)
	}
	return os.DirFS(l.Dir), nil
}
