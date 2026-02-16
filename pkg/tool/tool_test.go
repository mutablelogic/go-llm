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
