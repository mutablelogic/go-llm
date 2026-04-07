package httphandler_test

import (
	"bufio"
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
		ChatRequestCore: schema.ChatRequestCore{
			Session: session.ID,
			Text:    "hello from chat",
		},
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
	chatBody, _ := json.Marshal(schema.ChatRequest{ChatRequestCore: schema.ChatRequestCore{Session: session.ID, Text: "first"}})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatBody))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("first chat: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Second message
	chatBody, _ = json.Marshal(schema.ChatRequest{ChatRequestCore: schema.ChatRequestCore{Session: session.ID, Text: "second"}})
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

func TestChat_Stream_OK(t *testing.T) {
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

	// Send a streaming chat request
	chatBody, _ := json.Marshal(schema.ChatRequest{
		ChatRequestCore: schema.ChatRequestCore{
			Session: session.ID,
			Text:    "hello stream",
		},
	})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatBody))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "text/event-stream")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check content type
	ct := w.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Fatalf("expected Content-Type text/event-stream, got %q", ct)
	}

	// Parse SSE events from the response body
	type sseEvent struct {
		name string
		data string
	}
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

	// We expect at least one "result" event
	var found bool
	for _, evt := range events {
		if evt.name == schema.EventResult {
			found = true
			var resp schema.ChatResponse
			if err := json.Unmarshal([]byte(evt.data), &resp); err != nil {
				t.Fatalf("failed to decode result event: %v", err)
			}
			if resp.Role != schema.RoleAssistant {
				t.Fatalf("expected role=assistant, got %q", resp.Role)
			}
			if resp.Session != session.ID {
				t.Fatalf("expected session=%q, got %q", session.ID, resp.Session)
			}
			if len(resp.Content) == 0 || resp.Content[0].Text == nil {
				t.Fatal("expected at least one text content block")
			}
			if !strings.Contains(*resp.Content[0].Text, "hello stream") {
				t.Fatalf("expected response to contain 'hello stream', got %q", *resp.Content[0].Text)
			}
		}
	}
	if !found {
		t.Fatalf("no 'result' event found in SSE stream; events: %+v", events)
	}
}

func TestChat_Stream_InvalidSession(t *testing.T) {
	mgr := newGeneratorManager(t)
	mux := serveMux(mgr)

	chatBody := `{"session":"nonexistent","text":"hello"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewBufferString(chatBody))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "text/event-stream")
	mux.ServeHTTP(w, r)

	// Should still get SSE content type
	ct := w.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Fatalf("expected Content-Type text/event-stream, got %q", ct)
	}

	// Should contain an error event
	body := w.Body.String()
	if !strings.Contains(body, "event: "+schema.EventError) {
		t.Fatalf("expected error event in stream, got: %s", body)
	}
}

func TestChat_NoAcceptHeader_DefaultsJSON(t *testing.T) {
	mgr := newGeneratorManager(t)
	mux := serveMux(mgr)

	// Create a session
	sessionBody := `{"model":"test-model"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewBufferString(sessionBody))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create session: expected 201, got %d", w.Code)
	}
	var session schema.Session
	if err := json.NewDecoder(w.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}

	// Send chat without Accept header — should get JSON
	chatBody, _ := json.Marshal(schema.ChatRequest{ChatRequestCore: schema.ChatRequestCore{Session: session.ID, Text: "hello"}})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatBody))
	r.Header.Set("Content-Type", "application/json")
	// No Accept header
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Should be JSON, not SSE
	var resp schema.ChatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}
	if resp.Role != schema.RoleAssistant {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
}

func TestChat_UnsupportedAccept(t *testing.T) {
	mgr := newGeneratorManager(t)
	mux := serveMux(mgr)

	// Create a session
	sessionBody := `{"model":"test-model"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewBufferString(sessionBody))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create session: expected 201, got %d", w.Code)
	}
	var session schema.Session
	if err := json.NewDecoder(w.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}

	// Send chat with unsupported Accept header
	chatBody, _ := json.Marshal(schema.ChatRequest{ChatRequestCore: schema.ChatRequestCore{Session: session.ID, Text: "hello"}})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatBody))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "text/xml")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotAcceptable {
		t.Fatalf("expected 406, got %d: %s", w.Code, w.Body.String())
	}
}
