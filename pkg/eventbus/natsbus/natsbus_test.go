package natsbus_test

import (
	"context"
	"os"
	"testing"

	"github.com/JoseEscajadillo/JAPpi/pkg/eventbus/bustest"
	"github.com/JoseEscajadillo/JAPpi/pkg/eventbus/natsbus"
)

// Necesita un NATS con JetStream: docker compose -f deploy/docker-compose.dev.yml up -d
// y luego JAPPI_TEST_NATS_URL=nats://localhost:4222 go test ./pkg/eventbus/...
func TestContract(t *testing.T) {
	url := os.Getenv("JAPPI_TEST_NATS_URL")
	if url == "" {
		t.Skip("JAPPI_TEST_NATS_URL no definida")
	}
	bustest.Run(t, func(t *testing.T) bustest.Bus {
		b, err := natsbus.Connect(context.Background(), url, "jappi-test")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = b.Close() })
		return b
	})
}
