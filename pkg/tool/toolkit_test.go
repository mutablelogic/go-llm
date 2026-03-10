package tool_test

import (
	"context"
	"encoding/json"
	"testing"

	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// Helpers

type stubTool struct {
	name string
}

func (s *stubTool) Name() string                                          { return s.name }
func (s *stubTool) Description() string                                   { return "stub" }
func (s *stubTool) InputSchema() (*jsonschema.Schema, error)              { return nil, nil }
func (s *stubTool) OutputSchema() (*jsonschema.Schema, error)             { return nil, nil }
func (s *stubTool) Meta() llm.ToolMeta                                    { return llm.ToolMeta{} }
func (s *stubTool) Run(_ context.Context, _ json.RawMessage) (any, error) { return nil, nil }

// echoTool echoes its raw input as output.
type echoTool struct{ name string }

func (e *echoTool) Name() string                              { return e.name }
func (e *echoTool) Description() string                       { return "echo" }
func (e *echoTool) InputSchema() (*jsonschema.Schema, error)  { return nil, nil }
func (e *echoTool) OutputSchema() (*jsonschema.Schema, error) { return nil, nil }
func (e *echoTool) Meta() llm.ToolMeta                        { return llm.ToolMeta{} }
func (e *echoTool) Run(_ context.Context, input json.RawMessage) (any, error) {
	return input, nil
}

// schemaTool is a tool that has a JSON schema for input validation.
type schemaTool struct {
	name   string
	schema *jsonschema.Schema
}

func (s *schemaTool) Name() string                              { return s.name }
func (s *schemaTool) Description() string                       { return "schema tool" }
func (s *schemaTool) InputSchema() (*jsonschema.Schema, error)  { return s.schema, nil }
func (s *schemaTool) OutputSchema() (*jsonschema.Schema, error) { return nil, nil }
func (s *schemaTool) Meta() llm.ToolMeta                        { return llm.ToolMeta{} }
func (s *schemaTool) Run(_ context.Context, input json.RawMessage) (any, error) {
	return input, nil
}

///////////////////////////////////////////////////////////////////////////////
// TestAddBuiltin

func TestRegister_ReservedName(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()
	err = tk.AddBuiltin(&stubTool{name: tool.OutputToolName})
	if err == nil {
		t.Fatal("expected error when registering a tool with reserved name")
	}
	t.Log("got expected error:", err)
}

func TestRegister_OutputToolAllowed(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()
	outputTool := tool.NewOutputTool(&jsonschema.Schema{})
	if err := tk.AddBuiltin(outputTool); err != nil {
		t.Fatal("OutputTool should be allowed:", err)
	}
}

func TestRegister_NormalToolOK(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()
	if err := tk.AddBuiltin(&stubTool{name: "my_tool"}); err != nil {
		t.Fatal("normal tool should register:", err)
	}
}

func TestAddBuiltin_Duplicate(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddBuiltin(&stubTool{name: "dup"}); err != nil {
		t.Fatal(err)
	}
	if err := tk.AddBuiltin(&stubTool{name: "dup"}); err == nil {
		t.Fatal("expected error for duplicate tool name")
	}
}

func TestAddBuiltin_InvalidName(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddBuiltin(&stubTool{name: "invalid name!"}); err == nil {
		t.Fatal("expected error for invalid tool name")
	}
}

///////////////////////////////////////////////////////////////////////////////
// TestOutputTool_Validate

func TestOutputTool_ValidateValid(t *testing.T) {
	s := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"summary":   {Type: "string"},
			"sentiment": {Type: "string"},
		},
		Required: []string{"summary", "sentiment"},
	}
	ot := tool.NewOutputTool(s)
	valid := json.RawMessage(`{"summary":"hello","sentiment":"positive"}`)
	if err := ot.Validate(valid); err != nil {
		t.Fatal("expected valid data to pass:", err)
	}
}

func TestOutputTool_ValidateMissingRequired(t *testing.T) {
	s := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"summary":   {Type: "string"},
			"sentiment": {Type: "string"},
		},
		Required: []string{"summary", "sentiment"},
	}
	ot := tool.NewOutputTool(s)
	invalid := json.RawMessage(`{"summary":"hello"}`)
	if err := ot.Validate(invalid); err == nil {
		t.Fatal("expected error for missing required field")
	} else {
		t.Log("got expected error:", err)
	}
}

func TestOutputTool_ValidateWrongType(t *testing.T) {
	s := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"count": {Type: "integer"},
		},
	}
	ot := tool.NewOutputTool(s)
	invalid := json.RawMessage(`{"count":"not a number"}`)
	if err := ot.Validate(invalid); err == nil {
		t.Fatal("expected error for wrong type")
	} else {
		t.Log("got expected error:", err)
	}
}

func TestOutputTool_ValidateInvalidJSON(t *testing.T) {
	ot := tool.NewOutputTool(&jsonschema.Schema{Type: "object"})
	if err := ot.Validate(json.RawMessage(`{not json`)); err == nil {
		t.Fatal("expected error for invalid JSON")
	} else {
		t.Log("got expected error:", err)
	}
}

func TestOutputTool_ValidateNilSchema(t *testing.T) {
	ot := tool.NewOutputTool(nil)
	if err := ot.Validate(json.RawMessage(`{"anything":"goes"}`)); err != nil {
		t.Fatal("nil schema should accept anything:", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// TestListTools

func TestListTools_Empty(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	tools := tk.ListTools(schema.ListToolsRequest{})
	if len(tools) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(tools))
	}
}

func TestListTools_Builtins(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddBuiltin(&stubTool{name: "alpha"}, &stubTool{name: "beta"}); err != nil {
		t.Fatal(err)
	}

	tools := tk.ListTools(schema.ListToolsRequest{})
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
}

func TestListTools_BuiltinNamespace(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddBuiltin(&stubTool{name: "alpha"}); err != nil {
		t.Fatal(err)
	}

	// Builtin namespace should return the tool
	tools := tk.ListTools(schema.ListToolsRequest{Namespace: schema.BuiltinNamespace})
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool in builtin namespace, got %d", len(tools))
	}
}

func TestListTools_NameFilter(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddBuiltin(&stubTool{name: "alpha"}, &stubTool{name: "beta"}); err != nil {
		t.Fatal(err)
	}

	tools := tk.ListTools(schema.ListToolsRequest{Name: []string{"alpha"}})
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool with name filter, got %d", len(tools))
	}
	if tools[0].Name() != "alpha" {
		t.Fatalf("expected tool 'alpha', got %q", tools[0].Name())
	}
}

func TestListTools_OutputToolExcluded(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	// OutputTool is allowed to be registered but excluded from ListTools
	if err := tk.AddBuiltin(tool.NewOutputTool(nil), &stubTool{name: "visible"}); err != nil {
		t.Fatal(err)
	}

	tools := tk.ListTools(schema.ListToolsRequest{})
	for _, t2 := range tools {
		if t2.Name() == tool.OutputToolName {
			t.Fatal("OutputTool should not appear in ListTools results")
		}
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 visible tool, got %d", len(tools))
	}
}

///////////////////////////////////////////////////////////////////////////////
// TestLookup

func TestLookup_Found(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddBuiltin(&stubTool{name: "my_tool"}); err != nil {
		t.Fatal(err)
	}

	found := tk.Lookup("my_tool")
	if found == nil {
		t.Fatal("expected to find 'my_tool'")
	}
	if found.Name() != "my_tool" {
		t.Fatalf("unexpected tool name: %q", found.Name())
	}
}

func TestLookup_NotFound(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	found := tk.Lookup("nonexistent")
	if found != nil {
		t.Fatal("expected nil for nonexistent tool")
	}
}

///////////////////////////////////////////////////////////////////////////////
// TestRun

func TestRun_NotFound(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	_, err = tk.Run(context.Background(), "no_such_tool", nil)
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

func TestRun_NilInput(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddBuiltin(&stubTool{name: "my_tool"}); err != nil {
		t.Fatal(err)
	}

	result, err := tk.Run(context.Background(), "my_tool", nil)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if result != nil {
		t.Fatalf("expected nil result from stub, got %v", result)
	}
}

func TestRun_JSONRawMessage(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddBuiltin(&echoTool{name: "echo"}); err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"key":"value"}`)
	result, err := tk.Run(context.Background(), "echo", input)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	raw, ok := result.(json.RawMessage)
	if !ok {
		t.Fatalf("expected json.RawMessage, got %T", result)
	}
	if string(raw) != string(input) {
		t.Fatalf("expected %q, got %q", string(input), string(raw))
	}
}

func TestRun_MarshalInput(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddBuiltin(&echoTool{name: "echo"}); err != nil {
		t.Fatal(err)
	}

	// Provide a struct — should be marshalled to JSON
	input := map[string]string{"key": "value"}
	result, err := tk.Run(context.Background(), "echo", input)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestRun_SchemaValidationError(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	st := &schemaTool{
		name: "strict",
		schema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"count": {Type: "integer"},
			},
			Required: []string{"count"},
		},
	}
	if err := tk.AddBuiltin(st); err != nil {
		t.Fatal(err)
	}

	// Missing required field — should fail validation
	_, err = tk.Run(context.Background(), "strict", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected validation error for missing required field")
	}
	t.Log("got expected error:", err)
}

///////////////////////////////////////////////////////////////////////////////
// TestFeedback

func TestFeedback_ToolFound(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddBuiltin(&stubTool{name: "my_tool"}); err != nil {
		t.Fatal(err)
	}

	fb := tk.Feedback(schema.ToolCall{Name: "my_tool"})
	expected := "my_tool: stub"
	if fb != expected {
		t.Fatalf("expected %q, got %q", expected, fb)
	}
}

func TestFeedback_ToolNotFound(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	fb := tk.Feedback(schema.ToolCall{Name: "unknown"})
	if fb != "unknown" {
		t.Fatalf("expected %q, got %q", "unknown", fb)
	}
}

///////////////////////////////////////////////////////////////////////////////
// TestString

func TestString_Empty(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	s := tk.String()
	if s == "" {
		t.Log("String() returned empty string for empty toolkit — OK")
	}
}

func TestString_WithTools(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddBuiltin(&stubTool{name: "alpha"}); err != nil {
		t.Fatal(err)
	}

	s := tk.String()
	if s == "" {
		t.Fatal("expected non-empty String() for toolkit with tools")
	}
}
