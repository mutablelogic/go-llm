package schema

import "testing"

func TestAgentMetaTableCells(t *testing.T) {
	provider, model := "ollama", "llama3"
	agent := AgentMeta{
		Name:        "builtin.alpha",
		Title:       "Alpha Agent",
		Description: "First line\nsecond line",
		GeneratorMeta: GeneratorMeta{
			Provider: &provider,
			Model:    &model,
		},
		Tools: []string{"builtin.search", "builtin.fetch"},
	}

	if got := agent.Header(); len(got) != 4 || got[1] != "DESCRIPTION" || got[2] != "MODEL" || got[3] != "TOOLS" {
		t.Fatalf("unexpected headers: %v", got)
	}
	if got := agent.Cell(1); got != "Alpha Agent - First line second line" {
		t.Fatalf("unexpected description cell: %q", got)
	}
	if got := agent.Cell(2); got != "ollama/llama3" {
		t.Fatalf("unexpected model cell: %q", got)
	}
	if got := agent.Cell(3); got != "builtin.search, builtin.fetch" {
		t.Fatalf("unexpected tools cell: %q", got)
	}
}

func TestToolMetaTableCells(t *testing.T) {
	tool := ToolMeta{
		Name:        "builtin.alpha",
		Title:       "Alpha Tool",
		Description: "First line\nsecond line",
		Input:       JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`),
		Output:      JSONSchema(`{"type":"string"}`),
		Hints:       []string{"readonly", "idempotent"},
	}

	if got := tool.Header(); len(got) != 5 || got[1] != "DESCRIPTION" || got[2] != "INPUT" || got[3] != "OUTPUT" {
		t.Fatalf("unexpected headers: %v", got)
	}
	if got := tool.Cell(1); got != "Alpha Tool - First line second line" {
		t.Fatalf("unexpected description cell: %q", got)
	}
	if got := tool.Cell(2); got != `{"type":"object","properties":{"name":{"type":"string"}}}` {
		t.Fatalf("unexpected input schema cell: %q", got)
	}
	if got := tool.Cell(3); got != `{"type":"string"}` {
		t.Fatalf("unexpected output schema cell: %q", got)
	}
	if got := tool.Cell(4); got != "readonly, idempotent" {
		t.Fatalf("unexpected hints cell: %q", got)
	}
}
