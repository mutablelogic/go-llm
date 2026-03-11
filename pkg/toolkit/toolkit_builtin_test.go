package toolkit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	resource "github.com/mutablelogic/go-llm/pkg/toolkit/resource"
	toolpkg "github.com/mutablelogic/go-llm/pkg/toolkit/tool"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK TYPES

type mockTool struct {
	name string
}

func (m *mockTool) Name() string                                          { return m.name }
func (m *mockTool) Description() string                                   { return "mock tool " + m.name }
func (m *mockTool) InputSchema() (*jsonschema.Schema, error)              { return nil, nil }
func (m *mockTool) OutputSchema() (*jsonschema.Schema, error)             { return nil, nil }
func (m *mockTool) Meta() llm.ToolMeta                                    { return llm.ToolMeta{} }
func (m *mockTool) Run(_ context.Context, _ json.RawMessage) (any, error) { return nil, nil }

type mockPrompt struct {
	name string
}

func (m *mockPrompt) Name() string        { return m.name }
func (m *mockPrompt) Title() string       { return "mock prompt " + m.name }
func (m *mockPrompt) Description() string { return "" }

///////////////////////////////////////////////////////////////////////////////
// AddTool

func Test_AddTool_001(t *testing.T) {
	tk, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := tk.AddTool(&mockTool{name: "my_tool"}); err != nil {
		t.Fatal(err)
	}
	if len(tk.tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tk.tools))
	}
}

func Test_AddTool_002_invalid_name(t *testing.T) {
	tk, _ := New()
	if err := tk.AddTool(&mockTool{name: "bad name!"}); !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_AddTool_003_reserved_name(t *testing.T) {
	tk, _ := New()
	if err := tk.AddTool(&mockTool{name: "submit_output"}); !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_AddTool_004_duplicate(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "my_tool"})
	if err := tk.AddTool(&mockTool{name: "my_tool"}); !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_AddTool_005_namespace_prefixed(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "my_tool"})
	t.Log(tk.tools["my_tool"].Name())
	if tk.tools["my_tool"].Name() != NamespaceBuiltin+".my_tool" {
		t.Fatalf("expected namespaced name, got %q", tk.tools["my_tool"].Name())
	}
}

///////////////////////////////////////////////////////////////////////////////
// AddPrompt

func Test_AddPrompt_001(t *testing.T) {
	tk, _ := New()
	if err := tk.AddPrompt(&mockPrompt{name: "summarize"}); err != nil {
		t.Fatal(err)
	}
	if len(tk.prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(tk.prompts))
	}
}

func Test_AddPrompt_002_invalid_name(t *testing.T) {
	tk, _ := New()
	if err := tk.AddPrompt(&mockPrompt{name: "bad name!"}); !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_AddPrompt_003_duplicate(t *testing.T) {
	tk, _ := New()
	_ = tk.AddPrompt(&mockPrompt{name: "summarize"})
	if err := tk.AddPrompt(&mockPrompt{name: "summarize"}); !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// AddResource

func Test_AddResource_001(t *testing.T) {
	tk, _ := New()
	r, err := resource.Text("greeting", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if err := tk.AddResource(r); err != nil {
		t.Fatal(err)
	}
	if len(tk.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(tk.resources))
	}
}

func Test_AddResource_002_duplicate(t *testing.T) {
	tk, _ := New()
	r, _ := resource.Text("greeting", "hello")
	_ = tk.AddResource(r)
	if err := tk.AddResource(r); !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// RemoveBuiltin

func Test_RemoveBuiltin_001_tool(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "my_tool"})
	if err := tk.RemoveBuiltin("my_tool"); err != nil {
		t.Fatal(err)
	}
	if len(tk.tools) != 0 {
		t.Fatal("expected tool to be removed")
	}
}

func Test_RemoveBuiltin_002_prompt(t *testing.T) {
	tk, _ := New()
	_ = tk.AddPrompt(&mockPrompt{name: "summarize"})
	if err := tk.RemoveBuiltin("summarize"); err != nil {
		t.Fatal(err)
	}
	if len(tk.prompts) != 0 {
		t.Fatal("expected prompt to be removed")
	}
}

func Test_RemoveBuiltin_003_resource(t *testing.T) {
	tk, _ := New()
	r, _ := resource.Text("greeting", "hello")
	_ = tk.AddResource(r)
	if err := tk.RemoveBuiltin(r.URI()); err != nil {
		t.Fatal(err)
	}
	if len(tk.resources) != 0 {
		t.Fatal("expected resource to be removed")
	}
}

func Test_RemoveBuiltin_004_not_found(t *testing.T) {
	tk, _ := New()
	if err := tk.RemoveBuiltin("nonexistent"); !errors.Is(err, llm.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func Test_RemoveBuiltin_005_tool_precedence_over_prompt(t *testing.T) {
	// When a tool and prompt share the same bare name (shouldn't happen in practice,
	// but RemoveBuiltin removes the tool first).
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "overlap"})
	_ = tk.AddPrompt(&mockPrompt{name: "other"})
	if err := tk.RemoveBuiltin("overlap"); err != nil {
		t.Fatal(err)
	}
	if len(tk.tools) != 0 {
		t.Fatal("expected tool to be removed")
	}
}

///////////////////////////////////////////////////////////////////////////////
// Lookup

func Test_Lookup_001_tool_by_name(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "my_tool"})
	v, err := tk.Lookup(context.Background(), "my_tool")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(llm.Tool); !ok {
		t.Fatalf("expected llm.Tool, got %T", v)
	}
}

func Test_Lookup_002_prompt_by_name(t *testing.T) {
	tk, _ := New()
	_ = tk.AddPrompt(&mockPrompt{name: "summarize"})
	v, err := tk.Lookup(context.Background(), "summarize")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(llm.Prompt); !ok {
		t.Fatalf("expected llm.Prompt, got %T", v)
	}
}

func Test_Lookup_003_tool_by_namespace_name(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "my_tool"})
	v, err := tk.Lookup(context.Background(), NamespaceBuiltin+".my_tool")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(llm.Tool); !ok {
		t.Fatalf("expected llm.Tool, got %T", v)
	}
}

func Test_Lookup_004_resource_by_uri(t *testing.T) {
	tk, _ := New()
	r, _ := resource.Text("greeting", "hello")
	_ = tk.AddResource(r)
	v, err := tk.Lookup(context.Background(), r.URI())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(llm.Resource); !ok {
		t.Fatalf("expected llm.Resource, got %T", v)
	}
}

func Test_Lookup_005_tool_precedence_over_prompt(t *testing.T) {
	// Both tool and prompt registered with same bare name; tool must win.
	tk := &toolkit{
		tools:     make(map[string]llm.Tool),
		prompts:   make(map[string]llm.Prompt),
		resources: make(map[string]llm.Resource),
	}
	tk.tools["clash"] = &mockTool{name: "clash"}
	tk.prompts["clash"] = &mockPrompt{name: "clash"}
	v, err := tk.Lookup(context.Background(), "clash")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(llm.Tool); !ok {
		t.Fatalf("expected llm.Tool, got %T", v)
	}
}

func Test_Lookup_006_not_found(t *testing.T) {
	tk, _ := New()
	_, err := tk.Lookup(context.Background(), "nonexistent")
	if !errors.Is(err, llm.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func Test_Lookup_007_invalid_key(t *testing.T) {
	tk, _ := New()
	_, err := tk.Lookup(context.Background(), "bad key!")
	if !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_Lookup_008_unknown_namespace(t *testing.T) {
	tk, _ := New()
	_ = tk.AddTool(&mockTool{name: "my_tool"})
	// "other" namespace is not builtin, so nothing should match.
	_, err := tk.Lookup(context.Background(), "other.my_tool")
	if !errors.Is(err, llm.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func Test_Lookup_009_resource_wrong_namespace(t *testing.T) {
	tk, _ := New()
	r, _ := resource.Text("greeting", "hello")
	_ = tk.AddResource(r)
	// Append a non-builtin fragment; should return ErrNotFound.
	_, err := tk.Lookup(context.Background(), r.URI()+"#unknown")
	if !errors.Is(err, llm.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func Test_Lookup_010_output_tool_by_name(t *testing.T) {
	tk, _ := New()
	v, err := tk.Lookup(context.Background(), toolpkg.OutputToolName)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(llm.Tool); !ok {
		t.Fatalf("expected llm.Tool, got %T", v)
	}
}

func Test_Lookup_011_output_tool_by_namespace_name(t *testing.T) {
	tk, _ := New()
	v, err := tk.Lookup(context.Background(), NamespaceBuiltin+"."+toolpkg.OutputToolName)
	if err != nil {
		t.Fatal(err)
	}
	tool, ok := v.(llm.Tool)
	if !ok {
		t.Fatalf("expected llm.Tool, got %T", v)
	}
	if tool.Name() != NamespaceBuiltin+"."+toolpkg.OutputToolName {
		t.Fatalf("expected namespaced name, got %q", tool.Name())
	}
}

///////////////////////////////////////////////////////////////////////////////
// New

func Test_New_001_option_error(t *testing.T) {
	// An invalid tool name inside an option should cause New to return an error.
	_, err := New(WithTool(&mockTool{name: "bad name!"}))
	if !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// AddTool (additional coverage)

func Test_AddTool_006_variadic_duplicate(t *testing.T) {
	// Passing the same-named tool twice in a single variadic call hits the seen map.
	tk, _ := New()
	a := &mockTool{name: "dup"}
	if err := tk.AddTool(a, a); !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_AddTool_007_nil_input(t *testing.T) {
	// nil entries should be silently skipped.
	tk, _ := New()
	if err := tk.AddTool(nil, &mockTool{name: "my_tool"}); err != nil {
		t.Fatal(err)
	}
	if len(tk.tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tk.tools))
	}
}

///////////////////////////////////////////////////////////////////////////////
// AddPrompt (additional coverage)

func Test_AddPrompt_004_variadic_duplicate(t *testing.T) {
	// Passing the same-named prompt twice in a single variadic call hits the seen map.
	tk, _ := New()
	p := &mockPrompt{name: "dup"}
	if err := tk.AddPrompt(p, p); !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_AddPrompt_005_nil_input(t *testing.T) {
	// nil entries should be silently skipped.
	tk, _ := New()
	if err := tk.AddPrompt(nil, &mockPrompt{name: "summarize"}); err != nil {
		t.Fatal(err)
	}
	if len(tk.prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(tk.prompts))
	}
}

///////////////////////////////////////////////////////////////////////////////
// AddResource (additional coverage)

func Test_AddResource_003_variadic_duplicate(t *testing.T) {
	// Passing the same resource twice in a single variadic call hits the seen map.
	tk, _ := New()
	r, _ := resource.Text("greeting", "hello")
	if err := tk.AddResource(r, r); !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}
