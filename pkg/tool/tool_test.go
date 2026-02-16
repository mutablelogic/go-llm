package tool_test

import (
	"context"
	"encoding/json"
	"testing"

	jsonschema "github.com/google/jsonschema-go/jsonschema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

type stubTool struct {
	name string
}

func (s *stubTool) Name() string                                          { return s.name }
func (s *stubTool) Description() string                                   { return "stub" }
func (s *stubTool) Schema() (*jsonschema.Schema, error)                   { return nil, nil }
func (s *stubTool) Run(_ context.Context, _ json.RawMessage) (any, error) { return nil, nil }

func TestRegister_ReservedName(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	err = tk.Register(&stubTool{name: tool.OutputToolName})
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
	outputTool := tool.NewOutputTool(&jsonschema.Schema{})
	if err := tk.Register(outputTool); err != nil {
		t.Fatal("OutputTool should be allowed:", err)
	}
}

func TestRegister_NormalToolOK(t *testing.T) {
	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	if err := tk.Register(&stubTool{name: "my_tool"}); err != nil {
		t.Fatal("normal tool should register:", err)
	}
}

func TestOutputTool_ValidateValid(t *testing.T) {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"summary":   {Type: "string"},
			"sentiment": {Type: "string"},
		},
		Required: []string{"summary", "sentiment"},
	}
	ot := tool.NewOutputTool(schema)
	valid := json.RawMessage(`{"summary":"hello","sentiment":"positive"}`)
	if err := ot.Validate(valid); err != nil {
		t.Fatal("expected valid data to pass:", err)
	}
}

func TestOutputTool_ValidateMissingRequired(t *testing.T) {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"summary":   {Type: "string"},
			"sentiment": {Type: "string"},
		},
		Required: []string{"summary", "sentiment"},
	}
	ot := tool.NewOutputTool(schema)
	invalid := json.RawMessage(`{"summary":"hello"}`)
	if err := ot.Validate(invalid); err == nil {
		t.Fatal("expected error for missing required field")
	} else {
		t.Log("got expected error:", err)
	}
}

func TestOutputTool_ValidateWrongType(t *testing.T) {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"count": {Type: "integer"},
		},
	}
	ot := tool.NewOutputTool(schema)
	invalid := json.RawMessage(`{"count":"not a number"}`)
	if err := ot.Validate(invalid); err == nil {
		t.Fatal("expected error for wrong type")
	} else {
		t.Log("got expected error:", err)
	}
}

func TestOutputTool_ValidateInvalidJSON(t *testing.T) {
	schema := &jsonschema.Schema{Type: "object"}
	ot := tool.NewOutputTool(schema)
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
