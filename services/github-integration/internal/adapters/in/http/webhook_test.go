package http_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JoseEscajadillo/JAPpi/pkg/contracts/events"
	busmem "github.com/JoseEscajadillo/JAPpi/pkg/eventbus/memory"
	webhook "github.com/JoseEscajadillo/JAPpi/services/github-integration/internal/adapters/in/http"
	"github.com/JoseEscajadillo/JAPpi/services/github-integration/internal/app"
)

const secret = "s3cr3t"

const pushBody = `{
  "ref": "refs/heads/main",
  "after": "abc123",
  "repository": {"full_name": "acme/shop"},
  "installation": {"id": 42},
  "pusher": {"name": "ana"},
  "head_commit": {"message": "feat: carrito"},
  "commits": [
    {"added": ["apps/web/cart.tsx"], "removed": [], "modified": ["apps/web/app.tsx"]},
    {"added": [], "removed": ["apps/web/old.tsx"], "modified": ["apps/web/app.tsx"]}
  ]
}`

func sign(body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func setup(t *testing.T) (http.Handler, *[]events.RepoPushedPayload) {
	t.Helper()
	bus := busmem.New()
	var got []events.RepoPushedPayload
	_ = bus.Subscribe(context.Background(), "test", []events.Type{events.RepoPushed}, func(_ context.Context, e events.Envelope) error {
		var p events.RepoPushedPayload
		_ = e.Decode(&p)
		got = append(got, p)
		return nil
	})
	h := webhook.NewHandler(webhook.Webhook{Secret: []byte(secret), ReceivePush: app.ReceivePush{Events: bus}})
	return h, &got
}

func send(h http.Handler, event, delivery, body, signature string) int {
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-GitHub-Delivery", delivery)
	req.Header.Set("X-Hub-Signature-256", signature)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestValidPushPublishesEvent(t *testing.T) {
	h, got := setup(t)
	if code := send(h, "push", "d-1", pushBody, sign(pushBody)); code != http.StatusAccepted {
		t.Fatalf("código %d", code)
	}
	if len(*got) != 1 {
		t.Fatalf("esperaba 1 evento, hubo %d", len(*got))
	}
	p := (*got)[0]
	want := []string{"apps/web/app.tsx", "apps/web/cart.tsx", "apps/web/old.tsx"}
	if p.Repository != "acme/shop" || p.Branch != "main" || p.CommitSHA != "abc123" || !p.ChangedFilesComplete {
		t.Errorf("payload incorrecto: %+v", p)
	}
	if strings.Join(p.ChangedFiles, ",") != strings.Join(want, ",") {
		t.Errorf("archivos = %v, quería %v", p.ChangedFiles, want)
	}
}

func TestBadSignatureIsRejected(t *testing.T) {
	h, got := setup(t)
	for _, sig := range []string{"", "sha256=00", sign(pushBody + " ")} {
		if code := send(h, "push", "d-1", pushBody, sig); code != http.StatusUnauthorized {
			t.Errorf("firma %q: código %d, quería 401", sig, code)
		}
	}
	if len(*got) != 0 {
		t.Fatal("un webhook sin firma válida no puede publicar eventos")
	}
}

func TestRedeliveryIsDeduplicated(t *testing.T) {
	h, got := setup(t)
	send(h, "push", "d-7", pushBody, sign(pushBody))
	send(h, "push", "d-7", pushBody, sign(pushBody))
	if len(*got) != 1 {
		t.Fatalf("la reentrega de GitHub generó %d eventos", len(*got))
	}
}

func TestTagsAndDeletedBranchesAreIgnored(t *testing.T) {
	h, got := setup(t)
	for _, body := range []string{
		`{"ref":"refs/tags/v1.0.0","after":"x","repository":{"full_name":"acme/shop"}}`,
		`{"ref":"refs/heads/old","deleted":true,"after":"0000","repository":{"full_name":"acme/shop"}}`,
	} {
		if code := send(h, "push", "d-"+body[10:14], body, sign(body)); code != http.StatusAccepted {
			t.Errorf("código %d", code)
		}
	}
	if len(*got) != 0 {
		t.Fatalf("no debía desplegar nada: %+v", *got)
	}
}
