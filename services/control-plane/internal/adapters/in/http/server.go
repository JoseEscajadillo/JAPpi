// Package http es el adaptador de entrada HTTP del control-plane: traduce
// peticiones del dashboard a casos de uso y respuestas JSON.
package http

import (
	"encoding/json"
	"net/http"
)

// NewHandler monta las rutas.
//
// TODO(equipo): rutas de la API pública (proyectos, variables, despliegues,
// rollback) con autenticación. Cada ruta llama a un caso de uso de
// internal/app y nada más.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
