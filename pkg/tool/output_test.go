package tool_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

func TestOutputTool_Name(t *testing.T) {
	ot := tool.NewOutputTool(nil)
	if ot.Name() != tool.OutputToolName {
		t.Fatalf("expected %q, got %q", tool.OutputToolName, ot.Name())
	}
}

func TestOutputTool_Description(t *testing.T) {
	ot := tool.NewOutputTool(nil)
	desc := ot.Description()
	if desc == "" {
		t.Fatal("expected non-empty description")
	}
}

func TestOutputTool_InputSchema_Nil(t *testing.T) {
	ot := tool.NewOutputTool(nil)
	s := ot.InputSchema()
	if s != nil {
		t.Fatal("expected nil schema when constructed with nil")
	}
}

func TestOutputTool_Run(t *testing.T) {
	ot := tool.NewOutputTool(nil)
	input := json.RawMessage(`{"answer":42}`)
	result, err := ot.Run(context.Background(), input)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	raw, ok := result.(json.RawMessage)
	if !ok {
		t.Fatalf("expected json.RawMessage result, got %T", result)
	}
	if string(raw) != string(input) {
		t.Fatalf("expected %q, got %q", string(input), string(raw))
	}
}
