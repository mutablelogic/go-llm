package server_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	server "github.com/mutablelogic/go-llm/mcp/server"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TEST TYPES

type mockConnector struct {
	tools     []llm.Tool
	prompts   []llm.Prompt
	resources []llm.Resource
}

type mockTool struct {
	tool.DefaultTool
	name        string
	description string
	input       *jsonschema.Schema
}

type mockPrompt struct{}

type mockResource struct{}

func (m *mockConnector) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (m *mockConnector) ListTools(context.Context) ([]llm.Tool, error) {
	return m.tools, nil
}

func (m *mockConnector) ListPrompts(context.Context) ([]llm.Prompt, error) {
	return m.prompts, nil
}

func (m *mockConnector) ListResources(context.Context) ([]llm.Resource, error) {
	return m.resources, nil
}

func (m *mockTool) Name() string { return m.name }

func (m *mockTool) Description() string { return m.description }

func (m *mockTool) InputSchema() *jsonschema.Schema { return m.input }

func (*mockTool) Run(context.Context, json.RawMessage) (any, error) { return "ok", nil }

func (*mockPrompt) Name() string { return "mock_prompt" }

func (*mockPrompt) Title() string { return "Mock Prompt" }

func (*mockPrompt) Description() string { return "Prompt from connector" }

func (*mockPrompt) Prepare(_ context.Context, input json.RawMessage) (string, []opt.Opt, error) {
	if len(input) == 0 {
		return "hello from prompt", nil, nil
	}
	return string(input), nil, nil
}

func (*mockResource) URI() string { return "memory://mock-resource" }

func (*mockResource) Name() string { return "Mock Resource" }

func (*mockResource) Description() string { return "Resource from connector" }

func (*mockResource) Type() string { return "text/plain" }

func (*mockResource) Read(context.Context) ([]byte, error) { return []byte("resource text"), nil }

///////////////////////////////////////////////////////////////////////////////
// TESTS

func TestAddConnector(t *testing.T) {
	srv, err := server.New("test-server", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	tool := &mockTool{
		name:        "mock_tool",
		description: "Tool from connector",
		input: jsonschema.MustFor[struct {
			Value string `json:"value,omitempty"`
		}](),
	}
	conn := &mockConnector{
		tools:     []llm.Tool{tool},
		prompts:   []llm.Prompt{&mockPrompt{}},
		resources: []llm.Resource{&mockResource{}},
	}

	if err := srv.AddConnector(context.Background(), conn); err != nil {
		t.Fatal(err)
	}

	_, session := connect(t, srv)

	toolList, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(toolList.Tools) != 1 || toolList.Tools[0].Name != "mock_tool" {
		t.Fatalf("unexpected tools: %+v", toolList.Tools)
	}

	promptList, err := session.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(promptList.Prompts) != 1 || promptList.Prompts[0].Name != "mock_prompt" {
		t.Fatalf("unexpected prompts: %+v", promptList.Prompts)
	}

	promptResult, err := session.GetPrompt(context.Background(), &sdkmcp.GetPromptParams{Name: "mock_prompt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(promptResult.Messages) != 1 {
		t.Fatalf("expected 1 prompt message, got %d", len(promptResult.Messages))
	}

	resourceList, err := session.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resourceList.Resources) != 1 || resourceList.Resources[0].URI != "memory://mock-resource" {
		t.Fatalf("unexpected resources: %+v", resourceList.Resources)
	}
}
