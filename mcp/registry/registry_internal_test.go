package registry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	client "github.com/mutablelogic/go-llm/mcp/client"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	test "github.com/mutablelogic/go-llm/mcp/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// clientOf returns the client of the connector with the name, or nil when
// it has none
func clientOf(r *Registry, name string) *client.Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.clients[name].client
}

// setInterval sets the interval, until the test ends
func setInterval(t *testing.T, interval *time.Duration, value time.Duration) {
	t.Helper()
	before := *interval
	*interval = value
	t.Cleanup(func() { *interval = before })
}

func Test_Clients(t *testing.T) {
	setInterval(t, &ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	server := test.NewServer(t)
	logger := slog.New(slog.DiscardHandler)
	r, err := New(WithName("test", "1.0"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, r.Close()) })
	_, err = r.CreateConnector(ctx, schema.Connector{Name: "test", ConnectorMeta: schema.ConnectorMeta{URL: server.URL}})
	require.NoError(t, err)

	t.Run("Once", func(t *testing.T) {
		// A connector's client is created once, as it's reconciled, and again
		// once its definition changes, when the one it replaces is
		// disconnected
		assert.Nil(t, clientOf(r, "test"), "not until it's reconciled")
		r.reconcile(ctx, logger)
		c := clientOf(r, "test")
		require.NotNil(t, c)
		assert.True(t, c.Connected())
		assert.Equal(t, server.URL, c.Endpoint())
		r.reconcile(ctx, logger)
		assert.Same(t, c, clientOf(r, "test"))

		_, err = r.UpdateConnector(ctx, "test", schema.ConnectorMeta{URL: server.URL + "/"})
		require.NoError(t, err)
		assert.False(t, c.Connected(), "the client it replaced")
		r.reconcile(ctx, logger)
		updated := clientOf(r, "test")
		assert.NotSame(t, c, updated)
		assert.Equal(t, server.URL+"/", updated.Endpoint())
	})

	t.Run("Deleted", func(t *testing.T) {
		// A connector which is deleted is disconnected, and forgotten
		c := clientOf(r, "test")
		require.NotNil(t, c)
		require.True(t, c.Connected())
		_, err := r.DeleteConnector(ctx, "test")
		require.NoError(t, err)
		assert.False(t, c.Connected())
		assert.Nil(t, clientOf(r, "test"))
	})

	t.Run("Removed", func(t *testing.T) {
		// A connector which is removed from the store, such as by another
		// process, is disconnected, and forgotten, once it's reconciled
		created, err := r.CreateConnector(ctx, schema.Connector{Name: "removed", ConnectorMeta: schema.ConnectorMeta{URL: server.URL}})
		require.NoError(t, err)
		r.reconcile(ctx, logger)
		c := clientOf(r, "removed")
		require.NotNil(t, c)
		require.True(t, c.Connected())
		require.NoError(t, r.store.Delete(ctx, "removed", created.Serial))
		r.reconcile(ctx, logger)
		assert.False(t, c.Connected())
		assert.Nil(t, clientOf(r, "removed"))
	})

	t.Run("Close", func(t *testing.T) {
		// Closing the registry disconnects every server
		_, err := r.CreateConnector(ctx, schema.Connector{Name: "again", ConnectorMeta: schema.ConnectorMeta{URL: server.URL}})
		require.NoError(t, err)
		r.reconcile(ctx, logger)
		c := clientOf(r, "again")
		require.NoError(t, r.Close())
		assert.False(t, c.Connected())
		assert.Nil(t, clientOf(r, "again"))
	})

	t.Run("Name", func(t *testing.T) {
		// A name is needed
		_, err := New(WithName(" ", ""))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

func Test_Jitter(t *testing.T) {
	ctx := t.Context()
	server := test.NewServer(t)
	logger := slog.New(slog.DiscardHandler)

	t.Run("One", func(t *testing.T) {
		// One connector is connected at once
		setInterval(t, &ConnectJitter, time.Hour)
		r, err := New()
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, r.Close()) })
		_, err = r.CreateConnector(ctx, schema.Connector{Name: "one", ConnectorMeta: schema.ConnectorMeta{URL: server.URL}})
		require.NoError(t, err)
		start := time.Now()
		r.reconcile(ctx, logger)
		assert.Less(t, time.Since(start), 5*time.Second)
		assert.True(t, clientOf(r, "one").Connected())
	})

	t.Run("More", func(t *testing.T) {
		// More than one are each connected after a while, at random, within
		// the jitter
		setInterval(t, &ConnectJitter, 200*time.Millisecond)
		r, err := New()
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, r.Close()) })
		for i := range 5 {
			_, err = r.CreateConnector(ctx, schema.Connector{Name: fmt.Sprintf("c%d", i), ConnectorMeta: schema.ConnectorMeta{URL: server.URL}})
			require.NoError(t, err)
		}
		start := time.Now()
		r.reconcile(ctx, logger)
		assert.Less(t, time.Since(start), 5*time.Second)
		for i := range 5 {
			assert.True(t, clientOf(r, fmt.Sprintf("c%d", i)).Connected())
		}
	})

	t.Run("Cancelled", func(t *testing.T) {
		// A connector which is waiting isn't connected once the context is
		// cancelled
		setInterval(t, &ConnectJitter, time.Hour)
		r, err := New()
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, r.Close()) })
		for i := range 2 {
			_, err = r.CreateConnector(ctx, schema.Connector{Name: fmt.Sprintf("c%d", i), ConnectorMeta: schema.ConnectorMeta{URL: server.URL}})
			require.NoError(t, err)
		}
		cancelled, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		start := time.Now()
		r.reconcile(cancelled, logger)
		assert.Less(t, time.Since(start), 5*time.Second)
	})
}

func Test_UpdateKeepsSession(t *testing.T) {
	// A connector whose patterns change, at the same URL, keeps its session,
	// as some servers have one at once, but one whose URL changes doesn't
	setInterval(t, &ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	server := test.NewServer(t)
	r, err := New()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, r.Close()) })
	_, err = r.CreateConnector(ctx, schema.Connector{Name: "test", ConnectorMeta: schema.ConnectorMeta{URL: server.URL}})
	require.NoError(t, err)
	r.reconcile(ctx, slog.New(slog.DiscardHandler))
	c := clientOf(r, "test")
	require.True(t, c.Connected())

	updated, err := r.UpdateConnector(ctx, "test", schema.ConnectorMeta{URL: server.URL, Exclude: []string{"secret"}})
	require.NoError(t, err)
	assert.True(t, updated.Connected)
	assert.Equal(t, []string{"secret"}, updated.Exclude)
	assert.Same(t, c, clientOf(r, "test"))
	assert.True(t, c.Connected())

	_, err = r.UpdateConnector(ctx, "test", schema.ConnectorMeta{URL: server.URL + "/"})
	require.NoError(t, err)
	assert.NotSame(t, c, clientOf(r, "test"))
	assert.False(t, c.Connected())
}

func Test_UpdateTimeout(t *testing.T) {
	// A server which doesn't reply, as it's connected, within the connect
	// timeout, is unavailable
	ctx := t.Context()
	release := make(chan struct{})
	hung := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() {
		close(release)
		hung.Close()
	})
	r, err := New(WithConnectTimeout(100 * time.Millisecond))
	require.NoError(t, err)
	_, err = r.CreateConnector(ctx, schema.Connector{Name: "test", ConnectorMeta: schema.ConnectorMeta{URL: "https://mcp.example.com"}})
	require.NoError(t, err)
	start := time.Now()
	_, err = r.UpdateConnector(ctx, "test", schema.ConnectorMeta{URL: hung.URL})
	assert.Less(t, time.Since(start), 2*time.Second, "and is said as soon as it times out")
	assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
}

func Test_Timeouts(t *testing.T) {
	// The connect timeout is positive, and the tool timeout isn't negative,
	// and a connector's tools have it
	_, err := New(WithConnectTimeout(0))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = New(WithToolTimeout(-time.Second))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	r, err := New()
	require.NoError(t, err)
	assert.Equal(t, ConnectTimeout, r.connectTimeout)
	assert.Zero(t, r.toolTimeout)

	setInterval(t, &ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	server := test.NewServer(t)
	sdkmcp.AddTool(server.MCP, &sdkmcp.Tool{Name: "echo", Description: "Echo"}, func(context.Context, *sdkmcp.CallToolRequest, struct{}) (*sdkmcp.CallToolResult, any, error) {
		return &sdkmcp.CallToolResult{}, nil, nil
	})
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	r, err = New(WithToolkit(tk), WithToolTimeout(5*time.Minute))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, r.Close()) })
	_, err = r.CreateConnector(ctx, schema.Connector{Name: "test", ConnectorMeta: schema.ConnectorMeta{URL: server.URL}})
	require.NoError(t, err)
	r.reconcile(ctx, slog.New(slog.DiscardHandler))
	tools, err := tk.Tools(ctx, "test:echo")
	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, 5*time.Minute, tools[0].Timeout())
}

// toolCount returns how many tools the toolkit has, in the namespace
func toolCount(t *testing.T, tk *toolkit.Toolkit, namespace string) int {
	t.Helper()
	list, err := tk.ListTools(t.Context(), toolschema.ToolListRequest{Namespace: namespace})
	require.NoError(t, err)
	return len(list.Body)
}

// echoServer returns a mock server, with a tool which echoes
func echoServer(t *testing.T) *test.Server {
	t.Helper()
	server := test.NewServer(t)
	sdkmcp.AddTool(server.MCP, &sdkmcp.Tool{Name: "echo", Description: "Echo"}, func(context.Context, *sdkmcp.CallToolRequest, struct{}) (*sdkmcp.CallToolResult, any, error) {
		return &sdkmcp.CallToolResult{}, nil, nil
	})
	return server
}

func Test_Stale(t *testing.T) {
	// A definition which is older than the registry's, such as one read
	// before it was updated, or deleted, isn't connected, but one which is
	// created again, with its name, is
	setInterval(t, &ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	logger := slog.New(slog.DiscardHandler)
	server := echoServer(t)
	r, err := New()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, r.Close()) })
	first, err := r.CreateConnector(ctx, schema.Connector{Name: "test", ConnectorMeta: schema.ConnectorMeta{URL: server.URL}})
	require.NoError(t, err)
	r.reconcile(ctx, logger)
	_, err = r.UpdateConnector(ctx, "test", schema.ConnectorMeta{URL: server.URL + "/"})
	require.NoError(t, err)
	current := clientOf(r, "test")
	require.True(t, current.Connected())

	_, err = r.client(first.Definition())
	assert.ErrorIs(t, err, llm.ErrConflict, "the definition before it was updated")
	assert.Same(t, current, clientOf(r, "test"))
	assert.True(t, current.Connected())

	updated, err := r.GetConnector(ctx, "test")
	require.NoError(t, err)
	_, err = r.DeleteConnector(ctx, "test")
	require.NoError(t, err)
	_, err = r.client(updated.Definition())
	assert.ErrorIs(t, err, llm.ErrNotFound, "the definition before it was deleted")
	assert.Nil(t, clientOf(r, "test"))

	again, err := r.CreateConnector(ctx, schema.Connector{Name: "test", ConnectorMeta: schema.ConnectorMeta{URL: server.URL}})
	require.NoError(t, err)
	assert.Greater(t, again.Serial, updated.Serial)
	r.reconcile(ctx, logger)
	require.NotNil(t, clientOf(r, "test"))
	assert.True(t, clientOf(r, "test").Connected())
}

func Test_Displaced(t *testing.T) {
	// A connected client, which another process's change displaces, is said
	// to be disconnected, and its tools are cleared, as its new definition
	// can't be connected
	setInterval(t, &ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	logger := slog.New(slog.DiscardHandler)
	server := echoServer(t)
	b := broadcaster.New()
	ch := make(chan eventschema.Event, 16)
	watchCtx, cancel := context.WithCancel(ctx)
	done, err := b.Watch(watchCtx, ch)
	require.NoError(t, err)
	t.Cleanup(func() {
		cancel()
		<-done
	})
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	store := NewMemoryStore()
	r, err := New(WithStore(store), WithToolkit(tk), WithBroadcaster(b), WithConnectTimeout(time.Second))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, r.Close()) })
	created, err := r.CreateConnector(ctx, schema.Connector{Name: "test", ConnectorMeta: schema.ConnectorMeta{URL: server.URL}})
	require.NoError(t, err)
	r.reconcile(ctx, logger)
	require.Equal(t, 1, toolCount(t, tk, "test"))
	for len(ch) > 0 {
		<-ch
	}

	// Another process changes it, to a server which can't be connected
	changed := created.Definition()
	changed.URL = "http://localhost:1/mcp"
	_, err = store.Put(ctx, changed)
	require.NoError(t, err)
	r.reconcile(ctx, logger)
	assert.Zero(t, toolCount(t, tk, "test"))
	assert.True(t, tk.HasNamespace("test"))
	var deleted bool
	for len(ch) > 0 {
		if event := <-ch; event.Action == eventschema.EventDelete && event.Row == "test" {
			deleted = true
		}
	}
	assert.True(t, deleted, "its disconnection is said")
}

func Test_NamespaceRetry(t *testing.T) {
	// A connected client whose namespace hasn't its tools, such as when
	// they couldn't be listed, as it was connected, has them listed again,
	// as it's reconciled
	setInterval(t, &ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	logger := slog.New(slog.DiscardHandler)
	server := echoServer(t)
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	r, err := New(WithToolkit(tk))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, r.Close()) })
	_, err = r.CreateConnector(ctx, schema.Connector{Name: "test", ConnectorMeta: schema.ConnectorMeta{URL: server.URL}})
	require.NoError(t, err)
	r.reconcile(ctx, logger)
	require.Equal(t, 1, toolCount(t, tk, "test"))
	c := clientOf(r, "test")
	r.mu.Lock()
	ns := r.namespaces["test"]
	r.mu.Unlock()
	ns.Disconnect(c)
	require.Zero(t, toolCount(t, tk, "test"))
	require.True(t, c.Connected())
	r.reconcile(ctx, logger)
	assert.Equal(t, 1, toolCount(t, tk, "test"))
}
