package httphandler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	manager "github.com/mutablelogic/go-llm/pkg/manager"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

func newEmbedderManager(t *testing.T) *manager.Manager {
	t.Helper()
	client := &mockEmbedderClient{
		mockClient: mockClient{
			name:   "embedder",
			models: []schema.Model{{Name: "embed-model"}},
		},
	}
	m, err := manager.NewManager(manager.WithClient(client))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestEmbedding_OK(t *testing.T) {
	mgr := newEmbedderManager(t)
	mux := serveMux(mgr)

	body := `{"model":"embed-model","input":["hello world"]}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/embedding", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.EmbeddingResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Model != "embed-model" {
		t.Fatalf("expected model=embed-model, got %q", resp.Model)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 embedding, got %d", len(resp.Output))
	}
	if len(resp.Output[0]) != 3 {
		t.Fatalf("expected 3 dimensions, got %d", len(resp.Output[0]))
	}
}

func TestEmbedding_WithProvider(t *testing.T) {
	mgr := newEmbedderManager(t)
	mux := serveMux(mgr)

	body := `{"provider":"embedder","model":"embed-model","input":["hello"]}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/embedding", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.EmbeddingResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Provider != "embedder" {
		t.Fatalf("expected provider=embedder, got %q", resp.Provider)
	}
}

func TestEmbedding_BatchInput(t *testing.T) {
	mgr := newEmbedderManager(t)
	mux := serveMux(mgr)

	body := `{"model":"embed-model","input":["hello","world","foo"]}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/embedding", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.EmbeddingResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Output) != 3 {
		t.Fatalf("expected 3 embeddings, got %d", len(resp.Output))
	}
}

func TestEmbedding_EmptyInput(t *testing.T) {
	mgr := newEmbedderManager(t)
	mux := serveMux(mgr)

	body := `{"model":"embed-model","input":[]}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/embedding", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEmbedding_ModelNotFound(t *testing.T) {
	mgr := newEmbedderManager(t)
	mux := serveMux(mgr)

	body := `{"model":"nonexistent","input":["hello"]}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/embedding", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEmbedding_MethodNotAllowed(t *testing.T) {
	mgr := newEmbedderManager(t)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/embedding", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestEmbedding_InvalidJSON(t *testing.T) {
	mgr := newEmbedderManager(t)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/embedding", bytes.NewBufferString("{invalid"))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
