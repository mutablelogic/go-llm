package httphandler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// MODEL LIST TESTS

func TestModelList_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}, {Name: "model-b"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/model", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp schema.ListModelsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected count=2, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 models, got %d", len(resp.Body))
	}
}

func TestModelList_WithPagination(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "a"}, {Name: "b"}, {Name: "c"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/model?limit=2&offset=1", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp schema.ListModelsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected count=3, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 models in page, got %d", len(resp.Body))
	}
	if resp.Body[0].Name != "b" {
		t.Fatalf("expected first model=b, got %q", resp.Body[0].Name)
	}
}

func TestModelList_MethodNotAllowed(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1"},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/model", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

///////////////////////////////////////////////////////////////////////////////
// MODEL GET TESTS

func TestModelGet_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4", OwnedBy: "provider-1"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/model/gpt-4", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var model schema.Model
	if err := json.NewDecoder(w.Body).Decode(&model); err != nil {
		t.Fatal(err)
	}
	if model.Name != "gpt-4" {
		t.Fatalf("expected name=gpt-4, got %q", model.Name)
	}
}

func TestModelGet_WithProvider(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-x"}}},
		{name: "provider-2", models: []schema.Model{{Name: "model-x"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/model/provider-1/model-x", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var model schema.Model
	if err := json.NewDecoder(w.Body).Decode(&model); err != nil {
		t.Fatal(err)
	}
	if model.Name != "model-x" {
		t.Fatalf("expected name=model-x, got %q", model.Name)
	}
}

func TestModelGet_NotFound(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/model/nonexistent", nil)
	mux.ServeHTTP(w, r)

	// llm.ErrNotFound maps to 404 via httpErr
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
