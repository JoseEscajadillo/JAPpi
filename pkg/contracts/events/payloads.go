package events

// Payloads de cada Type. Los campos se añaden, nunca se renombran ni se
// eliminan dentro de una misma Version (ver docs/events.md).

// RepoPushedPayload lo publica github-integration al recibir un push.
type RepoPushedPayload struct {
	Provider       string   `json:"provider"`   // "github"
	Repository     string   `json:"repository"` // "owner/name"
	Branch         string   `json:"branch"`
	CommitSHA      string   `json:"commit_sha"`
	CommitMessage  string   `json:"commit_message,omitempty"`
	Pusher         string   `json:"pusher,omitempty"`
	InstallationID int64    `json:"installation_id,omitempty"`
	ChangedFiles   []string `json:"changed_files,omitempty"`
	// ChangedFilesComplete es false cuando el proveedor truncó la lista de
	// commits; entonces hay que redesplegar todos los servicios.
	ChangedFilesComplete bool `json:"changed_files_complete"`
}

// BuildRequestedPayload lo publica control-plane por cada servicio afectado.
type BuildRequestedPayload struct {
	DeploymentID string `json:"deployment_id"`
	ServiceID    string `json:"service_id"`
	Repository   string `json:"repository"`
	CommitSHA    string `json:"commit_sha"`
	// ServicePath es el directorio del servicio dentro del repo ("." o "apps/web").
	ServicePath string `json:"service_path"`
	// BuildEnv son las variables que deben existir durante el build
	// (NEXT_PUBLIC_*, VITE_*...). Nunca contiene secretos.
	BuildEnv map[string]string `json:"build_env,omitempty"`
}

// BuildSucceededPayload lo publica builder al subir la imagen al registry.
type BuildSucceededPayload struct {
	DeploymentID string `json:"deployment_id"`
	ServiceID    string `json:"service_id"`
	Image        string `json:"image"` // referencia inmutable: registry/repo@sha256:...
}

// BuildFailedPayload lo publica builder cuando el build no termina.
type BuildFailedPayload struct {
	DeploymentID string `json:"deployment_id"`
	ServiceID    string `json:"service_id"`
	Reason       string `json:"reason"`
}

// DeploymentStatusChangedPayload lo publica deployer en cada transición.
type DeploymentStatusChangedPayload struct {
	DeploymentID string `json:"deployment_id"`
	ServiceID    string `json:"service_id"`
	Status       string `json:"status"`
	Detail       string `json:"detail,omitempty"`
}

// RollbackRequestedPayload lo publica control-plane cuando el usuario pide volver atrás.
type RollbackRequestedPayload struct {
	ServiceID          string `json:"service_id"`
	TargetDeploymentID string `json:"target_deployment_id"`
	RequestedBy        string `json:"requested_by"`
}

// SubscriptionChangedPayload lo publica billing al procesar un webhook de Stripe.
type SubscriptionChangedPayload struct {
	AccountID string `json:"account_id"`
	Plan      string `json:"plan"`   // trial, hobby, pro, team
	Status    string `json:"status"` // trialing, active, past_due, canceled
}
