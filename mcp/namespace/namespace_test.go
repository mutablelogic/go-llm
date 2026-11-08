package namespace_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	client "github.com/mutablelogic/go-llm/mcp/client"
	namespace "github.com/mutablelogic/go-llm/mcp/namespace"
	test "github.com/mutablelogic/go-llm/mcp/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

type echoIn struct {
	Text string `json:"text" jsonschema:"the text to echo"`
}

type sumOut struct {
	Sum int `json:"sum"`
}

// newServer returns a mock MCP server, with tools which echo text, add two
// numbers, with structured output, and fail
func newServer(t *testing.T) *test.Server {
	t.Helper()
	server := test.NewServer(t)
	sdkmcp.AddTool(server.MCP, &sdkmcp.Tool{Name: "echo", Description: "Echo the text"}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in echoIn) (*sdkmcp.CallToolResult, any, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: in.Text}}}, nil, nil
	})
	sdkmcp.AddTool(server.MCP, &sdkmcp.Tool{Name: "sum", Title: "Add two numbers"}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in struct{ A, B int }) (*sdkmcp.CallToolResult, sumOut, error) {
		return nil, sumOut{Sum: in.A + in.B}, nil
	})
	sdkmcp.AddTool(server.MCP, &sdkmcp.Tool{Name: "fail", Description: "Fail"}, func(_ context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, any, error) {
		return &sdkmcp.CallToolResult{IsError: true, Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "it failed"}}}, nil, nil
	})
	return server
}

// connect returns a client of the server, which is connected
func connect(t *testing.T, server *test.Server, opts ...client.Opt) *client.Client {
	t.Helper()
	c, err := client.New(server.URL, opts...)
	require.NoError(t, err)
	require.NoError(t, c.Connect(t.Context()))
	t.Cleanup(func() { assert.NoError(t, c.Disconnect()) })
	return c
}

func Test_Namespace(t *testing.T) {
	ctx := t.Context()
	server := newServer(t)
	ns, err := namespace.New(ctx, "test", "Test server", connect(t, server))
	require.NoError(t, err)

	t.Run("Tools", func(t *testing.T) {
		// The namespace has its name, and description, and the server's
		// tools, in order of their names, with their descriptions, or
		// titles, and input schemas
		assert.Equal(t, "test", ns.Name())
		assert.Equal(t, "Test server", ns.Description())
		tools := ns.Tools()
		require.Len(t, tools, 3)
		names := []string{tools[0].Name(), tools[1].Name(), tools[2].Name()}
		assert.Equal(t, []string{"echo", "fail", "sum"}, names)
		assert.Equal(t, "Echo the text", tools[0].Description())
		assert.Equal(t, "Add two numbers", tools[2].Description())
		require.NotNil(t, tools[0].InputSchema())
		data, err := json.Marshal(tools[0].InputSchema())
		require.NoError(t, err)
		assert.Contains(t, string(data), `"text"`)
		assert.NotNil(t, tools[2].OutputSchema())
	})

	t.Run("Run", func(t *testing.T) {
		// A tool returns its text, or its structured output, or its error
		tools := ns.Tools()
		result, err := tools[0].Run(ctx, json.RawMessage(`{"text":"hello"}`))
		require.NoError(t, err)
		assert.Equal(t, "hello", result)
		result, err = tools[2].Run(ctx, json.RawMessage(`{"A":2,"B":3}`))
		require.NoError(t, err)
		data, err := json.Marshal(result)
		require.NoError(t, err)
		assert.JSONEq(t, `{"sum":5}`, string(data))
		_, err = tools[1].Run(ctx, nil)
		assert.EqualError(t, err, "it failed")
	})

	t.Run("Toolkit", func(t *testing.T) {
		// It's a toolkit's namespace, whose tools are called by their
		// canonical names
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		require.NoError(t, tk.Add(ns))
		result, err := tk.CallTool(ctx, "test:echo", json.RawMessage(`{"text":"hi"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `"hi"`, string(result))
	})
}

func Test_Match(t *testing.T) {
	// Only the tools which are included are the namespace's
	ctx := t.Context()
	server := newServer(t)
	ns, err := namespace.New(ctx, "test", "", connect(t, server), namespace.WithMatch(func(name string) bool { return name != "fail" }))
	require.NoError(t, err)
	tools := ns.Tools()
	require.Len(t, tools, 2)
	assert.Equal(t, "echo", tools[0].Name())
	assert.Equal(t, "sum", tools[1].Name())
	_, err = namespace.New(ctx, "test", "", connect(t, server), namespace.WithMatch(nil))
	assert.Error(t, err)
}

func Test_Timeout(t *testing.T) {
	// A tool has the namespace's timeout, or none, for the toolkit's default
	ctx := t.Context()
	server := newServer(t)
	ns, err := namespace.New(ctx, "test", "", connect(t, server))
	require.NoError(t, err)
	assert.Zero(t, ns.Tools()[0].Timeout())
	ns, err = namespace.New(ctx, "test", "", connect(t, server), namespace.WithTimeout(time.Minute))
	require.NoError(t, err)
	assert.Equal(t, time.Minute, ns.Tools()[0].Timeout())
	_, err = namespace.New(ctx, "test", "", connect(t, server), namespace.WithTimeout(-time.Minute))
	assert.Error(t, err)
}

func Test_Refresh(t *testing.T) {
	// The tools are listed again, as the server says they changed, and each
	// which was created, updated, or deleted is said
	ctx := t.Context()
	server := newServer(t)
	changed := make(chan struct{}, 8)
	c := connect(t, server, client.WithToolsChanged(func() { changed <- struct{}{} }))
	ns, err := namespace.New(ctx, "test", "", c)
	require.NoError(t, err)
	var mu sync.Mutex
	var events []string
	ns.Notify(func(action eventschema.EventAction, name string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, string(action)+" "+name)
	})

	// A tool is added, one changed, and one removed, on the server
	server.MCP.AddTool(&sdkmcp.Tool{Name: "new", Description: "New", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{}, nil
	})
	server.MCP.AddTool(&sdkmcp.Tool{Name: "echo", Description: "Echo it", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{}, nil
	})
	server.MCP.RemoveTools("fail")
	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("the server didn't say its tools changed")
	}
	require.NoError(t, ns.Refresh(ctx))
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"update echo", "create new", "delete fail"}, events)
	assert.Len(t, ns.Tools(), 3)
}

func Test_NotConnected(t *testing.T) {
	// A client which isn't connected has no tools to list, nor is a
	// namespace without a name, or a client
	ctx := t.Context()
	c, err := client.New(newServer(t).URL)
	require.NoError(t, err)
	_, err = namespace.New(ctx, "test", "", c)
	assert.Error(t, err)
	_, err = namespace.New(ctx, "", "", c)
	assert.Error(t, err)
	_, err = namespace.New(ctx, "test", "", nil)
	assert.Error(t, err)
}
