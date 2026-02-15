package httphandler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

func TestChat_OK(t *testing.T) {
	mgr := newGeneratorManager(t)
	mux := serveMux(mgr)

	// Create a session first
	sessionBody := `{"model":"test-model","name":"test-chat"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewBufferString(sessionBody))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("create session: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var session schema.Session
	if err := json.NewDecoder(w.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}

	// Send a chat message
	chatBody, _ := json.Marshal(schema.ChatRequest{
		Session: session.ID,
		Text:    "hello from chat",
	})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatBody))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ChatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Role != "assistant" {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
	if resp.Session != session.ID {
		t.Fatalf("expected session=%q, got %q", session.ID, resp.Session)
	}
	if len(resp.Content) == 0 || resp.Content[0].Text == nil {
		t.Fatal("expected at least one text content block")
	}
	if !strings.Contains(*resp.Content[0].Text, "hello from chat") {
		t.Fatalf("expected response to contain 'hello from chat', got %q", *resp.Content[0].Text)
	}
}

func TestChat_InvalidSession(t *testing.T) {
	mgr := newGeneratorManager(t)
	mux := serveMux(mgr)

	chatBody := `{"session":"nonexistent-id","text":"hello"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewBufferString(chatBody))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code == http.StatusOK {
		t.Fatal("expected error status for invalid session")
	}
}

func TestChat_MethodNotAllowed(t *testing.T) {
	mgr := newGeneratorManager(t)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/chat", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestChat_MultiTurn(t *testing.T) {
	mgr := newGeneratorManager(t)
	mux := serveMux(mgr)

	// Create a session
	sessionBody := `{"model":"test-model"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewBufferString(sessionBody))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("create session: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var session schema.Session
	if err := json.NewDecoder(w.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}

	// First message
	chatBody, _ := json.Marshal(schema.ChatRequest{Session: session.ID, Text: "first"})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatBody))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("first chat: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Second message
	chatBody, _ = json.Marshal(schema.ChatRequest{Session: session.ID, Text: "second"})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatBody))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("second chat: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ChatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(*resp.Content[0].Text, "second") {
		t.Fatalf("expected response to contain 'second', got %q", *resp.Content[0].Text)
	}
}
