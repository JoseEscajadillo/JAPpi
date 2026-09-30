// Package memory es un bus de eventos en memoria para pruebas y desarrollo
// local. Imita la semántica de natsbus: cada consumidor (durable) recibe una
// vez cada evento de los tipos que filtra, y los IDs repetidos se descartan.
//
// La entrega es síncrona para que las pruebas sean deterministas; no hay
// reintentos: los errores de los handlers se pasan a OnError.
package memory

import (
	"context"
	"log/slog"
	"slices"
	"sync"

	"github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
)

type subscription struct {
	consumer string
	types    []events.Type
	handler  events.Handler
	ctx      context.Context
}

// Bus implementa Publish y Subscribe en memoria.
type Bus struct {
	mu   sync.Mutex
	seen map[string]bool
	subs []subscription

	// OnError recibe los errores de los handlers. Por defecto, los registra.
	OnError func(consumer string, e events.Envelope, err error)
}

// New crea un bus vacío.
func New() *Bus {
	return &Bus{
		seen: make(map[string]bool),
		OnError: func(consumer string, e events.Envelope, err error) {
			slog.Error("handler falló", "consumer", consumer, "event", e.Type, "id", e.ID, "err", err)
		},
	}
}

// Publish entrega e a todos los consumidores suscritos a su tipo.
func (b *Bus) Publish(ctx context.Context, e events.Envelope) error {
	b.mu.Lock()
	if b.seen[e.ID] {
		b.mu.Unlock()
		return nil
	}
	b.seen[e.ID] = true
	// Copiamos la lista para no mantener el candado mientras corren los
	// handlers: un handler puede publicar a su vez.
	subs := slices.Clone(b.subs)
	b.mu.Unlock()

	for _, s := range subs {
		if s.ctx.Err() != nil || !slices.Contains(s.types, e.Type) {
			continue
		}
		if err := s.handler(ctx, e); err != nil {
			b.OnError(s.consumer, e, err)
		}
	}
	return nil
}

// Subscribe registra h hasta que ctx se cancele.
func (b *Bus) Subscribe(ctx context.Context, consumer string, types []events.Type, h events.Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = append(b.subs, subscription{consumer: consumer, types: types, handler: h, ctx: ctx})
	return nil
}
