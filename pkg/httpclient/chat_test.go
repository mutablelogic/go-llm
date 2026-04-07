package httpclient_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	uuid "github.com/google/uuid"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func newChatServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req schema.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		response := schema.ChatResponse{
			CompletionResponse: schema.CompletionResponse{
				Role:   schema.RoleAssistant,
				Result: schema.ResultStop,
				Content: []schema.ContentBlock{
					{Text: types.Ptr("echo: " + req.Text)},
				},
			},
			Usage: &schema.UsageMeta{InputTokens: 5, OutputTokens: 7},
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

func newChatClient(t *testing.T, serverURL string) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(serverURL + "/api")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestChatJSON(t *testing.T) {
	server := newChatServer(t)
	defer server.Close()

	client := newChatClient(t, server.URL)
	response, err := client.Chat(context.Background(), schema.ChatRequest{
		Session: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Text:    "hello world",
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

func TestChatStream(t *testing.T) {
	server := newChatServer(t)
	defer server.Close()

	client := newChatClient(t, server.URL)
	var chunks []string
	streamFn := opt.StreamFn(func(role, text string) {
		chunks = append(chunks, role+":"+text)
	})

	response, err := client.Chat(context.Background(), schema.ChatRequest{
		Session: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Text:    "stream me",
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

func TestChatEmptySession(t *testing.T) {
	server := newChatServer(t)
	defer server.Close()

	client := newChatClient(t, server.URL)
	_, err := client.Chat(context.Background(), schema.ChatRequest{
		Text: "hello",
	}, nil)
	if err == nil {
		t.Fatal("expected error for empty session")
	}
}

func TestChatEmptyText(t *testing.T) {
	server := newChatServer(t)
	defer server.Close()

	client := newChatClient(t, server.URL)
	_, err := client.Chat(context.Background(), schema.ChatRequest{
		Session: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
	}, nil)
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}
