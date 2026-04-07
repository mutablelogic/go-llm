package schema

import (
	"testing"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

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

func TestMessageTableCells(t *testing.T) {
	message := Message{
		Role:   RoleAssistant,
		Tokens: 42,
		Content: []ContentBlock{
			{Text: types.Ptr("First paragraph with enough words to wrap inside the table output.")},
			{Thinking: types.Ptr("Reasoning that should still be visible in summarized form.")},
			{ToolCall: &ToolCall{Name: "search_web"}},
		},
	}

	if got := message.Header(); len(got) != 4 || got[0] != "ROLE" || got[1] != "TEXT" || got[2] != "TOKENS" || got[3] != "RESULT" {
		t.Fatalf("unexpected headers: %v", got)
	}
	if got := message.Cell(0); got != "assistant" {
		t.Fatalf("unexpected role cell: %q", got)
	}
	if got := message.Cell(1); got == "" || got == message.Text() {
		t.Fatalf("expected summarized message text, got %q", got)
	}
	if got := message.Cell(2); got != "42" {
		t.Fatalf("unexpected token cell: %q", got)
	}
	if got := message.Cell(3); got != "" {
		t.Fatalf("unexpected result cell: %q", got)
	}

	message.Result = ResultToolCall
	if got := message.Cell(3); got != "tool_call" {
		t.Fatalf("unexpected result cell: %q", got)
	}
}

func TestMessageTableTextTruncates(t *testing.T) {
	text := truncateTableText("abcdefghijklmnopqrstuvwxyz", 10)
	if text != "abcdefghi..." {
		t.Fatalf("unexpected truncated text: %q", text)
	}
}

func TestMessageTableTextCompactsWhitespace(t *testing.T) {
	message := Message{
		Content: []ContentBlock{{Text: types.Ptr("Line one\n\nLine   two\twith   extra space")}},
	}

	if got := messageTableText(message); got != "Line one Line two with extra space" {
		t.Fatalf("unexpected compacted text: %q", got)
	}
}
