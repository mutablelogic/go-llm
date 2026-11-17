package namespace_test

import (
	"context"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	client "github.com/mutablelogic/go-llm/mcp/client"
	namespace "github.com/mutablelogic/go-llm/mcp/namespace"
	server "github.com/mutablelogic/go-llm/mcp/server"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
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
	case ch <- schema.Change{Action: eventschema.EventUpdate}:
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
func (noTools) Watch(ctx context.Context, ch chan<- eventschema.Event) error {
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
	listAgain := schema.Change{Action: eventschema.EventUpdate}
	assert.Equal(t, listAgain, <-changes, "it's watching")

	// It's subscribed to, before it's connected, and once it is, it lists
	// again, and is told of its updates
	uri := "file:///a.md"
	require.NoError(t, ns.Subscribe(ctx, uri))
	require.NoError(t, ns.Connect(ctx, connectTo(), "", nil, 1))
	assert.Equal(t, listAgain, <-changes, "as it's connected")
	updated := func() {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			// Said until it's told, as its subscription starts as it's
			// connected
			source.changes <- schema.Change{Action: eventschema.EventUpdate, URI: uri}
			select {
			case change := <-changes:
				if change.URI == "" {
					continue
				}
				assert.Equal(t, schema.Change{Action: eventschema.EventUpdate, URI: uri}, change)
				return
			case <-time.After(50 * time.Millisecond):
			case <-deadline:
				t.Fatal("not told it was updated")
			}
		}
	}
	updated()

	// Connected with another client, it lists again, and the new client is
	// subscribed to it
	require.NoError(t, ns.Connect(ctx, connectTo(), "", nil, 2))
	for len(changes) > 0 {
		<-changes
	}
	updated()

	cancel()
	require.NoError(t, <-watched)
}
