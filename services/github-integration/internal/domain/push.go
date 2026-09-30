// Package domain contiene las reglas de los webhooks de GitHub: verificar
// que el mensaje viene de GitHub y extraer de un push lo que importa.
package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// maxCommitsInPayload es el tope de commits que GitHub incluye en un push.
// Si llega ese número, la lista de archivos puede estar incompleta.
const maxCommitsInPayload = 20

// VerifySignature comprueba la cabecera X-Hub-Signature-256 en tiempo constante.
func VerifySignature(secret, body []byte, header string) bool {
	sig, ok := strings.CutPrefix(header, "sha256=")
	if !ok || len(secret) == 0 {
		return false
	}
	got, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}

// Push es lo que JAPpi necesita saber de un push.
type Push struct {
	Repository           string
	Branch               string
	CommitSHA            string
	CommitMessage        string
	Pusher               string
	InstallationID       int64
	ChangedFiles         []string
	ChangedFilesComplete bool
}

type pushPayload struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	Deleted    bool   `json:"deleted"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
	Pusher struct {
		Name string `json:"name"`
	} `json:"pusher"`
	HeadCommit *struct {
		Message string `json:"message"`
	} `json:"head_commit"`
	Commits []struct {
		Added    []string `json:"added"`
		Removed  []string `json:"removed"`
		Modified []string `json:"modified"`
	} `json:"commits"`
}

// ParsePush interpreta el cuerpo de un evento "push". Devuelve ok=false si
// el push no debe desplegar nada (borrado de rama, tags).
func ParsePush(body []byte) (push Push, ok bool, err error) {
	var p pushPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return Push{}, false, fmt.Errorf("payload de push inválido: %w", err)
	}
	branch, isBranch := strings.CutPrefix(p.Ref, "refs/heads/")
	if !isBranch || p.Deleted {
		return Push{}, false, nil
	}

	var files []string
	for _, c := range p.Commits {
		for _, group := range [][]string{c.Added, c.Removed, c.Modified} {
			for _, f := range group {
				if !slices.Contains(files, f) {
					files = append(files, f)
				}
			}
		}
	}
	slices.Sort(files)

	push = Push{
		Repository:     p.Repository.FullName,
		Branch:         branch,
		CommitSHA:      p.After,
		Pusher:         p.Pusher.Name,
		InstallationID: p.Installation.ID,
		ChangedFiles:   files,
		// Sin commits (push forzado de algo ya existente) o con la lista
		// truncada no podemos saber qué cambió.
		ChangedFilesComplete: len(p.Commits) > 0 && len(p.Commits) < maxCommitsInPayload,
	}
	if p.HeadCommit != nil {
		push.CommitMessage = p.HeadCommit.Message
	}
	return push, true, nil
}
