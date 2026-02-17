package httpclient_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// newTestServer creates an httptest.Server that mimics the /ask endpoint.
// It echoes the text from the request back in the response.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ask", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ct := r.Header.Get("Content-Type")
		var text string
		var attachments int

		switch {
		case len(ct) >= 16 && ct[:16] == "application/json":
			var req schema.AskRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			text = req.Text
			attachments = len(req.Attachments)
		case len(ct) >= 19 && ct[:19] == "multipart/form-data":
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			text = r.FormValue("text")
			if _, _, err := r.FormFile("file"); err == nil {
				attachments = 1
			}
		default:
			http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
			return
		}

		respText := "echo: " + text
		if attachments > 0 {
			respText += " [" + http.StatusText(attachments) + "]"
		}

		resp := schema.AskResponse{
			CompletionResponse: schema.CompletionResponse{
				Role: "assistant",
				Content: []schema.ContentBlock{
					{Text: types.Ptr(respText)},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	return httptest.NewServer(mux)
}

func newClient(t *testing.T, serverURL string) *httpclient.Client {
	t.Helper()
	c, err := httpclient.New(serverURL + "/api")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func TestAsk_NoAttachments(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	c := newClient(t, srv.URL)

	resp, err := c.Ask(context.Background(), schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model"},
			Text:          "hello world",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Role != "assistant" {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
	if len(resp.Content) == 0 || resp.Content[0].Text == nil {
		t.Fatal("expected content with text")
	}
	if got := *resp.Content[0].Text; got != "echo: hello world" {
		t.Fatalf("expected 'echo: hello world', got %q", got)
	}
}

func TestAsk_WithFile(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	c := newClient(t, srv.URL)

	fileData := bytes.NewReader([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	resp, err := c.Ask(context.Background(), schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model"},
			Text:          "describe this",
		},
	}, httpclient.WithFile("image.png", fileData))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Role != "assistant" {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
}

func TestAsk_WithURL(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	c := newClient(t, srv.URL)

	resp, err := c.Ask(context.Background(), schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model"},
			Text:          "describe this",
		},
	}, httpclient.WithURL("https://example.com/image.png"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Role != "assistant" {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
}

func TestAsk_MultipleAttachments(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	c := newClient(t, srv.URL)

	file1 := bytes.NewReader([]byte{0x89, 0x50, 0x4E, 0x47})
	file2 := bytes.NewReader([]byte{0xFF, 0xD8, 0xFF, 0xE0})
	resp, err := c.Ask(context.Background(), schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model"},
			Text:          "compare these",
		},
	}, httpclient.WithFile("a.png", file1), httpclient.WithFile("b.jpg", file2))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Role != "assistant" {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
}

func TestAsk_MixedAttachments(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	c := newClient(t, srv.URL)

	fileData := bytes.NewReader([]byte{0x89, 0x50, 0x4E, 0x47})
	resp, err := c.Ask(context.Background(), schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model"},
			Text:          "compare these",
		},
	}, httpclient.WithFile("a.png", fileData), httpclient.WithURL("https://example.com/b.png"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Role != "assistant" {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
}

func TestAsk_EmptyModel(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	c := newClient(t, srv.URL)

	_, err := c.Ask(context.Background(), schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			Text: "hello",
		},
	})
	if err == nil {
		t.Fatal("expected error for empty model")
	}
}

func TestAsk_EmptyText(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	c := newClient(t, srv.URL)

	_, err := c.Ask(context.Background(), schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model"},
		},
	})
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}
