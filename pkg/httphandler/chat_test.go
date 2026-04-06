package httphandler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func TestChatJSONIntegration(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ChatHandler(manager)
	session := createChatTestSession(t, manager, conn.Config.Name, modelName)

	body, err := json.Marshal(schema.ChatRequest{
		Session: session.ID,
		Text:    "Say hello in exactly three words.",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ChatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Role != schema.RoleAssistant {
		t.Fatalf("expected role %q, got %q", schema.RoleAssistant, resp.Role)
	}
	if len(resp.Content) == 0 {
		t.Fatal("expected response content")
	}
	text := resp.Content[0].Text
	if text == nil || strings.TrimSpace(*text) == "" {
		t.Fatalf("expected assistant text, got %+v", resp.Content)
	}
}

func TestChatStreamIntegration(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ChatHandler(manager)
	session := createChatTestSession(t, manager, conn.Config.Name, modelName)

	body, err := json.Marshal(schema.ChatRequest{
		Session: session.ID,
		Text:    "Say hello in exactly three words.",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.Header.Set(types.ContentAcceptHeader, types.ContentTypeTextStream)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get(types.ContentTypeHeader); ct != types.ContentTypeTextStream {
		t.Fatalf("expected content type %q, got %q", types.ContentTypeTextStream, ct)
	}

	type sseEvent struct{ name, data string }
	var events []sseEvent
	scanner := bufio.NewScanner(w.Body)
	var current sseEvent
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			current.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			current.data = strings.TrimPrefix(line, "data: ")
		case line == "":
			if current.name != "" || current.data != "" {
				events = append(events, current)
				current = sseEvent{}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	var sawResult bool
	for _, event := range events {
		if event.name != schema.EventResult {
			continue
		}
		sawResult = true
		var resp schema.ChatResponse
		if err := json.Unmarshal([]byte(event.data), &resp); err != nil {
			t.Fatalf("decode result event: %v", err)
		}
		if resp.Role != schema.RoleAssistant {
			t.Fatalf("expected role %q, got %q", schema.RoleAssistant, resp.Role)
		}
	}
	if !sawResult {
		t.Fatalf("expected result event, got %+v", events)
	}
}

func TestChatSessionNotFound(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ChatHandler(manager)

	body, err := json.Marshal(schema.ChatRequest{
		Session: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Text:    "hello",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChatInvalidJSON(t *testing.T) {
	_, _, item := ChatHandler(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewBufferString(`{invalid`))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChatNotAcceptable(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ChatHandler(manager)
	session := createChatTestSession(t, manager, conn.Config.Name, modelName)

	body, err := json.Marshal(schema.ChatRequest{
		Session: session.ID,
		Text:    "hello",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.Header.Set(types.ContentAcceptHeader, types.ContentTypeTextPlain)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotAcceptable {
		t.Fatalf("expected 406, got %d: %s", w.Code, w.Body.String())
	}
}

func createChatTestSession(t *testing.T, manager interface {
	CreateSession(ctx context.Context, req schema.SessionInsert, user *auth.User) (*schema.Session, error)
}, provider, model string) *schema.Session {
	t.Helper()

	session, err := manager.CreateSession(newModelHandlerTestContext(t), schema.SessionInsert{
		SessionMeta: schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{
				Provider: types.Ptr(provider),
				Model:    types.Ptr(model),
			},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	return session
}
