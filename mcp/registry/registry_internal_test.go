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
	client "github.com/mutablelogic/go-llm/mcp/client"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	test "github.com/mutablelogic/go-llm/mcp/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
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
