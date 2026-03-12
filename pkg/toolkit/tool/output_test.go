package tool

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// OutputTool

func Test_OutputTool_001_name(t *testing.T) {
	ot := NewOutputTool(nil)
	if ot.Name() != OutputToolName {
		t.Fatalf("expected %q, got %q", OutputToolName, ot.Name())
	}
}

func Test_OutputTool_002_description(t *testing.T) {
	ot := NewOutputTool(nil)
	if ot.Description() == "" {
		t.Fatal("expected non-empty description")
	}
}

func Test_OutputTool_003_input_schema_nil(t *testing.T) {
	ot := NewOutputTool(nil)
	s, err := ot.InputSchema()
	if err != nil {
		t.Fatal(err)
	}
	if s != nil {
		t.Fatalf("expected nil schema, got %v", s)
	}
}

func Test_OutputTool_004_input_schema_set(t *testing.T) {
	schema := &jsonschema.Schema{Type: "object"}
	ot := NewOutputTool(schema)
	s, err := ot.InputSchema()
	if err != nil {
		t.Fatal(err)
	}
	if s != schema {
		t.Fatal("expected returned schema to match the one provided")
	}
}

func Test_OutputTool_005_run_returns_input(t *testing.T) {
	ot := NewOutputTool(nil)
	input := json.RawMessage(`{"answer":42}`)
	result, err := ot.Run(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := result.(json.RawMessage)
	if !ok {
		t.Fatalf("expected json.RawMessage, got %T", result)
	}
	if string(got) != string(input) {
		t.Fatalf("expected %s, got %s", input, got)
	}
}

func Test_OutputTool_006_validate_nil_schema(t *testing.T) {
	ot := NewOutputTool(nil)
	if err := ot.Validate(json.RawMessage(`{"x":1}`)); err != nil {
		t.Fatalf("expected nil error with no schema, got %v", err)
	}
}

func Test_OutputTool_007_validate_valid_data(t *testing.T) {
	schema := &jsonschema.Schema{Type: "object"}
	schema.Properties = map[string]*jsonschema.Schema{
		"name": {Type: "string"},
	}
	ot := NewOutputTool(schema)
	if err := ot.Validate(json.RawMessage(`{"name":"Alice"}`)); err != nil {
		t.Fatalf("expected valid data to pass, got %v", err)
	}
}

func Test_OutputTool_008_validate_invalid_json(t *testing.T) {
	schema := &jsonschema.Schema{Type: "object"}
	ot := NewOutputTool(schema)
	if err := ot.Validate(json.RawMessage(`not-json`)); err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func Test_OutputTool_009_implements_tool_interface(t *testing.T) {
	var _ llm.Tool = NewOutputTool(nil)
}
