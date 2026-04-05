package httpclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient-new"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	"github.com/mutablelogic/go-server/pkg/types"
)

func newAgentServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	agents := []schema.AgentMeta{
		{Name: "builtin.alpha", Title: "Alpha Agent", Description: "A"},
		{Name: "builtin.bravo", Title: "Bravo Agent", Description: "B"},
		{Name: "remote.echo", Title: "Echo Agent", Description: "Echo"},
	}
	mux.HandleFunc("/api/agent", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		filtered := make([]schema.AgentMeta, 0, len(agents))
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
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name, err := url.PathUnescape(r.URL.Path[len("/api/agent/"):])
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, agent := range agents {
			if agent.Name == name {
				w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
				_ = json.NewEncoder(w).Encode(agent)
				return
			}
		}
		http.NotFound(w, r)
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
