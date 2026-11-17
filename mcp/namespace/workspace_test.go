package namespace_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	client "github.com/mutablelogic/go-llm/mcp/client"
	namespace "github.com/mutablelogic/go-llm/mcp/namespace"
	server "github.com/mutablelogic/go-llm/mcp/server"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// served is a workspace, of no resources, whose changes are said, which an
// MCP server serves
type served struct {
	changes chan schema.Change
}

func (served) List(context.Context, schema.ListRequest) (*schema.ListResponse, error) {
	return &schema.ListResponse{}, nil
}

func (s served) WatchResources(ctx context.Context, ch chan<- schema.Change) error {
	select {
	case ch <- schema.Change{Action: eventschema.ChangeUpdate}:
	case <-ctx.Done():
		return nil
	}
	for {
		select {
		case change := <-s.changes:
			select {
			case ch <- change:
			case <-ctx.Done():
				return nil
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// noTools is a namespace, of no tools, which the server serves
type noTools struct{}

func (noTools) Name() string         { return "files" }
func (noTools) Description() string  { return "Files" }
func (noTools) SystemPrompt() string { return "" }
func (noTools) Type() string         { return toolschema.NamespaceBuiltin }
func (noTools) Tools() []llm.Tool    { return nil }
func (noTools) Watch(ctx context.Context, ch chan<- eventschema.Change) error {
	return toolkit.WatchTools(ctx, ch, nil)
}

func Test_WatchResources(t *testing.T) {
	// A namespace's watch of its server's resources follows its client, and
	// subscribes each to the resources which are subscribed to
	ctx := t.Context()
	source := served{changes: make(chan schema.Change)}
	srv, err := server.New(noTools{}, server.WithWorkspace(source))
	require.NoError(t, err)
	runCtx, stop := context.WithCancel(ctx)
	ran := make(chan error, 1)
	go func() { ran <- srv.Run(runCtx) }()
	t.Cleanup(func() {
		stop()
		assert.NoError(t, <-ran)
	})
	connectTo := func() *client.Client {
		c, err := client.New("memory://files", client.WithTransport(srv.InMemoryTransport))
		require.NoError(t, err)
		require.NoError(t, c.Connect(ctx))
		t.Cleanup(func() { _ = c.Disconnect() })
		return c
	}

	ns, err := namespace.New("files", "")
	require.NoError(t, err)
	var _ workspace.Subscriber = ns
	changes := make(chan schema.Change, 16)
	watchCtx, cancel := context.WithCancel(ctx)
	watched := make(chan error, 1)
	go func() { watched <- ns.WatchResources(watchCtx, changes) }()
	listAgain := schema.Change{Action: eventschema.ChangeUpdate}
	assert.Equal(t, listAgain, <-changes, "it's watching")

	// It's subscribed to, before it's connected, and once it is, it lists
	// again, and is told of its updates
	uri := "file:///a.md"
	require.NoError(t, ns.Subscribe(ctx, uri))
	require.NoError(t, ns.Connect(ctx, connectTo(), "", nil, 1))
	assert.Equal(t, listAgain, <-changes, "as it's connected")
	updated := func(phase string) {
		// It's told of an update said once it lists again, as it's
		// subscribed, and watching, by then
		t.Helper()
		source.changes <- schema.Change{Action: eventschema.ChangeUpdate, URI: uri}
		select {
		case change := <-changes:
			assert.Equal(t, schema.Change{Action: eventschema.ChangeUpdate, URI: uri}, change)
		case <-time.After(5 * time.Second):
			t.Fatalf("not told it was updated, %s", phase)
		}
	}
	updated("connected")

	// Connected with another client, it lists again, and the new client is
	// subscribed to it
	require.NoError(t, ns.Connect(ctx, connectTo(), "", nil, 2))
	assert.Equal(t, listAgain, <-changes, "as it's connected again")
	updated("connected again")

	// Unsubscribed, it isn't told. It can be told to list again, such as by
	// the server's list changed, as it starts watching, which the SDK says
	// a moment later, which can be now, on a slow runner.
	require.NoError(t, ns.Unsubscribe(ctx, uri))
	source.changes <- schema.Change{Action: eventschema.ChangeUpdate, URI: uri}
	timeout := time.After(100 * time.Millisecond)
	for quiet := false; !quiet; {
		select {
		case change := <-changes:
			if change != listAgain {
				t.Fatalf("told of %v, once unsubscribed", change)
			}
		case <-timeout:
			quiet = true
		}
	}

	cancel()
	require.NoError(t, <-watched)
}

// rootedServed is a workspace whose resources are under file:///
type rootedServed struct{ served }

func (rootedServed) Root() string { return "file:///" }

func Test_Root(t *testing.T) {
	// A namespace's root is its server's, of its resource template, while
	// it's connected
	ctx := t.Context()
	srv, err := server.New(noTools{}, server.WithWorkspace(rootedServed{served{changes: make(chan schema.Change)}}))
	require.NoError(t, err)
	c, err := client.New("memory://files", client.WithTransport(srv.InMemoryTransport))
	require.NoError(t, err)
	require.NoError(t, c.Connect(ctx))
	t.Cleanup(func() { _ = c.Disconnect() })

	ns, err := namespace.New("files", "")
	require.NoError(t, err)
	var _ workspace.Rooted = ns
	assert.Empty(t, ns.Root(), "not connected")
	require.NoError(t, ns.Connect(ctx, c, "", nil, 1))
	assert.Equal(t, "file:///", ns.Root())
	ns.Disconnect(c)
	assert.Empty(t, ns.Root(), "disconnected")
}

func Test_RootRefreshFails(t *testing.T) {
	// A root which was listed is kept, when its templates can't be listed
	// again, with the same client
	ctx := t.Context()
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "files", Version: "1"}, nil)
	srv.AddResourceTemplate(&sdkmcp.ResourceTemplate{URITemplate: "file:///{+path}", Name: "files"}, func(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		return nil, sdkmcp.ResourceNotFoundError(req.Params.URI)
	})
	var fail atomic.Bool
	srv.AddReceivingMiddleware(func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			if method == "resources/templates/list" && fail.Load() {
				return nil, errors.New("templates aren't listed")
			}
			return next(ctx, method, req)
		}
	})
	c, err := client.New("memory://files", client.WithTransport(func(ctx context.Context) (sdkmcp.Transport, error) {
		clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
		if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
			return nil, err
		}
		return clientTransport, nil
	}))
	require.NoError(t, err)
	require.NoError(t, c.Connect(ctx))
	t.Cleanup(func() { _ = c.Disconnect() })

	ns, err := namespace.New("files", "")
	require.NoError(t, err)
	require.NoError(t, ns.Connect(ctx, c, "", nil, 1))
	require.Equal(t, "file:///", ns.Root())
	fail.Store(true)
	require.NoError(t, ns.Refresh(ctx))
	assert.Equal(t, "file:///", ns.Root(), "as it was")
}
