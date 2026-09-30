// Package events define los contratos del bus de eventos de JAPpi.
//
// Cada servicio se comunica con los demás publicando y consumiendo eventos;
// ninguno llama directamente a otro. Los tipos de este paquete son el
// "idioma común" del sistema: cambiarlos rompe a los consumidores, así que
// cualquier cambio incompatible sube Version.
package events

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Type identifica un evento. El subject de NATS es SubjectPrefix + Type.
type Type string

const (
	RepoPushed              Type = "repo.pushed"
	BuildRequested          Type = "build.requested"
	BuildSucceeded          Type = "build.succeeded"
	BuildFailed             Type = "build.failed"
	DeployRequested         Type = "deploy.requested"
	DeploymentStatusChanged Type = "deployment.status_changed"
	SubscriptionChanged     Type = "subscription.changed"
)

// SubjectPrefix agrupa todos los eventos de la plataforma en un solo stream.
const SubjectPrefix = "jappi."

// Subject devuelve el subject de NATS donde se publica este tipo de evento.
func (t Type) Subject() string { return SubjectPrefix + string(t) }

// Envelope es el sobre común de todos los eventos. Data lleva el payload
// concreto (ver payloads.go) serializado en JSON.
type Envelope struct {
	// ID único del evento. El bus lo usa para descartar duplicados, así que
	// quien reintenta una publicación debe reutilizar el mismo ID.
	ID         string          `json:"id"`
	Type       Type            `json:"type"`
	Version    int             `json:"version"`
	OccurredAt time.Time       `json:"occurred_at"`
	ProjectID  string          `json:"project_id,omitempty"`
	Data       json.RawMessage `json:"data"`
}

// New crea un evento con un ID aleatorio.
func New(t Type, projectID string, payload any) (Envelope, error) {
	return NewWithID(randomID(), t, projectID, payload)
}

// NewWithID crea un evento con un ID elegido por quien publica. Úsalo cuando
// el origen ya trae un identificador estable (p. ej. X-GitHub-Delivery), para
// que las reentregas del origen no generen trabajo duplicado.
func NewWithID(id string, t Type, projectID string, payload any) (Envelope, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("events: serializar payload de %s: %w", t, err)
	}
	return Envelope{
		ID:         id,
		Type:       t,
		Version:    1,
		OccurredAt: time.Now().UTC(),
		ProjectID:  projectID,
		Data:       data,
	}, nil
}

// Decode deserializa el payload en v.
func (e Envelope) Decode(v any) error {
	if err := json.Unmarshal(e.Data, v); err != nil {
		return fmt.Errorf("events: decodificar %s (%s): %w", e.Type, e.ID, err)
	}
	return nil
}

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand no falla en plataformas soportadas
	}
	return hex.EncodeToString(b[:])
}

// ErrPermanent marca un error que no se arregla reintentando (payload
// inválido, proyecto borrado...). El bus descarta el mensaje en vez de
// reentregarlo.
var ErrPermanent = errors.New("error permanente")

// Permanent envuelve err para que el bus no reintente el evento.
func Permanent(err error) error { return fmt.Errorf("%w: %w", ErrPermanent, err) }
