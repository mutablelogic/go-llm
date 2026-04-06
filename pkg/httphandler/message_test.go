package httphandler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func TestSessionMessageListIntegration(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := SessionMessageHandler(manager)
	session := createChatTestSession(t, manager, conn.Config.Name, modelName)

	if err := manager.PoolConn.Insert(newModelHandlerTestContext(t), nil, schema.MessageInsert{
		Session: session.ID,
		Message: schema.Message{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: types.Ptr("hello world")}}, Tokens: 2},
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.PoolConn.Insert(newModelHandlerTestContext(t), nil, schema.MessageInsert{
		Session: session.ID,
		Message: schema.Message{Role: schema.RoleAssistant, Content: []schema.ContentBlock{{Text: types.Ptr("daily news summary")}}, Tokens: 4, Result: schema.ResultStop},
	}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/session/"+session.ID.String()+"/message?role=assistant&text=news", nil).WithContext(newModelHandlerTestContext(t))
	r.SetPathValue("session", session.ID.String())
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.MessageList
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 {
		t.Fatalf("expected count=1, got %d", resp.Count)
	}
	if len(resp.Body) != 1 {
		t.Fatalf("expected 1 message, got %d", len(resp.Body))
	}
	if resp.Body[0].Role != schema.RoleAssistant {
		t.Fatalf("expected role %q, got %q", schema.RoleAssistant, resp.Body[0].Role)
	}
	if got := resp.Body[0].Text(); got != "daily news summary" {
		t.Fatalf("expected assistant text %q, got %q", "daily news summary", got)
	}
}

func TestSessionMessageListSessionNotFound(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := SessionMessageHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/session/00000000-0000-0000-0000-000000000001/message", nil).WithContext(newModelHandlerTestContext(t))
	r.SetPathValue("session", "00000000-0000-0000-0000-000000000001")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSessionMessageListInvalidSession(t *testing.T) {
	_, _, item := SessionMessageHandler(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/session/not-a-uuid/message", nil)
	r.SetPathValue("session", "not-a-uuid")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSessionMessageListInvalidQuery(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := SessionMessageHandler(manager)
	session := createChatTestSession(t, manager, modelHandlerConn.Config.Name, requireDownloadModel(t))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/session/"+session.ID.String()+"/message?limit=invalid", nil).WithContext(newModelHandlerTestContext(t))
	r.SetPathValue("session", session.ID.String())
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
