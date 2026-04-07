package httpclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/kernel/httpclient"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func newAgentServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	agents := []*schema.AgentMeta{
		{Name: "builtin.alpha", Title: "Alpha Agent", Description: "A"},
		{Name: "builtin.bravo", Title: "Bravo Agent", Description: "B"},
		{Name: "remote.echo", Title: "Echo Agent", Description: "Echo"},
	}
	mux.HandleFunc("/api/agent", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		filtered := make([]*schema.AgentMeta, 0, len(agents))
		for _, agent := range agents {
			if namespace := r.URL.Query().Get("namespace"); namespace != "" {
				if len(agent.Name) <= len(namespace) || agent.Name[:len(namespace)] != namespace {
					continue
				}
			}
			if names, ok := r.URL.Query()["name"]; ok && len(names) > 0 {
				matched := false
				for _, name := range names {
					if agent.Name == name {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			filtered = append(filtered, agent)
		}

		response := schema.AgentList{Count: uint(len(filtered)), Body: filtered}
		if limit := r.URL.Query().Get("limit"); limit == "1" && len(filtered) > 1 {
			response.Body = filtered[:1]
		}

		w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
		_ = json.NewEncoder(w).Encode(response)
	})
	mux.HandleFunc("/api/agent/", func(w http.ResponseWriter, r *http.Request) {
		name, err := url.PathUnescape(r.URL.Path[len("/api/agent/"):])
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			for _, agent := range agents {
				if agent.Name == name {
					w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
					_ = json.NewEncoder(w).Encode(agent)
					return
				}
			}
			http.NotFound(w, r)
		case http.MethodPost:
			var req schema.CallAgentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			switch name {
			case "builtin.alpha":
				w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
				w.Header().Set(types.ContentPathHeader, "json:alpha")
				w.Header().Set(types.ContentNameHeader, "alpha")
				w.Header().Set(types.ContentDescriptionHeader, "Alpha result")
				_, _ = w.Write(req.Input)
			case "builtin.bravo":
				w.WriteHeader(http.StatusNoContent)
			case "remote.echo":
				w.Header().Set(types.ContentTypeHeader, types.ContentTypeTextPlain)
				w.Header().Set(types.ContentPathHeader, "text:echo")
				w.Header().Set(types.ContentNameHeader, "echo")
				w.Header().Set(types.ContentDescriptionHeader, "Echo result")
				_, _ = w.Write([]byte("echo"))
			default:
				http.NotFound(w, r)
			}
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return httptest.NewServer(mux)
}

func newAgentClient(t *testing.T, serverURL string) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(serverURL + "/api")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestListAgents(t *testing.T) {
	server := newAgentServer(t)
	defer server.Close()

	client := newAgentClient(t, server.URL)
	response, err := client.ListAgents(context.Background(), schema.AgentListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response.Count != 3 || len(response.Body) != 3 {
		t.Fatalf("expected 3 agents, got count=%d len=%d", response.Count, len(response.Body))
	}
	if response.Body[0].Name != "builtin.alpha" {
		t.Fatalf("expected first agent %q, got %q", "builtin.alpha", response.Body[0].Name)
	}
	if response.Body[0].Title != "Alpha Agent" {
		t.Fatalf("expected title %q, got %q", "Alpha Agent", response.Body[0].Title)
	}
}

func TestListAgentsWithFilters(t *testing.T) {
	server := newAgentServer(t)
	defer server.Close()

	client := newAgentClient(t, server.URL)
	limit := uint64(1)
	response, err := client.ListAgents(context.Background(), schema.AgentListRequest{
		Namespace: "builtin",
		Name:      []string{"builtin.alpha", "builtin.bravo"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Count != 2 || len(response.Body) != 2 {
		t.Fatalf("expected 2 filtered agents, got count=%d len=%d", response.Count, len(response.Body))
	}
	if response.Body[0].Name != "builtin.alpha" || response.Body[1].Name != "builtin.bravo" {
		t.Fatalf("unexpected filtered agents: %+v", response.Body)
	}

	response, err = client.ListAgents(context.Background(), schema.AgentListRequest{
		OffsetLimit: pg.OffsetLimit{Limit: &limit},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Body) != 1 {
		t.Fatalf("expected paginated response length 1, got %d", len(response.Body))
	}
}

func TestGetAgent(t *testing.T) {
	server := newAgentServer(t)
	defer server.Close()

	client := newAgentClient(t, server.URL)
	response, err := client.GetAgent(context.Background(), "builtin.alpha")
	if err != nil {
		t.Fatal(err)
	}
	if response.Name != "builtin.alpha" {
		t.Fatalf("expected agent %q, got %q", "builtin.alpha", response.Name)
	}
	if response.Title != "Alpha Agent" {
		t.Fatalf("expected title %q, got %q", "Alpha Agent", response.Title)
	}
}

func TestGetAgentNotFound(t *testing.T) {
	server := newAgentServer(t)
	defer server.Close()

	client := newAgentClient(t, server.URL)
	if _, err := client.GetAgent(context.Background(), "builtin.missing"); err == nil {
		t.Fatal("expected not found error, got nil")
	}
}

func TestGetAgentEscapedName(t *testing.T) {
	server := newAgentServer(t)
	defer server.Close()

	client := newAgentClient(t, server.URL)
	response, err := client.GetAgent(context.Background(), "builtin%2Ealpha")
	if err != nil {
		t.Fatal(err)
	}
	if response.Name != "builtin.alpha" {
		t.Fatalf("expected agent %q, got %q", "builtin.alpha", response.Name)
	}
}

func TestCallAgent(t *testing.T) {
	server := newAgentServer(t)
	defer server.Close()

	client := newAgentClient(t, server.URL)
	response, err := client.CallAgent(context.Background(), "builtin.alpha", schema.CallAgentRequest{CallToolRequest: schema.CallToolRequest{Input: json.RawMessage(`{"query":"docs"}`)}})
	if err != nil {
		t.Fatal(err)
	}
	if response == nil {
		t.Fatal("expected resource, got nil")
	}
	if response.URI() != "json:alpha" {
		t.Fatalf("expected uri %q, got %q", "json:alpha", response.URI())
	}
	if response.Name() != "alpha" {
		t.Fatalf("expected name %q, got %q", "alpha", response.Name())
	}
	if response.Description() != "Alpha result" {
		t.Fatalf("expected description %q, got %q", "Alpha result", response.Description())
	}
	if response.Type() != types.ContentTypeJSON {
		t.Fatalf("expected content type %q, got %q", types.ContentTypeJSON, response.Type())
	}
	data, err := response.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"query":"docs"}` {
		t.Fatalf("unexpected response body: %s", string(data))
	}
}

func TestCallAgentText(t *testing.T) {
	server := newAgentServer(t)
	defer server.Close()

	client := newAgentClient(t, server.URL)
	response, err := client.CallAgent(context.Background(), "remote.echo", schema.CallAgentRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response == nil {
		t.Fatal("expected resource, got nil")
	}
	if response.Type() != types.ContentTypeTextPlain {
		t.Fatalf("expected content type %q, got %q", types.ContentTypeTextPlain, response.Type())
	}
	if response.Name() != "echo" {
		t.Fatalf("expected name %q, got %q", "echo", response.Name())
	}
	if response.Description() != "Echo result" {
		t.Fatalf("expected description %q, got %q", "Echo result", response.Description())
	}
	data, err := response.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "echo" {
		t.Fatalf("unexpected response body: %s", string(data))
	}
}

func TestCallAgentNoContent(t *testing.T) {
	server := newAgentServer(t)
	defer server.Close()

	client := newAgentClient(t, server.URL)
	response, err := client.CallAgent(context.Background(), "builtin.bravo", schema.CallAgentRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response != nil {
		t.Fatalf("expected nil resource, got %T", response)
	}
}

func TestCallAgentNotFound(t *testing.T) {
	server := newAgentServer(t)
	defer server.Close()

	client := newAgentClient(t, server.URL)
	if _, err := client.CallAgent(context.Background(), "builtin.missing", schema.CallAgentRequest{}); err == nil {
		t.Fatal("expected not found error, got nil")
	}
}

func TestCallAgentEscapedName(t *testing.T) {
	server := newAgentServer(t)
	defer server.Close()

	client := newAgentClient(t, server.URL)
	response, err := client.CallAgent(context.Background(), "builtin%2Ealpha", schema.CallAgentRequest{CallToolRequest: schema.CallToolRequest{Input: json.RawMessage(`{"query":"docs"}`)}})
	if err != nil {
		t.Fatal(err)
	}
	if response == nil {
		t.Fatal("expected resource, got nil")
	}
	data, err := response.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"query":"docs"}` {
		t.Fatalf("unexpected response body: %s", string(data))
	}
}
