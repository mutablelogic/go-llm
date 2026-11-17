package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	client "github.com/mutablelogic/go-llm/mcp/client"
	namespace "github.com/mutablelogic/go-llm/mcp/namespace"
	server "github.com/mutablelogic/go-llm/mcp/server"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// TEST NAMESPACE

// tools is a namespace whose tools change, as they're set
type tools struct {
	mu       sync.Mutex
	tools    []llm.Tool
	watchers toolkit.Watchers
}

func (n *tools) Name() string         { return "test" }
func (n *tools) Description() string  { return "Test tools" }
func (n *tools) SystemPrompt() string { return "Use the test tools" }
func (n *tools) Type() string         { return toolschema.NamespaceBuiltin }

func (n *tools) Tools() []llm.Tool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]llm.Tool(nil), n.tools...)
}

func (n *tools) Watch(ctx context.Context, ch chan<- eventschema.Change) error {
	return n.watchers.Watch(ctx, ch, n.Tools)
}

// add adds the tool, and says it was created
func (n *tools) add(tool llm.Tool) {
	n.mu.Lock()
	n.tools = append(n.tools, tool)
	n.mu.Unlock()
	n.watchers.Changed(eventschema.ChangeCreate, tool.Name())
}

// echo returns its text, or fails, when it's "fail"
type echo struct {
	toolkit.Base
	name string
}

func (t *echo) Name() string        { return t.name }
func (t *echo) Description() string { return "Echo the text" }
func (t *echo) InputSchema() *jsonschema.Schema {
	schema, _ := jsonschema.FromJSON(json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`))
	return schema
}
func (t *echo) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var in struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, err
	} else if in.Text == "fail" {
		return nil, errors.New("it failed")
	}
	toolkit.SessionFromContext(ctx).Progress(1, 1, "echoed")
	return map[string]string{"text": in.Text}, nil
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_InMemory(t *testing.T) {
	// A namespace's tools are served, in memory, and called with go-llm's
	// client, as another toolkit's namespace
	ctx := t.Context()
	source := &tools{tools: []llm.Tool{&echo{name: "echo"}}}
	srv, err := server.New(source)
	require.NoError(t, err)
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- srv.Run(runCtx) }()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-done)
	})

	// Connect to it, with the transport, rather than a URL
	changed := make(chan struct{}, 8)
	c, err := client.New("memory://test", client.WithTransport(srv.InMemoryTransport), client.WithToolsChanged(func() { changed <- struct{}{} }))
	require.NoError(t, err)
	require.NoError(t, c.Connect(ctx))
	t.Cleanup(func() { assert.NoError(t, c.Disconnect()) })
	server, _ := c.Status()
	require.NotNil(t, server)
	assert.Equal(t, "test", server.Name)
	assert.Equal(t, "Test tools", server.Title)

	// Its tools are another toolkit's
	ns, err := namespace.New("remote", "memory://test")
	require.NoError(t, err)
	require.NoError(t, ns.Connect(ctx, c, "", nil, 1))
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(ns))
	result, err := tk.CallTool(ctx, "remote:echo", json.RawMessage(`{"text":"hello"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"text":"hello"}`, string(result))

	t.Run("Errors", func(t *testing.T) {
		// A tool's error, and input which isn't valid, are errors
		_, err := tk.CallTool(ctx, "remote:echo", json.RawMessage(`{"text":"fail"}`))
		assert.ErrorContains(t, err, "it failed")
		result, err := c.CallTool(ctx, "echo", json.RawMessage(`{}`))
		require.NoError(t, err, "it's the result's error")
		assert.True(t, result.IsError)
	})

	t.Run("Changes", func(t *testing.T) {
		// A tool which is added is served, and the client is told
		source.add(&echo{name: "shout"})
		select {
		case <-changed:
		case <-time.After(5 * time.Second):
			t.Fatal("the client wasn't told the tools changed")
		}
		require.NoError(t, ns.Refresh(ctx))
		result, err := tk.CallTool(ctx, "remote:shout", json.RawMessage(`{"text":"hi"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"text":"hi"}`, string(result))
	})
}

func Test_New(t *testing.T) {
	_, err := server.New(nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = client.New("memory://test", client.WithTransport(nil))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = client.New("no-scheme", client.WithTransport(func(context.Context) (sdkmcp.Transport, error) { return nil, nil }))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

// image returns a PNG, with a value
type image struct{ toolkit.Base }

func (*image) Name() string                    { return "image" }
func (*image) Description() string             { return "An image" }
func (*image) InputSchema() *jsonschema.Schema { return nil }
func (*image) Run(context.Context, json.RawMessage) (any, error) {
	png := &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "image/png", Data: []byte("png")}}
	return toolkit.Result{Value: map[string]int{"width": 1}, Resources: []llm.Resource{png}}, nil
}

func Test_Resources(t *testing.T) {
	// A tool's value is JSON, and its resources are their content, such as
	// an image
	ctx := t.Context()
	srv, err := server.New(&tools{tools: []llm.Tool{&image{}}})
	require.NoError(t, err)
	c, err := client.New("memory://test", client.WithTransport(srv.InMemoryTransport))
	require.NoError(t, err)
	require.NoError(t, c.Connect(ctx))
	t.Cleanup(func() { assert.NoError(t, c.Disconnect()) })
	result, err := c.CallTool(ctx, "image", nil)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Len(t, result.Content, 2)
	text, ok := result.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	assert.JSONEq(t, `{"width":1}`, text.Text)
	img, ok := result.Content[1].(*sdkmcp.ImageContent)
	require.True(t, ok)
	assert.Equal(t, "image/png", img.MIMEType)
	assert.Equal(t, []byte("png"), img.Data)
}

// slow is a tool which ignores its context, and takes a second
type slow struct{ toolkit.Base }

func (*slow) Name() string                    { return "slow" }
func (*slow) Description() string             { return "Slow" }
func (*slow) InputSchema() *jsonschema.Schema { return nil }
func (*slow) Timeout() time.Duration          { return 50 * time.Millisecond }
func (*slow) Run(context.Context, json.RawMessage) (any, error) {
	time.Sleep(time.Second)
	return "late", nil
}

// none returns no value, as empty JSON, with an inline text resource
type none struct{ toolkit.Base }

func (*none) Name() string                    { return "none" }
func (*none) Description() string             { return "None" }
func (*none) InputSchema() *jsonschema.Schema { return nil }
func (*none) Run(context.Context, json.RawMessage) (any, error) {
	text := &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "text/plain", Data: []byte("hi")}}
	return toolkit.Result{Value: json.RawMessage(nil), Resources: []llm.Resource{text}}, nil
}

// connectTo returns a client connected to the server, in memory
func connectTo(t *testing.T, srv *server.Server) *client.Client {
	t.Helper()
	c, err := client.New("memory://test", client.WithTransport(srv.InMemoryTransport))
	require.NoError(t, err)
	require.NoError(t, c.Connect(t.Context()))
	t.Cleanup(func() { assert.NoError(t, c.Disconnect()) })
	return c
}

func Test_Review(t *testing.T) {
	ctx := t.Context()

	t.Run("Empty", func(t *testing.T) {
		// A client which connects while there are no tools lists them once
		// there are
		source := &tools{}
		srv, err := server.New(source)
		require.NoError(t, err)
		c := connectTo(t, srv)
		list, err := c.ListTools(ctx)
		require.NoError(t, err)
		assert.Empty(t, list)
		source.add(&echo{name: "echo"})
		list, err = c.ListTools(ctx)
		require.NoError(t, err)
		assert.Len(t, list, 1)
	})

	t.Run("Removed", func(t *testing.T) {
		// A tool which is removed, while it isn't watched, isn't served
		source := &tools{tools: []llm.Tool{&echo{name: "echo"}, &echo{name: "shout"}}}
		srv, err := server.New(source)
		require.NoError(t, err)
		source.mu.Lock()
		source.tools = source.tools[:1]
		source.mu.Unlock()
		c := connectTo(t, srv)
		list, err := c.ListTools(ctx)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, "echo", list[0].Name)
		result, err := c.CallTool(ctx, "shout", json.RawMessage(`{"text":"hi"}`))
		if err == nil {
			assert.True(t, result.IsError)
		}
	})

	t.Run("Timeout", func(t *testing.T) {
		// A tool which doesn't stop, once its time is up, is an error, then
		srv, err := server.New(&tools{tools: []llm.Tool{&slow{}}})
		require.NoError(t, err)
		c := connectTo(t, srv)
		start := time.Now()
		result, err := c.CallTool(ctx, "slow", nil)
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Less(t, time.Since(start), 500*time.Millisecond)
	})

	t.Run("Inline", func(t *testing.T) {
		// Empty JSON is no value, and an inline resource has a URI
		srv, err := server.New(&tools{tools: []llm.Tool{&none{}}})
		require.NoError(t, err)
		c := connectTo(t, srv)
		result, err := c.CallTool(ctx, "none", nil)
		require.NoError(t, err)
		require.False(t, result.IsError)
		require.Len(t, result.Content, 1)
		embedded, ok := result.Content[0].(*sdkmcp.EmbeddedResource)
		require.True(t, ok)
		assert.Regexp(t, `^resource:///[0-9a-f]{64}$`, embedded.Resource.URI)
		assert.Equal(t, "hi", embedded.Resource.Text)
	})
}
