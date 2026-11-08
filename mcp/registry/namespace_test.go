package registry_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	test "github.com/mutablelogic/go-llm/mcp/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// toolServer returns a mock MCP server, with a tool which echoes text
func toolServer(t *testing.T) *test.Server {
	t.Helper()
	server := test.NewServer(t)
	sdkmcp.AddTool(server.MCP, &sdkmcp.Tool{Name: "echo", Description: "Echo the text"}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in struct {
		Text string `json:"text"`
	}) (*sdkmcp.CallToolResult, any, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: in.Text}}}, nil, nil
	})
	return server
}

// toolNames returns the canonical names of the toolkit's tools, in the
// namespace
func toolNames(t *testing.T, tk *toolkit.Toolkit, namespace string) []string {
	t.Helper()
	list, err := tk.ListTools(context.Background(), toolschema.ToolListRequest{Namespace: namespace})
	require.NoError(t, err)
	var names []string
	for _, tool := range list.Body {
		names = append(names, tool.Canonical)
	}
	return names
}

func Test_Toolkit(t *testing.T) {
	setInterval(t, &registry.RetryInterval, 20*time.Millisecond)
	setInterval(t, &registry.ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	_, err := registry.New(registry.WithToolkit(nil))
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(toolkit.NewNamespace("memory", "")))
	r, err := registry.New(registry.WithToolkit(tk))
	require.NoError(t, err)
	server := toolServer(t)
	run(t, r)

	t.Run("Added", func(t *testing.T) {
		// A connector which is connected has its server's tools in the
		// toolkit, in its namespace, which is its name, and they're called
		_, err := r.CreateConnector(ctx, connector("search", server.URL))
		require.NoError(t, err)
		assert.Equal(t, "search", status(t, r, "search").Namespace, "which it has before it's connected")
		require.Eventually(t, func() bool { return len(toolNames(t, tk, "search")) > 0 }, 5*time.Second, 5*time.Millisecond)
		assert.Equal(t, []string{"search:echo"}, toolNames(t, tk, "search"))
		list, err := tk.ListTools(ctx, toolschema.ToolListRequest{Type: toolschema.NamespaceMCP})
		require.NoError(t, err)
		require.Len(t, list.Body, 1, "its tools are an MCP server's")
		assert.Equal(t, toolschema.NamespaceMCP, list.Body[0].Type)
		result, err := tk.CallTool(ctx, "search:echo", json.RawMessage(`{"text":"hi"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `"hi"`, string(result))
	})

	t.Run("Clash", func(t *testing.T) {
		// One whose name is a namespace of the toolkit, or the agents', has
		// another
		for _, name := range []string{"memory", "agent"} {
			_, err := r.CreateConnector(ctx, connector(name, server.URL))
			require.NoError(t, err)
			namespace := status(t, r, name).Namespace
			require.Eventually(t, func() bool { return len(toolNames(t, tk, namespace)) > 0 }, 5*time.Second, 5*time.Millisecond)
			assert.Regexp(t, `^`+name+`\d{4}$`, namespace)
			assert.Equal(t, []string{namespace + ":echo"}, toolNames(t, tk, namespace))
		}
	})

	t.Run("Patterns", func(t *testing.T) {
		// Only the tools which match its patterns are added, and they're
		// added again, as its patterns are updated
		created := connector("patterns", server.URL)
		created.Exclude = []string{"^echo$"}
		_, err := r.CreateConnector(ctx, created)
		require.NoError(t, err)
		require.Eventually(t, func() bool { return status(t, r, "patterns").Connected }, 5*time.Second, 5*time.Millisecond)
		assert.Empty(t, toolNames(t, tk, "patterns"))
		_, err = r.UpdateConnector(ctx, "patterns", schema.ConnectorMeta{URL: server.URL, Include: []string{"^ec"}})
		require.NoError(t, err)
		assert.Equal(t, []string{"patterns:echo"}, toolNames(t, tk, "patterns"))

		// But not with a pattern which isn't one
		_, err = r.UpdateConnector(ctx, "patterns", schema.ConnectorMeta{URL: server.URL, Include: []string{"("}})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		bad := connector("bad", server.URL)
		bad.Include = []string{"("}
		_, err = r.CreateConnector(ctx, bad)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Unreachable", func(t *testing.T) {
		// One whose server can't be connected has its namespace, with no
		// tools, so it can be seen, and edited, and deleted
		notFound := httptest.NewServer(http.NotFoundHandler())
		t.Cleanup(notFound.Close)
		_, err := r.CreateConnector(ctx, connector("down", notFound.URL))
		require.NoError(t, err)
		assert.Equal(t, "down", status(t, r, "down").Namespace)
		assert.True(t, tk.HasNamespace("down"))
		require.Eventually(t, func() bool { return status(t, r, "down").Error != "" }, 5*time.Second, 5*time.Millisecond)
		assert.Empty(t, toolNames(t, tk, "down"))
		assert.True(t, tk.HasNamespace("down"))
		_, err = r.DeleteConnector(ctx, "down")
		require.NoError(t, err)
		assert.False(t, tk.HasNamespace("down"))
	})

	t.Run("Disconnected", func(t *testing.T) {
		// One whose server closes its session keeps its namespace, without
		// its tools, until it's connected again
		closing := test.NewServer(t, test.WithSSE())
		sdkmcp.AddTool(closing.MCP, &sdkmcp.Tool{Name: "echo", Description: "Echo"}, func(context.Context, *sdkmcp.CallToolRequest, struct{}) (*sdkmcp.CallToolResult, any, error) {
			return &sdkmcp.CallToolResult{}, nil, nil
		})
		_, err := r.CreateConnector(ctx, connector("closing", closing.URL))
		require.NoError(t, err)
		require.Eventually(t, func() bool { return len(toolNames(t, tk, "closing")) == 1 }, 5*time.Second, 5*time.Millisecond)
		closing.CloseClientConnections()
		closing.Close()
		require.Eventually(t, func() bool { return len(toolNames(t, tk, "closing")) == 0 }, 5*time.Second, 5*time.Millisecond)
		assert.True(t, tk.HasNamespace("closing"))
		assert.Equal(t, "closing", status(t, r, "closing").Namespace)
		_, err = r.DeleteConnector(ctx, "closing")
		require.NoError(t, err)
	})

	t.Run("Changed", func(t *testing.T) {
		// A tool which the server adds is added, as it says its tools
		// changed
		server.MCP.AddTool(&sdkmcp.Tool{Name: "ping", Description: "Ping", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return &sdkmcp.CallToolResult{}, nil
		})
		require.Eventually(t, func() bool { return len(toolNames(t, tk, "search")) == 2 }, 5*time.Second, 5*time.Millisecond)
		assert.Equal(t, []string{"search:echo", "search:ping"}, toolNames(t, tk, "search"))
	})

	t.Run("Removed", func(t *testing.T) {
		// One which is updated has its namespace again, and one which is
		// deleted has none
		updated, err := r.UpdateConnector(ctx, "search", schema.ConnectorMeta{URL: server.URL + "/"})
		require.NoError(t, err)
		assert.Equal(t, "search", updated.Namespace)
		assert.Len(t, toolNames(t, tk, "search"), 2)
		_, err = r.DeleteConnector(ctx, "search")
		require.NoError(t, err)
		assert.False(t, tk.HasNamespace("search"))
	})
}

func Test_ToolEvents(t *testing.T) {
	// The toolkit's broadcaster is told of a connector's tools, as they're
	// added, as its server is connected, created, updated, or deleted, as
	// its server says they changed, and deleted, as it's disconnected
	setInterval(t, &registry.ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	b := broadcaster.New()
	ch := watch(t, b)
	tk, err := toolkit.New(ctx, toolkit.WithBroadcaster(b))
	require.NoError(t, err)
	r, err := registry.New(registry.WithToolkit(tk))
	require.NoError(t, err)
	server := toolServer(t)
	run(t, r)

	// tools returns the toolkit's events, as action and tool
	tools := func() []string {
		var result []string
		for _, event := range published(ch) {
			if event.Schema == toolkit.EventSchema && event.Table == toolkit.ToolTable {
				result = append(result, string(event.Action)+" "+event.Row)
			}
		}
		return result
	}
	_, err = r.CreateConnector(ctx, connector("search", server.URL))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return len(toolNames(t, tk, "search")) > 0 }, 5*time.Second, 5*time.Millisecond)
	assert.Equal(t, []string{"create search:echo"}, tools())

	// The server adds one tool, changes another, and removes it
	server.MCP.AddTool(&sdkmcp.Tool{Name: "ping", Description: "Ping", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{}, nil
	})
	assert.Equal(t, []string{"create search:ping"}, tools())
	server.MCP.AddTool(&sdkmcp.Tool{Name: "ping", Description: "Ping it", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{}, nil
	})
	assert.Equal(t, []string{"update search:ping"}, tools())
	server.MCP.RemoveTools("ping")
	assert.Equal(t, []string{"delete search:ping"}, tools())

	// And it's deleted, so its tools are
	_, err = r.DeleteConnector(ctx, "search")
	require.NoError(t, err)
	assert.Equal(t, []string{"delete search:echo"}, tools())
}

func Test_ToolkitRestart(t *testing.T) {
	// A connector which is stored, as the registry runs, has its namespace,
	// whether it's connected, or not
	setInterval(t, &registry.ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	store := registry.NewMemoryStore()
	_, err := store.Put(ctx, connector("stored", "http://localhost:1/mcp"))
	require.NoError(t, err)
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	r, err := registry.New(registry.WithStore(store), registry.WithToolkit(tk))
	require.NoError(t, err)
	run(t, r)
	require.Eventually(t, func() bool { return tk.HasNamespace("stored") }, 5*time.Second, 5*time.Millisecond)
	assert.Equal(t, "stored", status(t, r, "stored").Namespace)
}
