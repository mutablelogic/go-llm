package httpclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient-new"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func newToolServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	tools := []schema.ToolMeta{
		{
			Name:        "builtin.alpha",
			Title:       "Alpha Tool",
			Description: "A",
			Input:       schema.JSONSchema(mustToolSchemaJSON(t, jsonschema.MustFor[map[string]any]())),
			Output:      schema.JSONSchema(mustToolSchemaJSON(t, jsonschema.MustFor[string]())),
			Hints:       []string{"readonly"},
		},
		{Name: "builtin.bravo", Description: "B"},
		{Name: "remote.echo", Description: "Echo"},
	}
	mux.HandleFunc("/api/tool", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		filtered := make([]schema.ToolMeta, 0, len(tools))
		for _, tool := range tools {
			if namespace := r.URL.Query().Get("namespace"); namespace != "" {
				if len(tool.Name) <= len(namespace) || tool.Name[:len(namespace)] != namespace {
					continue
				}
			}
			if names, ok := r.URL.Query()["name"]; ok && len(names) > 0 {
				matched := false
				for _, name := range names {
					if tool.Name == name {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			filtered = append(filtered, tool)
		}

		response := schema.ToolList{Count: uint(len(filtered)), Body: filtered}
		if limit := r.URL.Query().Get("limit"); limit == "1" && len(filtered) > 1 {
			response.Body = filtered[:1]
		}

		w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
		_ = json.NewEncoder(w).Encode(response)
	})
	mux.HandleFunc("/api/tool/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := r.URL.Path[len("/api/tool/"):]
		for _, tool := range tools {
			if tool.Name == name {
				w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
				_ = json.NewEncoder(w).Encode(tool)
				return
			}
		}

		http.NotFound(w, r)
	})

	return httptest.NewServer(mux)
}

func newToolClient(t *testing.T, serverURL string) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(serverURL + "/api")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func mustToolSchemaJSON(t *testing.T, schemaValue interface{ MarshalJSON() ([]byte, error) }) []byte {
	t.Helper()

	data, err := schemaValue.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestListTools(t *testing.T) {
	server := newToolServer(t)
	defer server.Close()

	client := newToolClient(t, server.URL)
	response, err := client.ListTools(context.Background(), schema.ToolListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response.Count != 3 || len(response.Body) != 3 {
		t.Fatalf("expected 3 tools, got count=%d len=%d", response.Count, len(response.Body))
	}
	if response.Body[0].Name != "builtin.alpha" {
		t.Fatalf("expected first tool %q, got %q", "builtin.alpha", response.Body[0].Name)
	}
	if response.Body[0].Title != "Alpha Tool" {
		t.Fatalf("expected title %q, got %q", "Alpha Tool", response.Body[0].Title)
	}
	if len(response.Body[0].Hints) != 1 || response.Body[0].Hints[0] != "readonly" {
		t.Fatalf("unexpected hints: %+v", response.Body[0].Hints)
	}
	if string(response.Body[0].Input) == "" || string(response.Body[0].Output) == "" {
		t.Fatal("expected schemas in tool response")
	}
}

func TestListToolsWithFilters(t *testing.T) {
	server := newToolServer(t)
	defer server.Close()

	client := newToolClient(t, server.URL)
	limit := uint64(1)
	response, err := client.ListTools(context.Background(), schema.ToolListRequest{
		Namespace: "builtin",
		Name:      []string{"builtin.alpha", "builtin.bravo"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Count != 2 || len(response.Body) != 2 {
		t.Fatalf("expected 2 filtered tools, got count=%d len=%d", response.Count, len(response.Body))
	}
	if response.Body[0].Name != "builtin.alpha" || response.Body[1].Name != "builtin.bravo" {
		t.Fatalf("unexpected filtered tools: %+v", response.Body)
	}
	response, err = client.ListTools(context.Background(), schema.ToolListRequest{
		OffsetLimit: pg.OffsetLimit{Limit: &limit},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Body) != 1 {
		t.Fatalf("expected paginated response length 1, got %d", len(response.Body))
	}
}

func TestGetTool(t *testing.T) {
	server := newToolServer(t)
	defer server.Close()

	client := newToolClient(t, server.URL)
	response, err := client.GetTool(context.Background(), "builtin.alpha")
	if err != nil {
		t.Fatal(err)
	}
	if response.Name != "builtin.alpha" {
		t.Fatalf("expected tool %q, got %q", "builtin.alpha", response.Name)
	}
	if response.Title != "Alpha Tool" {
		t.Fatalf("expected title %q, got %q", "Alpha Tool", response.Title)
	}
	if string(response.Input) == "" || string(response.Output) == "" {
		t.Fatal("expected tool schemas in response")
	}
}

func TestGetToolNotFound(t *testing.T) {
	server := newToolServer(t)
	defer server.Close()

	client := newToolClient(t, server.URL)
	if _, err := client.GetTool(context.Background(), "builtin.missing"); err == nil {
		t.Fatal("expected not found error, got nil")
	}
}
