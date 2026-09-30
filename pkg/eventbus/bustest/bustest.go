// Package bustest es la suite de contrato que toda implementación del bus
// debe pasar (principio de sustitución de Liskov): un servicio no puede notar
// si por debajo hay memoria o NATS.
package bustest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
)

// Bus es lo mínimo que la suite necesita de una implementación.
type Bus interface {
	Publish(ctx context.Context, e events.Envelope) error
	Subscribe(ctx context.Context, consumer string, types []events.Type, h events.Handler) error
}

const wait = 5 * time.Second

// Run ejecuta la suite. newBus debe devolver un bus limpio en cada llamada.
func Run(t *testing.T, newBus func(t *testing.T) Bus) {
	t.Run("entrega el payload intacto", func(t *testing.T) {
		bus := newBus(t)
		rec := subscribe(t, bus, uniq("c"), events.RepoPushed)
		e := mustEvent(t, events.RepoPushed, events.RepoPushedPayload{Repository: "acme/shop", Branch: "main"})
		publish(t, bus, e)

		got := rec.waitFor(t, e.ID)
		var p events.RepoPushedPayload
		if err := got.Decode(&p); err != nil {
			t.Fatal(err)
		}
		if p.Repository != "acme/shop" || p.Branch != "main" {
			t.Fatalf("payload alterado: %+v", p)
		}
	})

	t.Run("filtra por tipo", func(t *testing.T) {
		bus := newBus(t)
		rec := subscribe(t, bus, uniq("c"), events.BuildFailed)
		other := mustEvent(t, events.BuildSucceeded, events.BuildSucceededPayload{})
		mine := mustEvent(t, events.BuildFailed, events.BuildFailedPayload{})
		publish(t, bus, other)
		publish(t, bus, mine)

		rec.waitFor(t, mine.ID)
		if rec.has(other.ID) {
			t.Fatal("recibió un evento de un tipo al que no está suscrito")
		}
	})

	t.Run("descarta IDs duplicados", func(t *testing.T) {
		bus := newBus(t)
		rec := subscribe(t, bus, uniq("c"), events.RepoPushed)
		e := mustEvent(t, events.RepoPushed, events.RepoPushedPayload{})
		publish(t, bus, e)
		publish(t, bus, e)

		rec.waitFor(t, e.ID)
		time.Sleep(200 * time.Millisecond) // margen para una segunda entrega indebida
		if n := rec.count(e.ID); n != 1 {
			t.Fatalf("entregado %d veces, se esperaba 1", n)
		}
	})

	t.Run("cada consumidor recibe su copia", func(t *testing.T) {
		bus := newBus(t)
		a := subscribe(t, bus, uniq("a"), events.RepoPushed)
		b := subscribe(t, bus, uniq("b"), events.RepoPushed)
		e := mustEvent(t, events.RepoPushed, events.RepoPushedPayload{})
		publish(t, bus, e)

		a.waitFor(t, e.ID)
		b.waitFor(t, e.ID)
	})
}

type recorder struct {
	mu     sync.Mutex
	got    map[string]events.Envelope
	counts map[string]int
	notify chan struct{}
}

func subscribe(t *testing.T, bus Bus, consumer string, typ events.Type) *recorder {
	t.Helper()
	r := &recorder{got: map[string]events.Envelope{}, counts: map[string]int{}, notify: make(chan struct{}, 100)}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	err := bus.Subscribe(ctx, consumer, []events.Type{typ}, func(_ context.Context, e events.Envelope) error {
		r.mu.Lock()
		r.got[e.ID] = e
		r.counts[e.ID]++
		r.mu.Unlock()
		select {
		case r.notify <- struct{}{}:
		default:
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *recorder) has(id string) bool { return r.count(id) > 0 }

func (r *recorder) count(id string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counts[id]
}

func (r *recorder) waitFor(t *testing.T, id string) events.Envelope {
	t.Helper()
	deadline := time.After(wait)
	for {
		r.mu.Lock()
		e, ok := r.got[id]
		r.mu.Unlock()
		if ok {
			return e
		}
		select {
		case <-r.notify:
		case <-time.After(50 * time.Millisecond):
		case <-deadline:
			t.Fatalf("el evento %s no llegó en %s", id, wait)
		}
	}
}

func mustEvent(t *testing.T, typ events.Type, payload any) events.Envelope {
	t.Helper()
	e, err := events.New(typ, "", payload)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func publish(t *testing.T, bus Bus, e events.Envelope) {
	t.Helper()
	if err := bus.Publish(context.Background(), e); err != nil {
		t.Fatal(err)
	}
}

func uniq(prefix string) string {
	e, _ := events.New(events.RepoPushed, "", nil)
	return prefix + "-" + e.ID[:8]
}
