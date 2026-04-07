package httpclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/kernel/httpclient"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS — minimal in-memory store used by the test server

type connectorStore struct {
	mu   sync.Mutex
	data map[string]*schema.Connector
}

func newConnectorStore() *connectorStore {
	return &connectorStore{data: make(map[string]*schema.Connector)}
}

func connectorInsert(rawURL string, meta schema.ConnectorMeta) schema.ConnectorInsert {
	return schema.ConnectorInsert{URL: rawURL, ConnectorMeta: meta}
}

func (s *connectorStore) create(rawURL string, meta schema.ConnectorMeta) (*schema.Connector, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[rawURL]; exists {
		return nil, http.StatusConflict
	}
	c := &schema.Connector{
		ConnectorInsert: schema.ConnectorInsert{URL: rawURL, ConnectorMeta: meta},
		CreatedAt:       time.Now(),
	}
	s.data[rawURL] = c
	return c, http.StatusCreated
}

func (s *connectorStore) get(rawURL string) (*schema.Connector, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.data[rawURL]
	if !ok {
		return nil, http.StatusNotFound
	}
	return c, http.StatusOK
}

func (s *connectorStore) update(rawURL string, meta schema.ConnectorMeta) (*schema.Connector, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.data[rawURL]
	if !ok {
		return nil, http.StatusNotFound
	}
	c.ConnectorMeta = meta
	return c, http.StatusOK
}

func (s *connectorStore) delete(rawURL string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[rawURL]; !ok {
		return http.StatusNotFound
	}
	delete(s.data, rawURL)
	return http.StatusNoContent
}

func (s *connectorStore) list(namespace string, enabled *bool) *schema.ConnectorList {
	s.mu.Lock()
	defer s.mu.Unlock()
	var body []*schema.Connector
	for _, c := range s.data {
		if namespace != "" && types.Value(c.Namespace) != namespace {
			continue
		}
		if enabled != nil && types.Value(c.Enabled) != types.Value(enabled) {
			continue
		}
		cp := *c
		body = append(body, &cp)
	}
	return &schema.ConnectorList{Count: uint(len(body)), Body: body}
}

// newConnectorTestServer returns an httptest.Server that mimics the connector API.
func newConnectorTestServer(t *testing.T) (*httptest.Server, *connectorStore) {
	t.Helper()
	store := newConnectorStore()
	mux := http.NewServeMux()

	// GET /api/connector — list
	mux.HandleFunc("/api/connector", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		var enabled *bool
		if v := q.Get("enabled"); v != "" {
			b := v == "true"
			enabled = &b
		}
		resp := store.list(q.Get("namespace"), enabled)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// GET|POST|PATCH|DELETE /api/connector/{url}
	mux.HandleFunc("/api/connector/", func(w http.ResponseWriter, r *http.Request) {
		rawURL, _ := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/api/connector/"))
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			c, code := store.get(rawURL)
			if code != http.StatusOK {
				http.Error(w, http.StatusText(code), code)
				return
			}
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(c)
		case http.MethodPost:
			var meta schema.ConnectorMeta
			if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			c, code := store.create(rawURL, meta)
			if code != http.StatusCreated {
				http.Error(w, http.StatusText(code), code)
				return
			}
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(c)
		case http.MethodPatch:
			var meta schema.ConnectorMeta
			if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			c, code := store.update(rawURL, meta)
			if code != http.StatusOK {
				http.Error(w, http.StatusText(code), code)
				return
			}
			json.NewEncoder(w).Encode(c)
		case http.MethodDelete:
			code := store.delete(rawURL)
			w.WriteHeader(code)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return httptest.NewServer(mux), store
}

func newConnectorClient(t *testing.T, serverURL string) *httpclient.Client {
	t.Helper()
	c, err := httpclient.New(serverURL + "/api")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func TestConnectorClient_ListEmpty(t *testing.T) {
	srv, _ := newConnectorTestServer(t)
	defer srv.Close()
	c := newConnectorClient(t, srv.URL)

	resp, err := c.ListConnectors(context.TODO(), schema.ConnectorListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 0 {
		t.Fatalf("expected 0, got %d", resp.Count)
	}
}

func TestConnectorClient_CreateAndGet(t *testing.T) {
	srv, _ := newConnectorTestServer(t)
	defer srv.Close()
	c := newConnectorClient(t, srv.URL)

	created, err := c.CreateConnector(context.TODO(), connectorInsert("https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.Ptr("mcp")}))
	if err != nil {
		t.Fatal(err)
	}
	if created.URL != "https://example.com/sse" {
		t.Fatalf("unexpected URL: %q", created.URL)
	}
	if !types.Value(created.Enabled) {
		t.Fatal("expected enabled")
	}
	if types.Value(created.Namespace) != "mcp" {
		t.Fatalf("expected namespace %q, got %q", "mcp", types.Value(created.Namespace))
	}

	got, err := c.GetConnector(context.TODO(), "https://example.com/sse")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != created.URL {
		t.Fatalf("URL mismatch: %q vs %q", created.URL, got.URL)
	}
}

func TestConnectorClient_UpdateAndGet(t *testing.T) {
	srv, _ := newConnectorTestServer(t)
	defer srv.Close()
	c := newConnectorClient(t, srv.URL)

	if _, err := c.CreateConnector(context.TODO(), connectorInsert("https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(false), Namespace: types.Ptr("old")})); err != nil {
		t.Fatal(err)
	}
	updated, err := c.UpdateConnector(context.TODO(), "https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.Ptr("new")})
	if err != nil {
		t.Fatal(err)
	}
	if !types.Value(updated.Enabled) {
		t.Fatal("expected enabled=true after update")
	}
	if types.Value(updated.Namespace) != "new" {
		t.Fatalf("expected namespace %q, got %q", "new", types.Value(updated.Namespace))
	}
}

func TestConnectorClient_DeleteAndVerify(t *testing.T) {
	srv, _ := newConnectorTestServer(t)
	defer srv.Close()
	c := newConnectorClient(t, srv.URL)

	if _, err := c.CreateConnector(context.TODO(), connectorInsert("https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true)})); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteConnector(context.TODO(), "https://example.com/sse"); err != nil {
		t.Fatal(err)
	}
	// Second delete should fail (404)
	if err := c.DeleteConnector(context.TODO(), "https://example.com/sse"); err == nil {
		t.Fatal("expected error after double-delete")
	}
}

func TestConnectorClient_ListWithFilter(t *testing.T) {
	srv, _ := newConnectorTestServer(t)
	defer srv.Close()
	c := newConnectorClient(t, srv.URL)

	for rawURL, ns := range map[string]string{
		"https://a.example.com/sse": "ns1",
		"https://b.example.com/sse": "ns2",
	} {
		if _, err := c.CreateConnector(context.TODO(), connectorInsert(rawURL, schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.Ptr(ns)})); err != nil {
			t.Fatalf("create %s: %v", rawURL, err)
		}
	}

	all, err := c.ListConnectors(context.TODO(), schema.ConnectorListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if all.Count != 2 {
		t.Fatalf("expected 2, got %d", all.Count)
	}

	filtered, err := c.ListConnectors(context.TODO(), schema.ConnectorListRequest{Namespace: "ns1"})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Count != 1 {
		t.Fatalf("expected 1 for ns1, got %d", filtered.Count)
	}

	enabledOnly, err := c.ListConnectors(context.TODO(), schema.ConnectorListRequest{Enabled: types.Ptr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if enabledOnly.Count != 2 {
		t.Fatalf("expected 2 enabled, got %d", enabledOnly.Count)
	}
}

func TestConnectorClient_EmptyURL(t *testing.T) {
	srv, _ := newConnectorTestServer(t)
	defer srv.Close()
	c := newConnectorClient(t, srv.URL)

	if _, err := c.GetConnector(context.TODO(), ""); err == nil {
		t.Fatal("expected error for empty URL in GetConnector")
	}
	if _, err := c.CreateConnector(context.TODO(), connectorInsert("", schema.ConnectorMeta{})); err == nil {
		t.Fatal("expected error for empty URL in CreateConnector")
	}
	if _, err := c.UpdateConnector(context.TODO(), "", schema.ConnectorMeta{}); err == nil {
		t.Fatal("expected error for empty URL in UpdateConnector")
	}
	if err := c.DeleteConnector(context.TODO(), ""); err == nil {
		t.Fatal("expected error for empty URL in DeleteConnector")
	}
}
