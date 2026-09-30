package memory_test

import (
	"testing"

	"github.com/JoseEscajadillo/JAPpi/pkg/eventbus/bustest"
	"github.com/JoseEscajadillo/JAPpi/pkg/eventbus/memory"
)

func TestContract(t *testing.T) {
	bustest.Run(t, func(*testing.T) bustest.Bus { return memory.New() })
}
