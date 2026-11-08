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

// connected returns a namespace, of the tools of the server the client is
// connected to, which are included by the match
func connected(t *testing.T, c *client.Client, match func(string) bool, opts ...namespace.Opt) *namespace.Namespace {
	t.Helper()
	ns, err := namespace.New("test", "https://mcp.example.com", opts...)
	require.NoError(t, err)
	require.NoError(t, ns.Connect(t.Context(), c, "Test server", match))
	return ns
}

func Test_Namespace(t *testing.T) {
	ctx := t.Context()
	server := newServer(t)
	ns := connected(t, connect(t, server), nil)

	t.Run("Tools", func(t *testing.T) {
		// The namespace has its name, and the server's title, as its
		// description, and the server's tools, in order of their names, with
		// their descriptions, or titles, and input schemas
		assert.Equal(t, "test", ns.Name())
		assert.Equal(t, "Test server", ns.Description())
		tools := ns.Tools()
		require.Len(t, tools, 3)
		names := []string{tools[0].Name(), tools[1].Name(), tools[2].Name()}
		assert.Equal(t, []string{"echo", "fail", "sum"}, names)
		assert.Equal(t, "Echo the text", tools[0].Description())
		assert.Equal(t, "Add two numbers", tools[2].Description(), "its title, as it has no description")
		assert.Equal(t, "Add two numbers", tools[2].Title())
		assert.Empty(t, tools[0].Title())
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

func Test_Connection(t *testing.T) {
	// A namespace has no tools, until it's connected, and none again, once
	// it's disconnected, and says each was created, and deleted
	ctx := t.Context()
	server := newServer(t)
	ns, err := namespace.New("test", "https://mcp.example.com")
	require.NoError(t, err)
	assert.Equal(t, "https://mcp.example.com", ns.Description())
	assert.Empty(t, ns.Tools())
	assert.Empty(t, ns.SystemPrompt())
	require.NoError(t, ns.Refresh(ctx), "which does nothing")
	var mu sync.Mutex
	var events []string
	ns.Notify(func(action eventschema.EventAction, name string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, string(action)+" "+name)
	})

	c := connect(t, server)
	require.NoError(t, ns.Connect(ctx, c, "", func(name string) bool { return name != "fail" }))
	assert.Len(t, ns.Tools(), 2)
	assert.Equal(t, "https://mcp.example.com", ns.Description(), "as the server said no title")
	ns.Disconnect()
	assert.Empty(t, ns.Tools())
	mu.Lock()
	assert.Equal(t, []string{"create echo", "create sum", "delete echo", "delete sum"}, events)
	mu.Unlock()

	// But not with a client which isn't connected, when it has none, nor
	// without a name, or a client
	unconnected, err := client.New(server.URL)
	require.NoError(t, err)
	assert.Error(t, ns.Connect(ctx, unconnected, "", nil))
	assert.Empty(t, ns.Tools())
	assert.Error(t, ns.Connect(ctx, nil, "", nil))
	_, err = namespace.New("", "")
	assert.Error(t, err)
}

func Test_Timeout(t *testing.T) {
	// A tool has the namespace's timeout, or none, for the toolkit's default
	server := newServer(t)
	assert.Zero(t, connected(t, connect(t, server), nil).Tools()[0].Timeout())
	assert.Equal(t, time.Minute, connected(t, connect(t, server), nil, namespace.WithTimeout(time.Minute)).Tools()[0].Timeout())
	_, err := namespace.New("test", "", namespace.WithTimeout(-time.Minute))
	assert.Error(t, err)
}

func Test_Refresh(t *testing.T) {
	// The tools are listed again, as the server says they changed, and each
	// which was created, updated, or deleted is said
	ctx := t.Context()
	server := newServer(t)
	changed := make(chan struct{}, 8)
	ns := connected(t, connect(t, server, client.WithToolsChanged(func() { changed <- struct{}{} })), nil)
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
