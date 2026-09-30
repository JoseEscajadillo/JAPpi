package githubintegration_test

import (
	"testing"

	"github.com/JoseEscajadillo/JAPpi/pkg/archtest"
)

func TestHexagonalRules(t *testing.T) {
	archtest.CheckHexagon(t, ".", "github.com/JoseEscajadillo/JAPpi/services/github-integration")
}
