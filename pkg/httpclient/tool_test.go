package httpclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func newToolServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	tools := []*schema.ToolMeta{
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

		filtered := make([]*schema.ToolMeta, 0, len(tools))
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
		name, err := url.PathUnescape(r.URL.Path[len("/api/tool/"):])
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			for _, tool := range tools {
				if tool.Name == name {
					w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
					_ = json.NewEncoder(w).Encode(tool)
					return
				}
			}
			http.NotFound(w, r)
		case http.MethodPost:
			var req schema.CallToolRequest
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

func TestGetToolEscapedName(t *testing.T) {
	server := newToolServer(t)
	defer server.Close()

	client := newToolClient(t, server.URL)
	response, err := client.GetTool(context.Background(), "builtin%2Ealpha")
	if err != nil {
		t.Fatal(err)
	}
	if response.Name != "builtin.alpha" {
		t.Fatalf("expected tool %q, got %q", "builtin.alpha", response.Name)
	}
}

func TestCallTool(t *testing.T) {
	server := newToolServer(t)
	defer server.Close()

	client := newToolClient(t, server.URL)
	response, err := client.CallTool(context.Background(), "builtin.alpha", schema.CallToolRequest{Input: json.RawMessage(`{"query":"docs"}`)})
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

func TestCallToolText(t *testing.T) {
	server := newToolServer(t)
	defer server.Close()

	client := newToolClient(t, server.URL)
	response, err := client.CallTool(context.Background(), "remote.echo", schema.CallToolRequest{})
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

func TestCallToolNoContent(t *testing.T) {
	server := newToolServer(t)
	defer server.Close()

	client := newToolClient(t, server.URL)
	response, err := client.CallTool(context.Background(), "builtin.bravo", schema.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response != nil {
		t.Fatalf("expected nil resource, got %T", response)
	}
}

func TestCallToolNotFound(t *testing.T) {
	server := newToolServer(t)
	defer server.Close()

	client := newToolClient(t, server.URL)
	if _, err := client.CallTool(context.Background(), "builtin.missing", schema.CallToolRequest{}); err == nil {
		t.Fatal("expected not found error, got nil")
	}
}

func TestCallToolEscapedName(t *testing.T) {
	server := newToolServer(t)
	defer server.Close()

	client := newToolClient(t, server.URL)
	response, err := client.CallTool(context.Background(), "builtin%2Ealpha", schema.CallToolRequest{Input: json.RawMessage(`{"query":"docs"}`)})
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
