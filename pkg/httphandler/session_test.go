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
		body, _ := json.Marshal(schema.SessionMeta{Name: name, Model: "model-a"})
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
		body, _ := json.Marshal(schema.SessionMeta{Name: name, Model: "model-a"})
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

	body, _ := json.Marshal(schema.SessionMeta{Name: "my chat", Model: "gpt-4"})
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

	body, _ := json.Marshal(schema.SessionMeta{Name: "test", Provider: "provider-2", Model: "model-x"})
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

	body, _ := json.Marshal(schema.SessionMeta{Name: "test", Model: "nonexistent"})
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
	body, _ := json.Marshal(schema.SessionMeta{Name: "my chat", Model: "gpt-4"})
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
	body, _ := json.Marshal(schema.SessionMeta{Name: "doomed", Model: "gpt-4"})
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
