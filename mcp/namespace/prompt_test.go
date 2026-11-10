package namespace_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	namespace "github.com/mutablelogic/go-llm/mcp/namespace"
	test "github.com/mutablelogic/go-llm/mcp/test"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// newPromptServer returns a mock MCP server, with its tools, and prompts: one
// with arguments, one whose name is the echo tool's, and one with an
// embedded resource, and an image
func newPromptServer(t *testing.T) *test.Server {
	t.Helper()
	server := newServer(t)
	server.MCP.AddPrompt(&sdkmcp.Prompt{
		Name: "args-prompt", Title: "Arguments Prompt", Description: "The weather in a city",
		Arguments: []*sdkmcp.PromptArgument{{Name: "city", Description: "Name of the city", Required: true}, {Name: "state", Title: "State"}},
	}, func(_ context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		return &sdkmcp.GetPromptResult{Messages: []*sdkmcp.PromptMessage{
			{Role: "user", Content: &sdkmcp.TextContent{Text: "What's the weather in " + req.Params.Arguments["city"] + "?"}},
		}}, nil
	})
	server.MCP.AddPrompt(&sdkmcp.Prompt{Name: "echo", Description: "A prompt whose name is a tool's"}, func(context.Context, *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		return &sdkmcp.GetPromptResult{}, nil
	})
	server.MCP.AddPrompt(&sdkmcp.Prompt{Name: "resource-prompt", Description: "A prompt with a resource"}, func(context.Context, *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		return &sdkmcp.GetPromptResult{Messages: []*sdkmcp.PromptMessage{
			{Role: "user", Content: &sdkmcp.EmbeddedResource{Resource: &sdkmcp.ResourceContents{URI: "test://notes", MIMEType: "text/plain", Text: "The notes"}}},
			{Role: "user", Content: &sdkmcp.ImageContent{MIMEType: "image/png", Data: []byte("\x89PNG")}},
			{Role: "assistant", Content: &sdkmcp.TextContent{Text: "Noted."}},
		}}, nil
	})
	return server
}

func Test_Prompts(t *testing.T) {
	ctx := t.Context()
	server := newPromptServer(t)

	t.Run("NotAgents", func(t *testing.T) {
		// Without WithAgents, the prompts aren't tools
		ns := connected(t, connect(t, server), nil)
		for _, tool := range ns.Tools() {
			_, isAgent := tool.(toolkit.AgentTool)
			assert.False(t, isAgent, tool.Name())
		}
		_, err := ns.Prompt("args_prompt")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	// With it, they're agents, by their names as identifiers, apart from the
	// one whose name is a tool's, which run with the function, by their
	// names in the namespace
	var mu sync.Mutex
	var ran []string
	run := func(_ context.Context, name string, input json.RawMessage) (any, error) {
		mu.Lock()
		defer mu.Unlock()
		ran = append(ran, name+" "+string(input))
		return "ran", nil
	}
	ns := connected(t, connect(t, server), nil, namespace.WithAgents(run))

	t.Run("Agents", func(t *testing.T) {
		agents := map[string]llm.Tool{}
		for _, tool := range ns.Tools() {
			if agent, ok := tool.(toolkit.AgentTool); ok && agent.IsAgent() {
				agents[tool.Name()] = tool
			}
		}
		require.Len(t, agents, 2)
		args := agents["args_prompt"]
		require.NotNil(t, args)
		assert.Equal(t, "Arguments Prompt", args.Title())
		assert.Equal(t, "The weather in a city. "+"This is an agent: it asks a model the server's prompt, with the input, and returns the model's reply, rather than the prompt's text.", args.Description(), "which says it's an agent")
		input, err := json.Marshal(args.InputSchema())
		require.NoError(t, err)
		assert.JSONEq(t, `{"type":"object","properties":{"city":{"type":"string","description":"Name of the city"},"state":{"type":"string","description":"State"}},"required":["city"]}`, string(input))
		assert.NotNil(t, agents["resource_prompt"])

		result, err := args.Run(ctx, json.RawMessage(`{"city":"Berlin"}`))
		require.NoError(t, err)
		assert.Equal(t, "ran", result)
		assert.Equal(t, []string{`test:args_prompt {"city":"Berlin"}`}, ran)
	})

	t.Run("Messages", func(t *testing.T) {
		// The prompt's messages, with its arguments, of which one which
		// isn't a string is its JSON, and one which is required must be given
		messages, err := ns.PromptMessages(ctx, "args_prompt", json.RawMessage(`{"city":"Berlin"}`))
		require.NoError(t, err)
		require.Len(t, messages, 1)
		assert.Equal(t, "user", messages[0].Role())
		assert.Equal(t, "What's the weather in Berlin?", messages[0].Text())
		messages, err = ns.PromptMessages(ctx, "args_prompt", json.RawMessage(`{"city":42}`))
		require.NoError(t, err)
		assert.Equal(t, "What's the weather in 42?", messages[0].Text())
		_, err = ns.PromptMessages(ctx, "args_prompt", json.RawMessage(`{"state":"Texas"}`))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = ns.PromptMessages(ctx, "args_prompt", json.RawMessage(`[]`))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = ns.PromptMessages(ctx, "args-prompt", nil)
		assert.ErrorIs(t, err, llm.ErrNotFound, "by its name as an agent")

		// An embedded resource's text, under its URI, an image, as an
		// attachment, and the assistant's message, as its
		messages, err = ns.PromptMessages(ctx, "resource_prompt", nil)
		require.NoError(t, err)
		require.Len(t, messages, 3)
		assert.Contains(t, messages[0].Text(), "test://notes")
		assert.Contains(t, messages[0].Text(), "The notes")
		image, ok := messages[1].(*providerschema.Message)
		require.True(t, ok)
		require.Len(t, image.Content, 1)
		require.NotNil(t, image.Content[0].Resource)
		assert.Equal(t, "image/png", image.Content[0].Resource.ContentType)
		assert.Equal(t, "assistant", messages[2].Role())
	})

	t.Run("Disconnected", func(t *testing.T) {
		// Its prompts go with its tools, once it's disconnected
		c := connect(t, server)
		ns := connected(t, c, nil, namespace.WithAgents(run))
		ns.Disconnect(c)
		assert.Empty(t, ns.Tools())
		_, err := ns.PromptMessages(ctx, "args_prompt", json.RawMessage(`{"city":"Berlin"}`))
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Patterns", func(t *testing.T) {
		// The connector's patterns include, or exclude, them, by their names
		// as agents, as they do its tools
		ns := connected(t, connect(t, server), func(name string) bool { return name != "resource_prompt" }, namespace.WithAgents(run))
		_, err := ns.Prompt("resource_prompt")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = ns.Prompt("args_prompt")
		assert.NoError(t, err)
	})
}
