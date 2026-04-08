package tool_test

import (
	"testing"

	// Packages
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

func TestDefaultTool_OutputSchema(t *testing.T) {
	dt := tool.DefaultTool{}
	s := dt.OutputSchema()
	if s != nil {
		t.Fatal("expected nil OutputSchema from DefaultTool")
	}
}

func TestDefaultTool_Meta(t *testing.T) {
	dt := tool.DefaultTool{}
	m := dt.Meta()
	// ToolMeta zero value — just confirm it doesn't panic
	_ = m
}
