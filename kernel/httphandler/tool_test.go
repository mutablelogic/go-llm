package httphandler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	llmmanager "github.com/mutablelogic/go-llm/kernel/manager"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

type toolHandlerMockTool struct {
	name        string
	description string
	input       *jsonschema.Schema
	output      *jsonschema.Schema
	meta        llm.ToolMeta
	run         func(context.Context, json.RawMessage) (any, error)
}

func (t *toolHandlerMockTool) Name() string                     { return t.name }
func (t *toolHandlerMockTool) Description() string              { return t.description }
func (t *toolHandlerMockTool) InputSchema() *jsonschema.Schema  { return t.input }
func (t *toolHandlerMockTool) OutputSchema() *jsonschema.Schema { return t.output }
func (t *toolHandlerMockTool) Meta() llm.ToolMeta               { return t.meta }
func (t *toolHandlerMockTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	if t.run != nil {
		return t.run(ctx, input)
	}
	return nil, nil
}

func TestToolList(t *testing.T) {
	manager := &llmmanager.Manager{Toolkit: mustToolToolkit(t)}
	_, _, item := ToolHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/tools?limit=2", nil)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ToolList
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected count=3, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(resp.Body))
	}
	if resp.Body[0].Name != "builtin.alpha" {
		t.Fatalf("expected first tool %q, got %q", "builtin.alpha", resp.Body[0].Name)
	}
	if resp.Body[0].Title != "Alpha Tool" {
		t.Fatalf("expected title %q, got %q", "Alpha Tool", resp.Body[0].Title)
	}
	if len(resp.Body[0].Hints) != 1 || resp.Body[0].Hints[0] != "readonly" {
		t.Fatalf("unexpected hints: %+v", resp.Body[0].Hints)
	}
	if string(resp.Body[0].Input) == "" {
		t.Fatal("expected input schema in response")
	}
	if string(resp.Body[0].Output) == "" {
		t.Fatal("expected output schema in response")
	}
	if resp.Body[1].Name != "builtin.bravo" {
		t.Fatalf("expected second tool %q, got %q", "builtin.bravo", resp.Body[1].Name)
	}
}

func TestToolListInvalidQuery(t *testing.T) {
	_, _, item := ToolHandler(&llmmanager.Manager{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/tools?limit=invalid", nil)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestToolListWithFilters(t *testing.T) {
	manager := &llmmanager.Manager{Toolkit: mustToolToolkit(t)}
	_, _, item := ToolHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/tool?namespace=builtin&name=builtin.bravo&name=builtin.alpha", nil)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ToolList
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 || len(resp.Body) != 2 {
		t.Fatalf("expected 2 filtered tools, got count=%d len=%d", resp.Count, len(resp.Body))
	}
	if resp.Body[0].Name != "builtin.alpha" || resp.Body[1].Name != "builtin.bravo" {
		t.Fatalf("unexpected filtered tools: %+v", resp.Body)
	}
}

func TestGetTool(t *testing.T) {
	tk, err := toolkit.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := tk.AddTool(
		&toolHandlerMockTool{name: "alpha", description: "A", input: jsonschema.MustFor[map[string]any](), output: jsonschema.MustFor[string](), meta: llm.ToolMeta{Title: "Alpha Tool", ReadOnlyHint: true}},
	); err != nil {
		t.Fatal(err)
	}

	manager := &llmmanager.Manager{Toolkit: tk}
	_, _, item := ToolResourceHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/tool/builtin.alpha", nil)
	r.SetPathValue("name", "builtin.alpha")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ToolMeta
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Name != "builtin.alpha" {
		t.Fatalf("expected tool %q, got %q", "builtin.alpha", resp.Name)
	}
	if resp.Title != "Alpha Tool" {
		t.Fatalf("expected title %q, got %q", "Alpha Tool", resp.Title)
	}
	if string(resp.Input) == "" || string(resp.Output) == "" {
		t.Fatal("expected tool schemas in response")
	}
}

func TestGetToolNotFound(t *testing.T) {
	manager := &llmmanager.Manager{Toolkit: mustToolToolkit(t)}
	_, _, item := ToolResourceHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/tool/builtin.missing", nil)
	r.SetPathValue("name", "builtin.missing")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetToolUnescapesName(t *testing.T) {
	manager := &llmmanager.Manager{Toolkit: mustToolToolkit(t)}
	_, _, item := ToolResourceHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/tool/builtin%2Ealpha", nil)
	r.SetPathValue("name", "builtin%2Ealpha")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ToolMeta
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Name != "builtin.alpha" {
		t.Fatalf("expected unescaped tool name %q, got %q", "builtin.alpha", resp.Name)
	}
}

func TestCallTool(t *testing.T) {
	tk, err := toolkit.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := tk.AddTool(
		&toolHandlerMockTool{
			name:   "alpha",
			input:  jsonschema.MustFor[map[string]any](),
			output: jsonschema.MustFor[map[string]any](),
			run: func(_ context.Context, input json.RawMessage) (any, error) {
				return input, nil
			},
		},
	); err != nil {
		t.Fatal(err)
	}

	manager := &llmmanager.Manager{Toolkit: tk}
	_, _, item := ToolResourceHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/tool/builtin.alpha", bytes.NewReader([]byte(`{"input":{"query":"docs"}}`)))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.SetPathValue("name", "builtin.alpha")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get(types.ContentTypeHeader); got != types.ContentTypeJSON {
		t.Fatalf("expected content type %q, got %q", types.ContentTypeJSON, got)
	}
	if got := w.Header().Get(types.ContentPathHeader); got != "json:alpha" {
		t.Fatalf("expected content path %q, got %q", "json:alpha", got)
	}
	if got := w.Header().Get(types.ContentNameHeader); got != "alpha" {
		t.Fatalf("expected content name %q, got %q", "alpha", got)
	}
	if got := w.Header().Get(types.ContentDescriptionHeader); got != "" {
		t.Fatalf("expected empty content description, got %q", got)
	}
	if got := w.Body.String(); got != `{"query":"docs"}` {
		t.Fatalf("unexpected response body: %s", got)
	}
}

func TestCallToolNoContent(t *testing.T) {
	manager := &llmmanager.Manager{Toolkit: mustToolToolkit(t)}
	_, _, item := ToolResourceHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/tool/builtin.bravo", bytes.NewReader([]byte(`{"input":{}}`)))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.SetPathValue("name", "builtin.bravo")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", w.Body.String())
	}
}

func TestCallToolInvalidBody(t *testing.T) {
	manager := &llmmanager.Manager{Toolkit: mustToolToolkit(t)}
	_, _, item := ToolResourceHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/tool/builtin.alpha", bytes.NewReader([]byte(`{"input":`)))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.SetPathValue("name", "builtin.alpha")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCallToolUnescapesName(t *testing.T) {
	tk, err := toolkit.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := tk.AddTool(
		&toolHandlerMockTool{
			name:   "alpha",
			input:  jsonschema.MustFor[map[string]any](),
			output: jsonschema.MustFor[map[string]any](),
			run: func(_ context.Context, input json.RawMessage) (any, error) {
				return input, nil
			},
		},
	); err != nil {
		t.Fatal(err)
	}

	manager := &llmmanager.Manager{Toolkit: tk}
	_, _, item := ToolResourceHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/tool/builtin%2Ealpha", bytes.NewReader([]byte(`{"input":{"query":"docs"}}`)))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.SetPathValue("name", "builtin%2Ealpha")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != `{"query":"docs"}` {
		t.Fatalf("unexpected response body: %s", got)
	}
}

func mustToolToolkit(t *testing.T) toolkit.Toolkit {
	t.Helper()
	tk, err := toolkit.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := tk.AddTool(
		&toolHandlerMockTool{name: "alpha", description: "A", input: jsonschema.MustFor[map[string]any](), output: jsonschema.MustFor[string](), meta: llm.ToolMeta{Title: "Alpha Tool", ReadOnlyHint: true}},
		&toolHandlerMockTool{name: "bravo", description: "B"},
		&toolHandlerMockTool{name: "charlie", description: "C"},
	); err != nil {
		t.Fatal(err)
	}
	return tk
}
