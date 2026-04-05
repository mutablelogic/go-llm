package httpclient_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient-new"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func newConnectorServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/connector", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			connectors := []*schema.Connector{
				{ConnectorInsert: schema.ConnectorInsert{URL: "https://example.com/public"}},
				{ConnectorInsert: schema.ConnectorInsert{URL: "https://example.com/namespaced", ConnectorMeta: schema.ConnectorMeta{Namespace: types.Ptr("mcp"), Enabled: types.Ptr(false)}}},
			}

			filtered := make([]*schema.Connector, 0, len(connectors))
			for _, connector := range connectors {
				if namespace := r.URL.Query().Get("namespace"); namespace != "" && types.Value(connector.Namespace) != namespace {
					continue
				}
				if enabled := r.URL.Query().Get("enabled"); enabled != "" {
					want := enabled == "true"
					if types.Value(connector.Enabled) != want {
						continue
					}
				}
				filtered = append(filtered, connector)
			}

			response := schema.ConnectorList{Count: uint(len(filtered)), Body: filtered}
			w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
			_ = json.NewEncoder(w).Encode(response)
		case http.MethodPost:
			var req schema.ConnectorInsert
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if req.URL == "" {
				http.Error(w, "connector URL cannot be empty", http.StatusBadRequest)
				return
			}

			response := schema.Connector{
				ConnectorInsert: req,
			}

			w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
			_ = json.NewEncoder(w).Encode(response)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/connector/", func(w http.ResponseWriter, r *http.Request) {
		rawURL, err := url.PathUnescape(r.URL.Path[len("/api/connector/"):])
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if rawURL == "" {
			http.Error(w, "connector URL cannot be empty", http.StatusBadRequest)
			return
		}
		if rawURL == "https://missing.example.com/sse" {
			http.Error(w, "connector not found", http.StatusNotFound)
			return
		}

		response := schema.Connector{ConnectorInsert: schema.ConnectorInsert{URL: rawURL}}
		switch r.Method {
		case http.MethodGet, http.MethodDelete:
			w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
			_ = json.NewEncoder(w).Encode(response)
		case http.MethodPatch:
			var req schema.ConnectorMeta
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			response.ConnectorMeta = req
			w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
			_ = json.NewEncoder(w).Encode(response)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return httptest.NewServer(mux)
}

func newConnectorClient(t *testing.T, serverURL string) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(serverURL + "/api")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestCreateConnector(t *testing.T) {
	server := newConnectorServer(t)
	defer server.Close()

	client := newConnectorClient(t, server.URL)
	enabled := false
	namespace := "mcp"
	response, err := client.CreateConnector(context.Background(), schema.ConnectorInsert{
		URL: "https://example.com/sse",
		ConnectorMeta: schema.ConnectorMeta{
			Enabled:   &enabled,
			Namespace: &namespace,
			Groups:    []string{"admins"},
			Meta:      schema.ProviderMetaMap{"env": "dev"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.URL != "https://example.com/sse" {
		t.Fatalf("expected URL %q, got %q", "https://example.com/sse", response.URL)
	}
	if types.Value(response.Namespace) != "mcp" {
		t.Fatalf("expected namespace %q, got %q", "mcp", types.Value(response.Namespace))
	}
	if types.Value(response.Enabled) {
		t.Fatal("expected connector to be disabled")
	}
	if len(response.Groups) != 1 || response.Groups[0] != "admins" {
		t.Fatalf("unexpected groups: %v", response.Groups)
	}
	if response.Meta["env"] != "dev" {
		t.Fatalf("unexpected meta: %v", response.Meta)
	}
}

func TestListConnectors(t *testing.T) {
	server := newConnectorServer(t)
	defer server.Close()

	client := newConnectorClient(t, server.URL)
	response, err := client.ListConnectors(context.Background(), schema.ConnectorListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response.Count != 2 || len(response.Body) != 2 {
		t.Fatalf("expected 2 connectors, got count=%d len=%d", response.Count, len(response.Body))
	}
}

func TestListConnectorsWithFilters(t *testing.T) {
	server := newConnectorServer(t)
	defer server.Close()

	client := newConnectorClient(t, server.URL)
	disabled := false
	response, err := client.ListConnectors(context.Background(), schema.ConnectorListRequest{Namespace: "mcp", Enabled: &disabled})
	if err != nil {
		t.Fatal(err)
	}
	if response.Count != 1 || len(response.Body) != 1 {
		t.Fatalf("expected 1 connector, got count=%d len=%d", response.Count, len(response.Body))
	}
	if got := response.Body[0].URL; got != "https://example.com/namespaced" {
		t.Fatalf("expected filtered connector URL %q, got %q", "https://example.com/namespaced", got)
	}
}

func TestGetConnector(t *testing.T) {
	server := newConnectorServer(t)
	defer server.Close()

	client := newConnectorClient(t, server.URL)
	response, err := client.GetConnector(context.Background(), "https://example.com/sse")
	if err != nil {
		t.Fatal(err)
	}
	if response.URL != "https://example.com/sse" {
		t.Fatalf("expected URL %q, got %q", "https://example.com/sse", response.URL)
	}
}

func TestUpdateConnector(t *testing.T) {
	server := newConnectorServer(t)
	defer server.Close()

	client := newConnectorClient(t, server.URL)
	enabled := false
	namespace := "mcp"
	response, err := client.UpdateConnector(context.Background(), "https://example.com/sse", schema.ConnectorMeta{
		Enabled:   &enabled,
		Namespace: &namespace,
		Groups:    []string{"admins"},
		Meta:      schema.ProviderMetaMap{"env": "dev"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.URL != "https://example.com/sse" {
		t.Fatalf("expected URL %q, got %q", "https://example.com/sse", response.URL)
	}
	if types.Value(response.Namespace) != "mcp" {
		t.Fatalf("expected namespace %q, got %q", "mcp", types.Value(response.Namespace))
	}
	if types.Value(response.Enabled) {
		t.Fatal("expected connector to be disabled")
	}
	if len(response.Groups) != 1 || response.Groups[0] != "admins" {
		t.Fatalf("unexpected groups: %v", response.Groups)
	}
	if response.Meta["env"] != "dev" {
		t.Fatalf("unexpected meta: %v", response.Meta)
	}
}

func TestCreateConnectorEmptyURL(t *testing.T) {
	server := newConnectorServer(t)
	defer server.Close()

	client := newConnectorClient(t, server.URL)
	_, err := client.CreateConnector(context.Background(), schema.ConnectorInsert{})
	if err == nil {
		t.Fatal("expected error for empty connector URL")
	}
	_, err = client.GetConnector(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty connector URL")
	}
	_, err = client.UpdateConnector(context.Background(), "", schema.ConnectorMeta{})
	if err == nil {
		t.Fatal("expected error for empty connector URL")
	}
}

func TestCreateConnectorMalformedJSONResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/connector", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
		_, _ = w.Write(bytes.TrimSpace([]byte(`{"url":`)))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectorClient(t, server.URL)
	_, err := client.CreateConnector(context.Background(), schema.ConnectorInsert{URL: "https://example.com/sse"})
	if err == nil {
		t.Fatal("expected error for malformed response")
	}
}

func TestDeleteConnector(t *testing.T) {
	server := newConnectorServer(t)
	defer server.Close()

	client := newConnectorClient(t, server.URL)
	response, err := client.DeleteConnector(context.Background(), "https://example.com/sse")
	if err != nil {
		t.Fatal(err)
	}
	if response.URL != "https://example.com/sse" {
		t.Fatalf("expected URL %q, got %q", "https://example.com/sse", response.URL)
	}
}

func TestDeleteConnectorEmptyURL(t *testing.T) {
	server := newConnectorServer(t)
	defer server.Close()

	client := newConnectorClient(t, server.URL)
	_, err := client.DeleteConnector(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty connector URL")
	}
}

func TestDeleteConnectorNotFound(t *testing.T) {
	server := newConnectorServer(t)
	defer server.Close()

	client := newConnectorClient(t, server.URL)
	_, err := client.DeleteConnector(context.Background(), "https://missing.example.com/sse")
	if err == nil {
		t.Fatal("expected error for missing connector")
	}
}

func TestGetConnectorNotFound(t *testing.T) {
	server := newConnectorServer(t)
	defer server.Close()

	client := newConnectorClient(t, server.URL)
	_, err := client.GetConnector(context.Background(), "https://missing.example.com/sse")
	if err == nil {
		t.Fatal("expected error for missing connector")
	}
}

func TestUpdateConnectorNotFound(t *testing.T) {
	server := newConnectorServer(t)
	defer server.Close()

	client := newConnectorClient(t, server.URL)
	enabled := false
	_, err := client.UpdateConnector(context.Background(), "https://missing.example.com/sse", schema.ConnectorMeta{Enabled: &enabled})
	if err == nil {
		t.Fatal("expected error for missing connector")
	}
}
