// Package http recibe los webhooks de la GitHub App.
package http

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/JoseEscajadillo/JAPpi/services/github-integration/internal/app"
	"github.com/JoseEscajadillo/JAPpi/services/github-integration/internal/domain"
)

// maxBody es el tamaño máximo de un webhook de GitHub (25 MB).
const maxBody = 25 << 20

// Webhook atiende POST /webhooks/github.
type Webhook struct {
	Secret      []byte
	ReceivePush app.ReceivePush
}

// NewHandler monta las rutas del servicio.
func NewHandler(wh Webhook) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /webhooks/github", wh)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return mux
}

func (wh Webhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		http.Error(w, "cuerpo demasiado grande o ilegible", http.StatusRequestEntityTooLarge)
		return
	}
	// Primero la firma: sin ella no se interpreta nada del cuerpo.
	if !domain.VerifySignature(wh.Secret, body, r.Header.Get("X-Hub-Signature-256")) {
		http.Error(w, "firma inválida", http.StatusUnauthorized)
		return
	}

	switch event := r.Header.Get("X-GitHub-Event"); event {
	case "ping":
		w.WriteHeader(http.StatusOK)
	case "push":
		push, ok, err := domain.ParsePush(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !ok {
			w.WriteHeader(http.StatusAccepted) // tag o rama borrada: nada que desplegar
			return
		}
		if err := wh.ReceivePush.Execute(r.Context(), r.Header.Get("X-GitHub-Delivery"), push); err != nil {
			slog.Error("no se pudo publicar el push", "repo", push.Repository, "err", err)
			// 5xx hace que GitHub permita reenviar la entrega.
			http.Error(w, "error interno", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	default:
		w.WriteHeader(http.StatusAccepted) // eventos que aún no usamos
	}
}
