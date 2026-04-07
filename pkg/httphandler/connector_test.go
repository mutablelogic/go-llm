package httphandler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func TestConnectorCreateJSONIntegration(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ConnectorHandler(manager)

	if err := manager.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := manager.Exec(context.Background(), `INSERT INTO auth."group" ("id") VALUES ('admins') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}

	enabled := false
	namespace := "mcp"
	rawURL := llmtest.ConnectorURL(t, "handler-create-connector")
	body, err := json.Marshal(schema.ConnectorInsert{
		URL: rawURL,
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

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/connector", bytes.NewReader(body))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.Connector
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.URL != rawURL {
		t.Fatalf("expected URL %q, got %q", rawURL, resp.URL)
	}
	if types.Value(resp.Enabled) {
		t.Fatal("expected connector to be disabled")
	}
	if types.Value(resp.Namespace) != namespace {
		t.Fatalf("expected namespace %q, got %q", namespace, types.Value(resp.Namespace))
	}
	if len(resp.Groups) != 1 || resp.Groups[0] != "admins" {
		t.Fatalf("unexpected groups: %v", resp.Groups)
	}
	if resp.Meta["env"] != "dev" {
		t.Fatalf("unexpected meta: %v", resp.Meta)
	}
}

func TestConnectorCreateInvalidJSON(t *testing.T) {
	_, _, item := ConnectorHandler(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/connector", bytes.NewBufferString(`{invalid`))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConnectorListIntegration(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ConnectorHandler(manager)

	if err := manager.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	enabled := true
	disabled := false
	publicURL := llmtest.ConnectorURL(t, "handler-public-connector")
	if _, _, _, err := manager.CreateConnector(context.Background(), schema.ConnectorInsert{
		URL: publicURL,
		ConnectorMeta: schema.ConnectorMeta{
			Enabled: &enabled,
		},
	}, nil); err != nil {
		t.Fatal(err)
	}
	namespace := "mcp"
	namespacedURL := llmtest.ConnectorURL(t, "handler-namespaced-connector")
	if _, _, _, err := manager.CreateConnector(context.Background(), schema.ConnectorInsert{
		URL: namespacedURL,
		ConnectorMeta: schema.ConnectorMeta{
			Enabled:   &disabled,
			Namespace: &namespace,
		},
	}, nil); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/connector", nil)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ConnectorList
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 || len(resp.Body) != 2 {
		t.Fatalf("expected 2 connectors, got count=%d len=%d", resp.Count, len(resp.Body))
	}
}

func TestConnectorListIntegrationWithFilters(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ConnectorHandler(manager)

	if err := manager.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	enabled := true
	disabled := false
	publicURL := llmtest.ConnectorURL(t, "handler-filter-public-connector")
	if _, _, _, err := manager.CreateConnector(context.Background(), schema.ConnectorInsert{
		URL: publicURL,
		ConnectorMeta: schema.ConnectorMeta{
			Enabled: &enabled,
		},
	}, nil); err != nil {
		t.Fatal(err)
	}
	namespace := "mcp"
	namespacedURL := llmtest.ConnectorURL(t, "handler-filter-namespaced-connector")
	if _, _, _, err := manager.CreateConnector(context.Background(), schema.ConnectorInsert{
		URL: namespacedURL,
		ConnectorMeta: schema.ConnectorMeta{
			Enabled:   &disabled,
			Namespace: &namespace,
		},
	}, nil); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/connector?namespace=mcp&enabled=false", nil)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ConnectorList
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 || len(resp.Body) != 1 {
		t.Fatalf("expected 1 connector, got count=%d len=%d", resp.Count, len(resp.Body))
	}
	if got := resp.Body[0].URL; got != namespacedURL {
		t.Fatalf("expected filtered connector URL %q, got %q", namespacedURL, got)
	}
}

func TestConnectorListInvalidQuery(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ConnectorHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/connector?enabled=notabool", nil)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConnectorGetIntegration(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ConnectorResourceHandler(manager)

	if err := manager.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	rawURL := llmtest.ConnectorURL(t, "handler-get-connector")
	if _, _, _, err := manager.CreateConnector(context.Background(), schema.ConnectorInsert{URL: rawURL}, nil); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/connector/"+url.PathEscape(rawURL), nil)
	r.SetPathValue("url", url.PathEscape(rawURL))
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.Connector
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.URL != rawURL {
		t.Fatalf("expected URL %q, got %q", rawURL, resp.URL)
	}
}

func TestConnectorGetNotFound(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ConnectorResourceHandler(manager)

	if err := manager.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	rawURL := "https://example.com/sse"
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/connector/"+url.PathEscape(rawURL), nil)
	r.SetPathValue("url", url.PathEscape(rawURL))
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConnectorUpdateIntegration(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ConnectorResourceHandler(manager)

	if err := manager.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	rawURL := llmtest.ConnectorURL(t, "handler-update-connector")
	if _, _, _, err := manager.CreateConnector(context.Background(), schema.ConnectorInsert{URL: rawURL}, nil); err != nil {
		t.Fatal(err)
	}

	enabled := false
	namespace := "mcp"
	body, err := json.Marshal(schema.ConnectorMeta{
		Enabled:   &enabled,
		Namespace: &namespace,
		Meta:      schema.ProviderMetaMap{"env": "dev"},
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/connector/"+url.PathEscape(rawURL), bytes.NewReader(body))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.SetPathValue("url", url.PathEscape(rawURL))
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.Connector
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.URL != rawURL {
		t.Fatalf("expected URL %q, got %q", rawURL, resp.URL)
	}
	if types.Value(resp.Enabled) {
		t.Fatal("expected connector to be disabled")
	}
	if types.Value(resp.Namespace) != namespace {
		t.Fatalf("expected namespace %q, got %q", namespace, types.Value(resp.Namespace))
	}
	if resp.Meta["env"] != "dev" {
		t.Fatalf("unexpected meta: %v", resp.Meta)
	}
}

func TestConnectorUpdateInvalidJSON(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ConnectorResourceHandler(manager)

	rawURL := "https://example.com/sse"
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/connector/"+url.PathEscape(rawURL), bytes.NewBufferString(`{invalid`))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.SetPathValue("url", url.PathEscape(rawURL))
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConnectorUpdateNotFound(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ConnectorResourceHandler(manager)

	if err := manager.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	enabled := false
	body, err := json.Marshal(schema.ConnectorMeta{Enabled: &enabled})
	if err != nil {
		t.Fatal(err)
	}

	rawURL := "https://example.com/sse"
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/connector/"+url.PathEscape(rawURL), bytes.NewReader(body))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.SetPathValue("url", url.PathEscape(rawURL))
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConnectorDeleteIntegration(t *testing.T) {
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, createItem := ConnectorHandler(manager)
	_, _, deleteItem := ConnectorResourceHandler(manager)

	if err := manager.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	rawURL := llmtest.ConnectorURL(t, "handler-delete-connector")
	enabled := true
	createBody, err := json.Marshal(schema.ConnectorInsert{
		URL: rawURL,
		ConnectorMeta: schema.ConnectorMeta{
			Enabled: &enabled,
			Groups:  conn.Config.Groups,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	createW := httptest.NewRecorder()
	createR := httptest.NewRequest(http.MethodPost, "/connector", bytes.NewReader(createBody))
	createR.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	createItem.Handler().ServeHTTP(createW, createR)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createW.Code, createW.Body.String())
	}

	deleteW := httptest.NewRecorder()
	deleteR := httptest.NewRequest(http.MethodDelete, "/connector/"+url.PathEscape(rawURL), nil)
	deleteR.SetPathValue("url", url.PathEscape(rawURL))
	deleteItem.Handler().ServeHTTP(deleteW, deleteR)

	if deleteW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", deleteW.Code, deleteW.Body.String())
	}

	var deleted schema.Connector
	if err := json.NewDecoder(deleteW.Body).Decode(&deleted); err != nil {
		t.Fatal(err)
	}
	if deleted.URL != rawURL {
		t.Fatalf("expected deleted URL %q, got %q", rawURL, deleted.URL)
	}
}

func TestConnectorDeleteNotFound(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ConnectorResourceHandler(manager)

	if err := manager.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	rawURL := "https://example.com/sse"
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/connector/"+url.PathEscape(rawURL), nil)
	r.SetPathValue("url", url.PathEscape(rawURL))
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
