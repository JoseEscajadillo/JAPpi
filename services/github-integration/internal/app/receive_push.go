// Package app contiene los casos de uso de github-integration.
package app

import (
	"context"
	"fmt"

	"github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
	"github.com/JoseEscajadillo/JAPpi/services/github-integration/internal/domain"
)

// EventPublisher publica eventos en el bus.
type EventPublisher interface {
	Publish(ctx context.Context, e events.Envelope) error
}

// ReceivePush publica un RepoPushed por cada push válido.
type ReceivePush struct {
	Events EventPublisher
}

// Execute usa deliveryID (X-GitHub-Delivery) como ID del evento: si GitHub
// reenvía el webhook, el bus descarta el duplicado.
func (uc ReceivePush) Execute(ctx context.Context, deliveryID string, p domain.Push) error {
	e, err := events.NewWithID("github-"+deliveryID, events.RepoPushed, "", events.RepoPushedPayload{
		Provider:             "github",
		Repository:           p.Repository,
		Branch:               p.Branch,
		CommitSHA:            p.CommitSHA,
		CommitMessage:        p.CommitMessage,
		Pusher:               p.Pusher,
		InstallationID:       p.InstallationID,
		ChangedFiles:         p.ChangedFiles,
		ChangedFilesComplete: p.ChangedFilesComplete,
	})
	if err != nil {
		return err
	}
	if err := uc.Events.Publish(ctx, e); err != nil {
		return fmt.Errorf("publicar push de %s: %w", p.Repository, err)
	}
	return nil
}
