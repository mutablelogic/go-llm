package httpclient_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func newChatTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ct := r.Header.Get("Content-Type")
		var text, session string
		var attachments int

		switch {
		case len(ct) >= 16 && ct[:16] == "application/json":
			var req schema.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			text = req.Text
			session = req.Session
			attachments = len(req.Attachments)
		case len(ct) >= 19 && ct[:19] == "multipart/form-data":
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			text = r.FormValue("text")
			session = r.FormValue("session")
			if _, _, err := r.FormFile("file"); err == nil {
				attachments = 1
			}
		default:
			http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
			return
		}

		if session == "" {
			http.Error(w, "session ID required", http.StatusBadRequest)
			return
		}

		respText := "echo: " + text
		if attachments > 0 {
			respText += " [with attachments]"
		}

		resp := schema.ChatResponse{
			CompletionResponse: schema.CompletionResponse{
				Role:    "assistant",
				Content: []schema.ContentBlock{{Text: types.Ptr(respText)}},
			},
			Session: session,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	return httptest.NewServer(mux)
}

func TestChat_NoAttachments(t *testing.T) {
	srv := newChatTestServer(t)
	defer srv.Close()
	c := newClient(t, srv.URL)

	resp, err := c.Chat(context.Background(), schema.ChatRequest{
		Session: "session-123",
		Text:    "hello world",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Role != "assistant" {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
	if resp.Session != "session-123" {
		t.Fatalf("expected session=session-123, got %q", resp.Session)
	}
	if len(resp.Content) == 0 || resp.Content[0].Text == nil {
		t.Fatal("expected content with text")
	}
	if got := *resp.Content[0].Text; got != "echo: hello world" {
		t.Fatalf("expected 'echo: hello world', got %q", got)
	}
}

func TestChat_WithFile(t *testing.T) {
	srv := newChatTestServer(t)
	defer srv.Close()
	c := newClient(t, srv.URL)

	fileData := bytes.NewReader([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	resp, err := c.Chat(context.Background(), schema.ChatRequest{
		Session: "session-123",
		Text:    "describe this",
	}, httpclient.WithChatFile("image.png", fileData))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Role != "assistant" {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
	if resp.Session != "session-123" {
		t.Fatalf("expected session=session-123, got %q", resp.Session)
	}
}

func TestChat_WithURL(t *testing.T) {
	srv := newChatTestServer(t)
	defer srv.Close()
	c := newClient(t, srv.URL)

	resp, err := c.Chat(context.Background(), schema.ChatRequest{
		Session: "session-123",
		Text:    "describe this",
	}, httpclient.WithChatURL("https://example.com/image.png"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Role != "assistant" {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
	if len(resp.Content) > 0 && resp.Content[0].Text != nil {
		if !strings.Contains(*resp.Content[0].Text, "describe this") {
			t.Fatalf("expected response to contain 'describe this', got %q", *resp.Content[0].Text)
		}
	}
}

func TestChat_EmptySession(t *testing.T) {
	srv := newChatTestServer(t)
	defer srv.Close()
	c := newClient(t, srv.URL)

	_, err := c.Chat(context.Background(), schema.ChatRequest{
		Text: "hello",
	})
	if err == nil {
		t.Fatal("expected error for empty session")
	}
}

func TestChat_EmptyText(t *testing.T) {
	srv := newChatTestServer(t)
	defer srv.Close()
	c := newClient(t, srv.URL)

	_, err := c.Chat(context.Background(), schema.ChatRequest{
		Session: "session-123",
	})
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestChat_WithTools(t *testing.T) {
	srv := newChatTestServer(t)
	defer srv.Close()
	c := newClient(t, srv.URL)

	resp, err := c.Chat(context.Background(), schema.ChatRequest{
		Session: "session-123",
		Text:    "use my tools",
		Tools:   []string{"tool_a", "tool_b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Role != "assistant" {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
	if resp.Session != "session-123" {
		t.Fatalf("expected session=session-123, got %q", resp.Session)
	}
}
