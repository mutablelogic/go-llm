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
// TOOL LIST TESTS

func TestToolList_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1"},
	},
		&mockTool{name: "tool_alpha", description: "Alpha tool"},
		&mockTool{name: "tool_beta", description: "Beta tool"},
	)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/tool", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp schema.ListToolResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected count=2, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(resp.Body))
	}
	// Sorted by name
	if resp.Body[0].Name != "tool_alpha" {
		t.Fatalf("expected first tool=tool_alpha, got %q", resp.Body[0].Name)
	}
}

func TestToolList_WithPagination(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1"},
	},
		&mockTool{name: "tool_a"},
		&mockTool{name: "tool_b"},
		&mockTool{name: "tool_c"},
	)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/tool?limit=1&offset=1", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp schema.ListToolResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected count=3, got %d", resp.Count)
	}
	if len(resp.Body) != 1 {
		t.Fatalf("expected 1 tool in page, got %d", len(resp.Body))
	}
	if resp.Body[0].Name != "tool_b" {
		t.Fatalf("expected tool_b, got %q", resp.Body[0].Name)
	}
}

func TestToolList_Empty(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1"},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/tool", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp schema.ListToolResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 0 {
		t.Fatalf("expected count=0, got %d", resp.Count)
	}
}

///////////////////////////////////////////////////////////////////////////////
// TOOL GET TESTS

func TestToolGet_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1"},
	},
		&mockTool{name: "my_tool", description: "A test tool"},
	)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/tool/my_tool", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var meta schema.ToolMeta
	if err := json.NewDecoder(w.Body).Decode(&meta); err != nil {
		t.Fatal(err)
	}
	if meta.Name != "my_tool" {
		t.Fatalf("expected name=my_tool, got %q", meta.Name)
	}
	if meta.Description != "A test tool" {
		t.Fatalf("expected description='A test tool', got %q", meta.Description)
	}
}

func TestToolGet_NotFound(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1"},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/tool/nonexistent", nil)
	mux.ServeHTTP(w, r)

	// llm.ErrNotFound maps to 404 via httpErr
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestToolGet_MethodNotAllowed(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1"},
	},
		&mockTool{name: "my_tool"},
	)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/tool/my_tool", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}
