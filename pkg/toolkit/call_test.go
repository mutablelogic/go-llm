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
)

///////////////////////////////////////////////////////////////////////////////
// MOCK TYPES for call tests

// callableTool controls what Run returns.
type callableTool struct {
	name         string
	result       any
	err          error
	inputSchema  *jsonschema.Schema
	outputSchema *jsonschema.Schema
}

func (m *callableTool) Name() string        { return m.name }
func (m *callableTool) Description() string { return "callable tool " + m.name }
func (m *callableTool) InputSchema() (*jsonschema.Schema, error) {
	return m.inputSchema, nil
}
func (m *callableTool) OutputSchema() (*jsonschema.Schema, error) {
	return m.outputSchema, nil
}
func (m *callableTool) Meta() llm.ToolMeta                                    { return llm.ToolMeta{} }
func (m *callableTool) Run(_ context.Context, _ json.RawMessage) (any, error) { return m.result, m.err }

///////////////////////////////////////////////////////////////////////////////
// Call — bad key type

func Test_Call_001_bad_key_type(t *testing.T) {
	tk, _ := New()
	_, err := tk.Call(context.Background(), 42)
	if !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// Call — string key

func Test_Call_002_string_key_not_found(t *testing.T) {
	tk, _ := New()
	_, err := tk.Call(context.Background(), "nonexistent")
	if !errors.Is(err, llm.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func Test_Call_003_string_key_tool(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet", result: "hello"}
	_ = tk.AddTool(tool)
	res, err := tk.Call(context.Background(), "greet")
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}
}

///////////////////////////////////////////////////////////////////////////////
// Call — direct llm.Tool / llm.Prompt

func Test_Call_004_direct_tool(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet", result: "hello"}
	res, err := tk.Call(context.Background(), tool)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}
}

func Test_Call_005_direct_prompt_no_delegate(t *testing.T) {
	tk, _ := New()
	_, err := tk.Call(context.Background(), &mockPrompt{name: "summarize"})
	if !errors.Is(err, llm.ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
}

func Test_Call_006_direct_prompt_with_delegate(t *testing.T) {
	d := &mockDelegate{}
	tk, _ := New(WithDelegate(d))
	p := &mockPrompt{name: "summarize"}
	// mockDelegate.Call returns nil, nil — that is a valid (nil resource) result.
	_, err := tk.Call(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// callTool — return type variants

func Test_Call_007_tool_returns_nil_no_output_schema(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "noop", result: nil}
	res, err := tk.Call(context.Background(), tool)
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Fatalf("expected nil resource, got %v", res)
	}
}

func Test_Call_008_tool_returns_string(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet", result: "hello world"}
	res, err := tk.Call(context.Background(), tool)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := res.Read(context.Background())
	if string(data) != "hello world" {
		t.Fatalf("expected %q, got %q", "hello world", string(data))
	}
}

func Test_Call_009_tool_returns_bytes(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet", result: []byte("hello bytes")}
	res, err := tk.Call(context.Background(), tool)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := res.Read(context.Background())
	if string(data) != "hello bytes" {
		t.Fatalf("expected %q, got %q", "hello bytes", string(data))
	}
}

func Test_Call_010_tool_returns_raw_json(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet", result: json.RawMessage(`{"ok":true}`)}
	res, err := tk.Call(context.Background(), tool)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}
}

func Test_Call_011_tool_returns_resource(t *testing.T) {
	tk, _ := New()
	inner, _ := resource.Text("inner", "content")
	tool := &callableTool{name: "greet", result: inner}
	res, err := tk.Call(context.Background(), tool)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}
}

func Test_Call_012_tool_returns_bad_type(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet", result: 12345}
	_, err := tk.Call(context.Background(), tool)
	if !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_Call_013_tool_propagates_run_error(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "fail", err: errors.New("boom")}
	_, err := tk.Call(context.Background(), tool)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected 'boom', got %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// callTool — resource input validation

func Test_Call_014_too_many_resources(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet"}
	r1, _ := resource.Text("a", "x")
	r2, _ := resource.Text("b", "y")
	_, err := tk.Call(context.Background(), tool, r1, r2)
	if !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_Call_015_nil_resource(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet"}
	_, err := tk.Call(context.Background(), tool, nil)
	if !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_Call_016_non_json_resource(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet"}
	r, _ := resource.Text("input", "not json")
	_, err := tk.Call(context.Background(), tool, r)
	if !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_Call_017_json_resource_input(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet", result: "ok"}
	r, _ := resource.JSON("input", map[string]string{"name": "world"})
	res, err := tk.Call(context.Background(), tool, r)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}
}
