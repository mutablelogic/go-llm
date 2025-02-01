package ollama_test

import (
	"testing"

	// Packagees

	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
)

func Test_tool_001(t *testing.T) {
	tool, err := ollama.NewTool("test", "test_tool", struct{}{})
	if err != nil {
		t.FailNow()
	}
	t.Log(tool)
}

func Test_tool_002(t *testing.T) {
	tool, err := ollama.NewTool("test", "test_tool", struct {
		A string  `help:"A string"`
		B int     `help:"An integer"`
		C float64 `help:"A float" required:""`
	}{})
	if err != nil {
		t.FailNow()
	}
	t.Log(tool)
}
