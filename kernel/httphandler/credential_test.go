package httphandler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func TestCredentialCreateJSONIntegration(t *testing.T) {
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := CredentialHandler(manager)

	if err := manager.Exec(newModelHandlerTestContext(t), `TRUNCATE llm.credential CASCADE`); err != nil {
		t.Fatal(err)
	}

	user := llmtest.User(conn)
	userID := user.UUID()
	body, err := json.Marshal(schema.CredentialInsert{
		CredentialKey: schema.CredentialKey{
			URL:  "HTTPS://Example.COM/sse?token=abc#frag",
			User: &userID,
		},
		Credentials: []byte("secret"),
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/credential", bytes.NewReader(body))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	raw := w.Body.String()
	if strings.Contains(raw, `"credentials"`) {
		t.Fatalf("expected response to omit credentials: %s", raw)
	}
	if strings.Contains(raw, `"pv"`) {
		t.Fatalf("expected response to omit pv: %s", raw)
	}

	var resp schema.Credential
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.URL != "https://example.com/sse" {
		t.Fatalf("expected canonical URL, got %q", resp.URL)
	}
	if resp.User == nil || *resp.User != userID {
		t.Fatalf("expected user %s, got %v", userID, resp.User)
	}
	if resp.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}
}
