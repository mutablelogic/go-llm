package httphandler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func TestAskJSONIntegration(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := AskHandler(manager)

	body, err := json.Marshal(schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			GeneratorMeta: schema.GeneratorMeta{
				Provider: conn.Config.Name,
				Model:    modelName,
			},
			Text: "Say hello in exactly three words.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.AskResponse
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

func TestAskStreamIntegration(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := AskHandler(manager)

	body, err := json.Marshal(schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			GeneratorMeta: schema.GeneratorMeta{
				Provider: conn.Config.Name,
				Model:    modelName,
			},
			Text: "Say hello in exactly three words.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
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
		var resp schema.AskResponse
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

func TestAskModelNotFound(t *testing.T) {
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := AskHandler(manager)

	body, err := json.Marshal(schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			GeneratorMeta: schema.GeneratorMeta{
				Provider: conn.Config.Name,
				Model:    "missing-model",
			},
			Text: "hello",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAskInvalidJSON(t *testing.T) {
	_, _, item := AskHandler(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewBufferString(`{invalid`))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAskNotAcceptable(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := AskHandler(manager)

	body, err := json.Marshal(schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			GeneratorMeta: schema.GeneratorMeta{
				Provider: conn.Config.Name,
				Model:    modelName,
			},
			Text: "hello",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.Header.Set(types.ContentAcceptHeader, types.ContentTypeTextPlain)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotAcceptable {
		t.Fatalf("expected 406, got %d: %s", w.Code, w.Body.String())
	}
}
