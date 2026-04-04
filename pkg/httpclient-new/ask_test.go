package httpclient_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient-new"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func newAskServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ask", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req schema.AskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		response := schema.AskResponse{
			CompletionResponse: schema.CompletionResponse{
				Role:   schema.RoleAssistant,
				Result: schema.ResultStop,
				Content: []schema.ContentBlock{
					{Text: types.Ptr("echo: " + req.Text)},
				},
			},
			Usage: &schema.Usage{InputTokens: 5, OutputTokens: 7},
		}

		if r.Header.Get(types.ContentAcceptHeader) == types.ContentTypeTextStream {
			stream := fmt.Sprintf(
				"event: %s\ndata: {\"role\":\"assistant\",\"text\":\"echo: %s\"}\n\n"+
					"event: %s\ndata: %s\n\n",
				schema.EventAssistant,
				req.Text,
				schema.EventResult,
				mustJSON(t, response),
			)
			w.Header().Set(types.ContentTypeHeader, types.ContentTypeTextStream)
			_, _ = w.Write([]byte(stream))
			return
		}

		w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
		_ = json.NewEncoder(w).Encode(response)
	})

	return httptest.NewServer(mux)
}

func newAskClient(t *testing.T, serverURL string) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(serverURL + "/api")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestAskJSON(t *testing.T) {
	server := newAskServer(t)
	defer server.Close()

	client := newAskClient(t, server.URL)
	response, err := client.Ask(context.Background(), schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model"},
			Text:          "hello world",
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.Role != schema.RoleAssistant {
		t.Fatalf("expected role %q, got %q", schema.RoleAssistant, response.Role)
	}
	if len(response.Content) == 0 || response.Content[0].Text == nil {
		t.Fatal("expected content text")
	}
	if got := *response.Content[0].Text; got != "echo: hello world" {
		t.Fatalf("expected %q, got %q", "echo: hello world", got)
	}
	if response.Usage == nil || response.Usage.InputTokens != 5 || response.Usage.OutputTokens != 7 {
		t.Fatalf("unexpected usage: %+v", response.Usage)
	}
}

func TestAskStream(t *testing.T) {
	server := newAskServer(t)
	defer server.Close()

	client := newAskClient(t, server.URL)
	var chunks []string
	streamFn := opt.StreamFn(func(role, text string) {
		chunks = append(chunks, role+":"+text)
	})

	response, err := client.Ask(context.Background(), schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model"},
			Text:          "stream me",
		},
	}, streamFn)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 || chunks[0] != "assistant:echo: stream me" {
		t.Fatalf("unexpected stream chunks: %+v", chunks)
	}
	if response == nil || len(response.Content) == 0 || response.Content[0].Text == nil {
		t.Fatalf("unexpected response: %+v", response)
	}
	if got := *response.Content[0].Text; got != "echo: stream me" {
		t.Fatalf("expected %q, got %q", "echo: stream me", got)
	}
}

func TestAskEmptyModel(t *testing.T) {
	server := newAskServer(t)
	defer server.Close()

	client := newAskClient(t, server.URL)
	_, err := client.Ask(context.Background(), schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{Text: "hello"},
	}, nil)
	if err == nil {
		t.Fatal("expected error for empty model")
	}
}

func TestAskEmptyText(t *testing.T) {
	server := newAskServer(t)
	defer server.Close()

	client := newAskClient(t, server.URL)
	_, err := client.Ask(context.Background(), schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model"},
		},
	}, nil)
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes.TrimSpace(data))
}
