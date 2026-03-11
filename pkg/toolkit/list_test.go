package toolkit

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/toolkit/resource"
	"github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// List tools

func Test_List_Tools_001_basic(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "alpha"}, &mockTool{name: "beta"})
	resp, err := tk.List(context.Background(), ListRequest{Type: ListTypeTools})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(resp.Tools))
	}
	if resp.Count != 2 {
		t.Fatalf("expected Count=2, got %d", resp.Count)
	}
}

func Test_List_Tools_002_sorted(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "zebra"}, &mockTool{name: "alpha"})
	resp, err := tk.List(context.Background(), ListRequest{Type: ListTypeTools})
	if err != nil {
		t.Fatal(err)
	}
	// Names are namespace-prefixed, so compare full names
	if resp.Tools[0].Name() > resp.Tools[1].Name() {
		t.Fatalf("expected sorted order, got %q before %q", resp.Tools[0].Name(), resp.Tools[1].Name())
	}
}

func Test_List_Tools_003_name_filter(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "alpha"}, &mockTool{name: "beta"})
	resp, err := tk.List(context.Background(), ListRequest{
		Type: ListTypeTools,
		Name: NamespaceBuiltin + ".alpha",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(resp.Tools))
	}
}

func Test_List_Tools_004_namespace_builtin(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "alpha"})
	resp, err := tk.List(context.Background(), ListRequest{
		Type:      ListTypeTools,
		Namespace: NamespaceBuiltin,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(resp.Tools))
	}
}

func Test_List_Tools_005_unknown_namespace(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "alpha"})
	resp, err := tk.List(context.Background(), ListRequest{
		Type:      ListTypeTools,
		Namespace: "other",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Tools) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(resp.Tools))
	}
}

func Test_List_Tools_006_pagination_limit(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "alpha"}, &mockTool{name: "beta"}, &mockTool{name: "gamma"})
	resp, err := tk.List(context.Background(), ListRequest{
		Type:  ListTypeTools,
		Limit: types.Ptr(uint(2)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected Count=3, got %d", resp.Count)
	}
	if len(resp.Tools) != 2 {
		t.Fatalf("expected 2 tools after limit, got %d", len(resp.Tools))
	}
	if resp.Limit == nil || *resp.Limit != 2 {
		t.Fatalf("expected Limit=2, got %v", resp.Limit)
	}
}

func Test_List_Tools_007_pagination_offset(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "alpha"}, &mockTool{name: "beta"}, &mockTool{name: "gamma"})
	resp, err := tk.List(context.Background(), ListRequest{
		Type:   ListTypeTools,
		Limit:  types.Ptr(uint(10)),
		Offset: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Tools) != 1 {
		t.Fatalf("expected 1 tool after offset, got %d", len(resp.Tools))
	}
}

func Test_List_Tools_008_offset_beyond_count(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "alpha"})
	resp, err := tk.List(context.Background(), ListRequest{
		Type:   ListTypeTools,
		Limit:  types.Ptr(uint(10)),
		Offset: 99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Tools) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(resp.Tools))
	}
}

func Test_List_Tools_009_limit_capped_at_max(t *testing.T) {
	tk, _ := New()
	// Add 3 tools; request limit far above the max — resp.Limit should be
	// min(count, min(requested, listMaxLimit)) = min(3, min(9999, 100)) = 3.
	_ = tk.AddTool(&mockTool{name: "alpha"}, &mockTool{name: "beta"}, &mockTool{name: "gamma"})
	resp, err := tk.List(context.Background(), ListRequest{
		Type:  ListTypeTools,
		Limit: types.Ptr(uint(9999)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Limit == nil || *resp.Limit != 3 {
		t.Fatalf("expected Limit=3 (min of count and max), got %v", resp.Limit)
	}
}

func Test_List_Tools_010_zero_limit_no_pagination(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "alpha"}, &mockTool{name: "beta"})
	resp, err := tk.List(context.Background(), ListRequest{
		Type:  ListTypeTools,
		Limit: types.Ptr(uint(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Limit=0 means no pagination — all items returned.
	if len(resp.Tools) != 2 {
		t.Fatalf("expected 2 tools with limit=0, got %d", len(resp.Tools))
	}
}

func Test_List_Tools_011_nil_limit_no_pagination(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "alpha"}, &mockTool{name: "beta"})
	resp, err := tk.List(context.Background(), ListRequest{Type: ListTypeTools})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Limit != nil {
		t.Fatalf("expected nil Limit, got %v", resp.Limit)
	}
	if len(resp.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(resp.Tools))
	}
}

///////////////////////////////////////////////////////////////////////////////
// List prompts

func Test_List_Prompts_001_basic(t *testing.T) {
	tk, _ := New()
	_ = tk.AddPrompt(&mockPrompt{name: "summarize"}, &mockPrompt{name: "translate"})
	resp, err := tk.List(context.Background(), ListRequest{Type: ListTypePrompts})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(resp.Prompts))
	}
	if resp.Count != 2 {
		t.Fatalf("expected Count=2, got %d", resp.Count)
	}
}

func Test_List_Prompts_002_name_filter(t *testing.T) {
	tk, _ := New()
	_ = tk.AddPrompt(&mockPrompt{name: "summarize"}, &mockPrompt{name: "translate"})
	resp, err := tk.List(context.Background(), ListRequest{
		Type: ListTypePrompts,
		Name: NamespaceBuiltin + ".summarize",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(resp.Prompts))
	}
}

func Test_List_Prompts_003_unknown_namespace(t *testing.T) {
	tk, _ := New()
	_ = tk.AddPrompt(&mockPrompt{name: "summarize"})
	resp, err := tk.List(context.Background(), ListRequest{
		Type:      ListTypePrompts,
		Namespace: "other",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Prompts) != 0 {
		t.Fatalf("expected 0 prompts, got %d", len(resp.Prompts))
	}
}

///////////////////////////////////////////////////////////////////////////////
// List resources

func Test_List_Resources_001_basic(t *testing.T) {
	tk, _ := New()
	r1, _ := resource.Text("greeting", "hello")
	r2, _ := resource.Text("farewell", "goodbye")
	_ = tk.AddResource(r1, r2)
	resp, err := tk.List(context.Background(), ListRequest{Type: ListTypeResources})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resp.Resources))
	}
	if resp.Count != 2 {
		t.Fatalf("expected Count=2, got %d", resp.Count)
	}
}

func Test_List_Resources_002_sorted(t *testing.T) {
	tk, _ := New()
	r1, _ := resource.Text("zzz", "last")
	r2, _ := resource.Text("aaa", "first")
	_ = tk.AddResource(r1, r2)
	resp, err := tk.List(context.Background(), ListRequest{Type: ListTypeResources})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Resources[0].URI() > resp.Resources[1].URI() {
		t.Fatalf("expected sorted order, got %q before %q", resp.Resources[0].URI(), resp.Resources[1].URI())
	}
}

func Test_List_Resources_003_uri_filter(t *testing.T) {
	tk, _ := New()
	r, _ := resource.Text("greeting", "hello")
	_ = tk.AddResource(r)
	resp, err := tk.List(context.Background(), ListRequest{
		Type: ListTypeResources,
		Name: r.URI(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resp.Resources))
	}
}

func Test_List_Resources_004_unknown_namespace(t *testing.T) {
	tk, _ := New()
	r, _ := resource.Text("greeting", "hello")
	_ = tk.AddResource(r)
	resp, err := tk.List(context.Background(), ListRequest{
		Type:      ListTypeResources,
		Namespace: "other",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resp.Resources))
	}
}

func Test_List_Resources_005_pagination(t *testing.T) {
	tk, _ := New()
	r1, _ := resource.Text("aaa", "first")
	r2, _ := resource.Text("bbb", "second")
	r3, _ := resource.Text("ccc", "third")
	_ = tk.AddResource(r1, r2, r3)
	resp, err := tk.List(context.Background(), ListRequest{
		Type:  ListTypeResources,
		Limit: types.Ptr(uint(2)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected Count=3, got %d", resp.Count)
	}
	if len(resp.Resources) != 2 {
		t.Fatalf("expected 2 resources after limit, got %d", len(resp.Resources))
	}
}

///////////////////////////////////////////////////////////////////////////////
// Mixed type listing (no cross-contamination)

func Test_List_Mixed_001_tools_only(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "alpha"})
	_ = tk.AddPrompt(&mockPrompt{name: "summarize"})
	resp, err := tk.List(context.Background(), ListRequest{Type: ListTypeTools})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Tools) != 1 || len(resp.Prompts) != 0 {
		t.Fatalf("expected only tools, got tools=%d prompts=%d", len(resp.Tools), len(resp.Prompts))
	}
}

func Test_List_Mixed_002_count_is_type_specific(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "alpha"}, &mockTool{name: "beta"})
	_ = tk.AddPrompt(&mockPrompt{name: "summarize"})
	resp, err := tk.List(context.Background(), ListRequest{Type: ListTypeTools})
	if err != nil {
		t.Fatal(err)
	}
	// Count is only for the requested type, not combined.
	if resp.Count != 2 {
		t.Fatalf("expected Count=2 (tools only), got %d", resp.Count)
	}
}

func Test_List_Empty_001_no_items(t *testing.T) {
	tk, _ := New()
	for _, lt := range []ListType{ListTypeTools, ListTypePrompts, ListTypeResources} {
		resp, err := tk.List(context.Background(), ListRequest{Type: lt})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Count != 0 {
			t.Fatalf("type %s: expected Count=0, got %d", lt, resp.Count)
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// Offset metadata preserved

func Test_List_Tools_012_offset_preserved(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "alpha"}, &mockTool{name: "beta"})
	resp, err := tk.List(context.Background(), ListRequest{
		Type:   ListTypeTools,
		Limit:  types.Ptr(uint(10)),
		Offset: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Offset != 1 {
		t.Fatalf("expected Offset=1, got %d", resp.Offset)
	}
}

///////////////////////////////////////////////////////////////////////////////
// llm.Resource interface preserved after WithNamespace wrapping

func Test_List_Resources_006_resource_interface(t *testing.T) {
	tk, _ := New()
	r, _ := resource.Text("greeting", "hello")
	_ = tk.AddResource(r)
	resp, err := tk.List(context.Background(), ListRequest{Type: ListTypeResources})
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Resources[0]
	if _, ok := got.(llm.Resource); !ok {
		t.Fatalf("expected llm.Resource, got %T", got)
	}
	// URI should be unchanged by WithNamespace wrapping.
	if got.URI() != r.URI() {
		t.Fatalf("expected URI %q, got %q", r.URI(), got.URI())
	}
}
