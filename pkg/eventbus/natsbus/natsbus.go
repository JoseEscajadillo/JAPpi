// Package natsbus implementa el bus de eventos sobre NATS JetStream.
//
// Todos los eventos viven en el stream JAPPI (subjects "jappi.>"). Cada
// servicio crea un consumidor durable con su nombre, de modo que si el
// servicio se cae, al volver retoma donde lo dejó. El ID del sobre viaja en
// la cabecera Nats-Msg-Id y JetStream descarta duplicados dentro de la
// ventana Duplicates.
package natsbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
)

const (
	streamName = "JAPPI"
	ackWait    = 30 * time.Second
	// heartbeat avisa a JetStream de que un handler largo (un build) sigue
	// vivo, para que no reentregue el mensaje a otra réplica.
	heartbeat  = 10 * time.Second
	maxDeliver = 5
	retryDelay = 10 * time.Second
)

// Bus implementa Publish y Subscribe sobre JetStream.
type Bus struct {
	nc     *nats.Conn
	js     jetstream.JetStream
	stream jetstream.Stream
}

// Connect abre la conexión y crea (o actualiza) el stream.
func Connect(ctx context.Context, url, clientName string) (*Bus, error) {
	nc, err := nats.Connect(url, nats.Name(clientName), nats.MaxReconnects(-1))
	if err != nil {
		return nil, fmt.Errorf("natsbus: conectar a %s: %w", url, err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("natsbus: jetstream: %w", err)
	}
	stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       streamName,
		Subjects:   []string{events.SubjectPrefix + ">"},
		Storage:    jetstream.FileStorage,
		Retention:  jetstream.LimitsPolicy,
		MaxAge:     7 * 24 * time.Hour,
		Duplicates: 10 * time.Minute,
	})
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("natsbus: crear stream %s: %w", streamName, err)
	}
	return &Bus{nc: nc, js: js, stream: stream}, nil
}

// Publish publica e y espera la confirmación de JetStream.
func (b *Bus) Publish(ctx context.Context, e events.Envelope) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("natsbus: serializar %s: %w", e.ID, err)
	}
	if _, err := b.js.Publish(ctx, e.Type.Subject(), data, jetstream.WithMsgID(e.ID)); err != nil {
		return fmt.Errorf("natsbus: publicar %s: %w", e.Type, err)
	}
	return nil
}

// Subscribe crea el consumidor durable y procesa mensajes hasta que ctx se cancele.
func (b *Bus) Subscribe(ctx context.Context, consumer string, types []events.Type, h events.Handler) error {
	subjects := make([]string, len(types))
	for i, t := range types {
		subjects[i] = t.Subject()
	}
	cons, err := b.stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:        consumer,
		FilterSubjects: subjects,
		AckPolicy:      jetstream.AckExplicitPolicy,
		AckWait:        ackWait,
		MaxDeliver:     maxDeliver,
	})
	if err != nil {
		return fmt.Errorf("natsbus: consumidor %s: %w", consumer, err)
	}
	cc, err := cons.Consume(func(msg jetstream.Msg) { b.handle(ctx, consumer, msg, h) })
	if err != nil {
		return fmt.Errorf("natsbus: consumir %s: %w", consumer, err)
	}
	go func() {
		<-ctx.Done()
		cc.Stop()
	}()
	return nil
}

func (b *Bus) handle(ctx context.Context, consumer string, msg jetstream.Msg, h events.Handler) {
	var e events.Envelope
	if err := json.Unmarshal(msg.Data(), &e); err != nil {
		slog.Error("mensaje ilegible, descartado", "consumer", consumer, "subject", msg.Subject(), "err", err)
		_ = msg.Term()
		return
	}

	done := make(chan struct{})
	go func() {
		t := time.NewTicker(heartbeat)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				_ = msg.InProgress()
			}
		}
	}()
	err := h(ctx, e)
	close(done)

	switch {
	case err == nil:
		_ = msg.Ack()
	case errors.Is(err, events.ErrPermanent):
		slog.Error("evento descartado", "consumer", consumer, "event", e.Type, "id", e.ID, "err", err)
		_ = msg.Term()
	default:
		slog.Warn("evento se reintentará", "consumer", consumer, "event", e.Type, "id", e.ID, "err", err)
		_ = msg.NakWithDelay(retryDelay)
	}
}

// Close cierra la conexión esperando a que se vacíen las publicaciones pendientes.
func (b *Bus) Close() error { return b.nc.Drain() }
