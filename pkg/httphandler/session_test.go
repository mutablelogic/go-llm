package httphandler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

func TestSessionList_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/session", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ListSessionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 0 {
		t.Fatalf("expected count=0, got %d", resp.Count)
	}
}

func TestSessionList_WithSessions(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	for _, name := range []string{"session-1", "session-2"} {
		body, _ := json.Marshal(schema.SessionMeta{Name: name, GeneratorMeta: schema.GeneratorMeta{Model: "model-a"}})
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s: expected 201, got %d: %s", name, w.Code, w.Body.String())
		}
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/session", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ListSessionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected count=2, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(resp.Body))
	}
}

func TestSessionList_WithPagination(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	for _, name := range []string{"a", "b", "c"} {
		body, _ := json.Marshal(schema.SessionMeta{Name: name, GeneratorMeta: schema.GeneratorMeta{Model: "model-a"}})
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s: expected 201, got %d: %s", name, w.Code, w.Body.String())
		}
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/session?limit=2&offset=1", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ListSessionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected count=3, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 sessions in page, got %d", len(resp.Body))
	}
}

func TestSessionCreate_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.SessionMeta{Name: "my chat", GeneratorMeta: schema.GeneratorMeta{Model: "gpt-4"}})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var session schema.Session
	if err := json.NewDecoder(w.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if session.ID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if session.Name != "my chat" {
		t.Fatalf("expected name=my chat, got %q", session.Name)
	}
	if session.Model != "gpt-4" {
		t.Fatalf("expected model=gpt-4, got %q", session.Model)
	}
	if session.Provider != "provider-1" {
		t.Fatalf("expected provider=provider-1, got %q", session.Provider)
	}
}

func TestSessionCreate_WithProvider(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-x"}}},
		{name: "provider-2", models: []schema.Model{{Name: "model-x"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Provider: "provider-2", Model: "model-x"}})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var session schema.Session
	if err := json.NewDecoder(w.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if session.Provider != "provider-2" {
		t.Fatalf("expected provider=provider-2, got %q", session.Provider)
	}
}

func TestSessionCreate_ModelNotFound(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "nonexistent"}})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSessionCreate_InvalidJSON(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewBufferString("{invalid"))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSession_MethodNotAllowed(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/session", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

///////////////////////////////////////////////////////////////////////////////
// SESSION GET TESTS

func TestSessionGet_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	// Create a session first
	body, _ := json.Marshal(schema.SessionMeta{Name: "my chat", GeneratorMeta: schema.GeneratorMeta{Model: "gpt-4"}})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created schema.Session
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	// Get the session by ID
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/session/"+created.ID, nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var session schema.Session
	if err := json.NewDecoder(w.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if session.ID != created.ID {
		t.Fatalf("expected id=%s, got %s", created.ID, session.ID)
	}
	if session.Name != "my chat" {
		t.Fatalf("expected name='my chat', got %q", session.Name)
	}
}

func TestSessionGet_NotFound(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/session/nonexistent-id", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

///////////////////////////////////////////////////////////////////////////////
// SESSION DELETE TESTS

func TestSessionDelete_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	// Create a session
	body, _ := json.Marshal(schema.SessionMeta{Name: "doomed", GeneratorMeta: schema.GeneratorMeta{Model: "gpt-4"}})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created schema.Session
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	// Delete the session
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodDelete, "/session/"+created.ID, nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %s", w.Body.String())
	}

	// Verify it's gone
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/session/"+created.ID, nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSessionDelete_NotFound(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/session/nonexistent-id", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSessionGet_MethodNotAllowed(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/session/some-id", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

///////////////////////////////////////////////////////////////////////////////
// SESSION PATCH TESTS

func TestSessionPatch_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	// Create a session
	body, _ := json.Marshal(schema.SessionMeta{Name: "original", GeneratorMeta: schema.GeneratorMeta{Model: "gpt-4"}})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created schema.Session
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	// Patch the session name
	body, _ = json.Marshal(schema.SessionMeta{Name: "renamed"})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPatch, "/session/"+created.ID, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated schema.Session
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Name != "renamed" {
		t.Fatalf("expected name=renamed, got %q", updated.Name)
	}
	if updated.Model != "gpt-4" {
		t.Fatalf("expected model=gpt-4, got %q", updated.Model)
	}
}

func TestSessionPatch_SystemPrompt(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	// Create a session
	body, _ := json.Marshal(schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "gpt-4", SystemPrompt: "old"}})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	var created schema.Session
	json.NewDecoder(w.Body).Decode(&created)

	// Patch system prompt only
	body, _ = json.Marshal(schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{SystemPrompt: "new"}})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPatch, "/session/"+created.ID, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated schema.Session
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.SystemPrompt != "new" {
		t.Fatalf("expected system_prompt=new, got %q", updated.SystemPrompt)
	}
	if updated.Name != "test" {
		t.Fatalf("expected name=test preserved, got %q", updated.Name)
	}
}

func TestSessionPatch_NotFound(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.SessionMeta{Name: "x"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/session/nonexistent-id", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSessionPatch_InvalidJSON(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/session/some-id", bytes.NewBufferString("{invalid"))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

///////////////////////////////////////////////////////////////////////////////
// SESSION LABEL TESTS

func TestSessionCreate_WithLabels(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.SessionMeta{
		Name:          "labeled",
		GeneratorMeta: schema.GeneratorMeta{Model: "gpt-4"},
		Labels:        map[string]string{"chat-id": "12345", "ui": "telegram"},
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var session schema.Session
	if err := json.NewDecoder(w.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if session.Labels["chat-id"] != "12345" {
		t.Fatalf("expected chat-id=12345, got %q", session.Labels["chat-id"])
	}
	if session.Labels["ui"] != "telegram" {
		t.Fatalf("expected ui=telegram, got %q", session.Labels["ui"])
	}
}

func TestSessionCreate_InvalidLabelKey(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.SessionMeta{
		Name:          "bad",
		GeneratorMeta: schema.GeneratorMeta{Model: "gpt-4"},
		Labels:        map[string]string{"bad key!": "value"},
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code == http.StatusCreated {
		t.Fatal("expected error for invalid label key, got 201")
	}
}

func TestSessionList_WithLabelFilter(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	// Create sessions with different labels
	for _, tc := range []struct {
		name   string
		labels map[string]string
	}{
		{"telegram-chat", map[string]string{"ui": "telegram", "chat-id": "100"}},
		{"web-chat", map[string]string{"ui": "web"}},
		{"no-labels", nil},
	} {
		meta := schema.SessionMeta{
			Name:          tc.name,
			GeneratorMeta: schema.GeneratorMeta{Model: "gpt-4"},
			Labels:        tc.labels,
		}
		body, _ := json.Marshal(meta)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s: expected 201, got %d: %s", tc.name, w.Code, w.Body.String())
		}
	}

	// Filter by ui:telegram
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/session?label=ui:telegram", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp schema.ListSessionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Body) != 1 {
		t.Fatalf("expected 1 session, got %d", len(resp.Body))
	}
	if resp.Body[0].Name != "telegram-chat" {
		t.Fatalf("expected name=telegram-chat, got %q", resp.Body[0].Name)
	}

	// Filter by ui:web
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/session?label=ui:web", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = schema.ListSessionResponse{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Body) != 1 {
		t.Fatalf("expected 1 session, got %d", len(resp.Body))
	}
	if resp.Body[0].Name != "web-chat" {
		t.Fatalf("expected name=web-chat, got %q", resp.Body[0].Name)
	}

	// No filter returns all
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/session", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = schema.ListSessionResponse{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Body) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(resp.Body))
	}
}

func TestSessionList_WithMultipleLabelFilters(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	for _, tc := range []struct {
		name   string
		labels map[string]string
	}{
		{"match", map[string]string{"ui": "telegram", "chat-id": "100"}},
		{"partial", map[string]string{"ui": "telegram"}},
	} {
		body, _ := json.Marshal(schema.SessionMeta{
			Name:          tc.name,
			GeneratorMeta: schema.GeneratorMeta{Model: "gpt-4"},
			Labels:        tc.labels,
		})
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
	}

	// Both labels must match
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/session?label=ui:telegram&label=chat-id:100", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp schema.ListSessionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Body) != 1 {
		t.Fatalf("expected 1 session, got %d", len(resp.Body))
	}
	if resp.Body[0].Name != "match" {
		t.Fatalf("expected name=match, got %q", resp.Body[0].Name)
	}
}

func TestSessionPatch_Labels(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	// Create with initial labels
	body, _ := json.Marshal(schema.SessionMeta{
		Name:          "test",
		GeneratorMeta: schema.GeneratorMeta{Model: "gpt-4"},
		Labels:        map[string]string{"env": "prod", "team": "backend"},
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	var created schema.Session
	json.NewDecoder(w.Body).Decode(&created)

	// Patch: merge labels
	body, _ = json.Marshal(schema.SessionMeta{
		Labels: map[string]string{"team": "frontend", "region": "us"},
	})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPatch, "/session/"+created.ID, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated schema.Session
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Labels["env"] != "prod" {
		t.Fatalf("expected env=prod preserved, got %q", updated.Labels["env"])
	}
	if updated.Labels["team"] != "frontend" {
		t.Fatalf("expected team=frontend, got %q", updated.Labels["team"])
	}
	if updated.Labels["region"] != "us" {
		t.Fatalf("expected region=us, got %q", updated.Labels["region"])
	}
}

func TestSessionPatch_RemoveLabel(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	// Create with labels
	body, _ := json.Marshal(schema.SessionMeta{
		Name:          "test",
		GeneratorMeta: schema.GeneratorMeta{Model: "gpt-4"},
		Labels:        map[string]string{"env": "prod", "team": "backend"},
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	var created schema.Session
	json.NewDecoder(w.Body).Decode(&created)

	// Remove team label with empty value
	body, _ = json.Marshal(schema.SessionMeta{
		Labels: map[string]string{"team": ""},
	})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPatch, "/session/"+created.ID, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated schema.Session
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Labels["env"] != "prod" {
		t.Fatalf("expected env=prod preserved, got %q", updated.Labels["env"])
	}
	if _, exists := updated.Labels["team"]; exists {
		t.Fatal("expected team label to be removed")
	}
}
