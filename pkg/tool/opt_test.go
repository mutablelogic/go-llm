package tool_test

import (
	"log/slog"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

func TestWithLogHandler(t *testing.T) {
	called := false
	tk, err := tool.NewToolkit(
		tool.WithLogHandler(func(url string, level slog.Level, msg string, args ...any) {
			called = true
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()
	_ = called
}

func TestWithStateHandler(t *testing.T) {
	called := false
	tk, err := tool.NewToolkit(
		tool.WithStateHandler(func(url string, state schema.ConnectorState) {
			called = true
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()
	_ = called
}

func TestWithToolsHandler(t *testing.T) {
	tk, err := tool.NewToolkit(
		tool.WithToolsHandler(func(url string, tools []llm.Tool) {
			_ = tools
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()
}

func TestWithBuiltin(t *testing.T) {
	tk, err := tool.NewToolkit(
		tool.WithBuiltin(&stubTool{name: "via_opt"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	found := tk.Lookup("via_opt")
	if found == nil {
		t.Fatal("expected 'via_opt' to be registered via WithBuiltin")
	}
}

func TestWithBuiltin_InvalidName(t *testing.T) {
	_, err := tool.NewToolkit(
		tool.WithBuiltin(&stubTool{name: "bad name!"}),
	)
	if err == nil {
		t.Fatal("expected error for invalid tool name via WithBuiltin")
	}
}
