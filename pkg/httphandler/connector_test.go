package httphandler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	// Packages
	manager "github.com/mutablelogic/go-llm/pkg/manager"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

func newConnectorManager(t *testing.T) *manager.Manager {
	t.Helper()
	m, err := manager.NewManager("test", "0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func connectorPath(rawURL string) string {
	return "/connector/" + url.PathEscape(rawURL)
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func TestConnector_ListEmpty(t *testing.T) {
	mgr := newConnectorManager(t)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/connector", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp schema.ListConnectorsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 0 {
		t.Fatalf("expected 0 connectors, got %d", resp.Count)
	}
}

func TestConnector_GetNotFound(t *testing.T) {
	mgr := newConnectorManager(t)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, connectorPath("https://example.com/sse"), nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConnector_CreateAndGet(t *testing.T) {
	mgr := newConnectorManager(t)
	mux := serveMux(mgr)

	meta := schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.StringPtr("mcp")}
	body, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}

	// POST to create
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, connectorPath("https://example.com/sse"), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created schema.Connector
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.URL != "https://example.com/sse" {
		t.Fatalf("unexpected URL: %q", created.URL)
	}
	if !types.Value(created.Enabled) {
		t.Fatal("expected connector to be enabled")
	}
	if types.Value(created.Namespace) != "mcp" {
		t.Fatalf("expected namespace %q, got %q", "mcp", types.Value(created.Namespace))
	}

	// GET to verify round-trip
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, connectorPath("https://example.com/sse"), nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got schema.Connector
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.URL != created.URL {
		t.Fatalf("expected URL %q, got %q", created.URL, got.URL)
	}
}

func TestConnector_CreateConflict(t *testing.T) {
	mgr := newConnectorManager(t)
	mux := serveMux(mgr)

	meta := schema.ConnectorMeta{Enabled: types.Ptr(true)}
	body, _ := json.Marshal(meta)

	post := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, connectorPath("https://example.com/sse"), bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
		return w
	}

	if w := post(); w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if w := post(); w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConnector_CreateInvalidURL(t *testing.T) {
	mgr := newConnectorManager(t)
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.ConnectorMeta{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, connectorPath("ftp://example.com/sse"), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConnector_UpdateAndGet(t *testing.T) {
	mgr := newConnectorManager(t)
	mux := serveMux(mgr)

	// Create
	body, _ := json.Marshal(schema.ConnectorMeta{Enabled: types.Ptr(false), Namespace: types.StringPtr("old")})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, connectorPath("https://example.com/sse"), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create failed: %d: %s", w.Code, w.Body.String())
	}

	// PATCH
	body, _ = json.Marshal(schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.StringPtr("new")})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPatch, connectorPath("https://example.com/sse"), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("update failed: %d: %s", w.Code, w.Body.String())
	}

	var updated schema.Connector
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if !types.Value(updated.Enabled) {
		t.Fatal("expected enabled=true after patch")
	}
	if types.Value(updated.Namespace) != "new" {
		t.Fatalf("expected namespace %q, got %q", "new", types.Value(updated.Namespace))
	}
}

func TestConnector_UpdateNotFound(t *testing.T) {
	mgr := newConnectorManager(t)
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.ConnectorMeta{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, connectorPath("https://example.com/sse"), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConnector_DeleteAndVerify(t *testing.T) {
	mgr := newConnectorManager(t)
	mux := serveMux(mgr)

	// Create
	body, _ := json.Marshal(schema.ConnectorMeta{Enabled: types.Ptr(true)})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, connectorPath("https://example.com/sse"), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create failed: %d", w.Code)
	}

	// DELETE
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodDelete, connectorPath("https://example.com/sse"), nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// GET should now 404
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, connectorPath("https://example.com/sse"), nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", w.Code)
	}
}

func TestConnector_DeleteNotFound(t *testing.T) {
	mgr := newConnectorManager(t)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, connectorPath("https://example.com/sse"), nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConnector_ListWithFilter(t *testing.T) {
	mgr := newConnectorManager(t)
	mux := serveMux(mgr)

	for rawURL, ns := range map[string]string{
		"https://a.example.com/sse": "ns1",
		"https://b.example.com/sse": "ns2",
	} {
		body, _ := json.Marshal(schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.StringPtr(ns)})
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, connectorPath(rawURL), bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s failed: %d", rawURL, w.Code)
		}
	}

	// List all
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/connector", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var all schema.ListConnectorsResponse
	if err := json.NewDecoder(w.Body).Decode(&all); err != nil {
		t.Fatal(err)
	}
	if all.Count != 2 {
		t.Fatalf("expected 2 connectors, got %d", all.Count)
	}

	// List filtered by namespace=ns1
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/connector?namespace=ns1", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with filter, got %d", w.Code)
	}
	var filtered schema.ListConnectorsResponse
	if err := json.NewDecoder(w.Body).Decode(&filtered); err != nil {
		t.Fatal(err)
	}
	if filtered.Count != 1 {
		t.Fatalf("expected 1 connector for ns1, got %d", filtered.Count)
	}
}

func TestConnector_URLCanonicalisationRoundTrip(t *testing.T) {
	mgr := newConnectorManager(t)
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.ConnectorMeta{Enabled: types.Ptr(true)})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, connectorPath("HTTPS://Example.COM/sse?token=abc"), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created schema.Connector
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.URL != "https://example.com/sse" {
		t.Fatalf("expected canonical URL, got %q", created.URL)
	}
}
