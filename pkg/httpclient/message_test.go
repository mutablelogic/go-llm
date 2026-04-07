package httpclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	uuid "github.com/google/uuid"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func newMessageServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/session/11111111-1111-1111-1111-111111111111/message", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if got := r.URL.Query().Get("role"); got != "assistant" {
			http.Error(w, "unexpected role query", http.StatusBadRequest)
			return
		}
		if got := r.URL.Query().Get("text"); got != "news" {
			http.Error(w, "unexpected text query", http.StatusBadRequest)
			return
		}
		if got := r.URL.Query().Get("limit"); got != "1" {
			http.Error(w, "unexpected limit query", http.StatusBadRequest)
			return
		}

		response := schema.MessageList{
			MessageListRequest: schema.MessageListRequest{
				Role: "assistant",
				Text: "news",
			},
			Count: 1,
			Body: []*schema.Message{{
				Role: schema.RoleAssistant,
				Content: []schema.ContentBlock{
					{Text: types.Ptr("daily news summary")},
				},
				Tokens: 4,
				Result: schema.ResultStop,
			}},
		}

		w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
		_ = json.NewEncoder(w).Encode(response)
	})

	return httptest.NewServer(mux)
}

func newMessageClient(t *testing.T, serverURL string) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(serverURL + "/api")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestListMessages(t *testing.T) {
	server := newMessageServer(t)
	defer server.Close()

	client := newMessageClient(t, server.URL)
	limit := uint64(1)
	response, err := client.ListMessages(context.Background(), uuid.MustParse("11111111-1111-1111-1111-111111111111"), schema.MessageListRequest{
		OffsetLimit: pg.OffsetLimit{Limit: &limit},
		Role:        schema.RoleAssistant,
		Text:        "news",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Count != 1 || len(response.Body) != 1 {
		t.Fatalf("expected 1 message, got count=%d len=%d", response.Count, len(response.Body))
	}
	if response.Body[0].Role != schema.RoleAssistant {
		t.Fatalf("expected role %q, got %q", schema.RoleAssistant, response.Body[0].Role)
	}
	if got := response.Body[0].Text(); got != "daily news summary" {
		t.Fatalf("expected %q, got %q", "daily news summary", got)
	}
	if response.Body[0].Tokens != 4 {
		t.Fatalf("expected 4 tokens, got %d", response.Body[0].Tokens)
	}
	if response.Body[0].Result != schema.ResultStop {
		t.Fatalf("expected result %q, got %q", schema.ResultStop, response.Body[0].Result)
	}
}

func TestListMessagesEmptySession(t *testing.T) {
	server := newMessageServer(t)
	defer server.Close()

	client := newMessageClient(t, server.URL)
	_, err := client.ListMessages(context.Background(), uuid.Nil, schema.MessageListRequest{})
	if err == nil {
		t.Fatal("expected error for empty session")
	}
}
