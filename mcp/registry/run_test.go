package registry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	// Packages
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	test "github.com/mutablelogic/go-llm/mcp/test"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// setInterval sets the interval, until the test ends
func setInterval(t *testing.T, interval *time.Duration, value time.Duration) {
	t.Helper()
	before := *interval
	*interval = value
	t.Cleanup(func() { *interval = before })
}

// run runs the registry, until the test ends, when it's stopped
func run(t *testing.T, r *registry.Registry) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- r.Run(ctx, nil) }()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-done)
	})
}

// status returns the connector with the name, with its status
func status(t *testing.T, r *registry.Registry, name string) *schema.Connector {
	t.Helper()
	connector, err := r.GetConnector(context.Background(), name)
	require.NoError(t, err)
	return connector
}

func Test_Run(t *testing.T) {
	setInterval(t, &registry.RetryInterval, 20*time.Millisecond)
	setInterval(t, &registry.ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	server := test.NewServer(t)
	store := registry.NewMemoryStore()
	r, err := registry.New(registry.WithStore(store))
	require.NoError(t, err)

	// A connector which is in the store, as it starts, is connected
	_, err = r.CreateConnector(ctx, connector("first", server.URL))
	require.NoError(t, err)
	run(t, r)
	require.Eventually(t, func() bool { return status(t, r, "first").Connected }, 5*time.Second, 5*time.Millisecond)
	assert.Equal(t, &schema.ConnectorServer{Name: test.Name, Version: test.Version}, status(t, r, "first").Server)

	t.Run("Created", func(t *testing.T) {
		// As is one which is created
		_, err := r.CreateConnector(ctx, connector("second", server.URL))
		require.NoError(t, err)
		require.Eventually(t, func() bool { return status(t, r, "second").Connected }, 5*time.Second, 5*time.Millisecond)
	})

	t.Run("Unavailable", func(t *testing.T) {
		// One whose server can't be connected to has why, and is tried
		// again, so it's connected once it's updated
		notFound := httptest.NewServer(http.NotFoundHandler())
		t.Cleanup(notFound.Close)
		_, err := r.CreateConnector(ctx, connector("third", notFound.URL))
		require.NoError(t, err)
		require.Eventually(t, func() bool { return status(t, r, "third").Error != "" }, 5*time.Second, 5*time.Millisecond)
		assert.False(t, status(t, r, "third").Connected)
		_, err = r.UpdateConnector(ctx, "third", schema.ConnectorMeta{URL: server.URL})
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			s := status(t, r, "third")
			return s.Connected && s.Error == ""
		}, 5*time.Second, 5*time.Millisecond)
	})

	t.Run("Deleted", func(t *testing.T) {
		// One which another process deletes from the store is disconnected,
		// and one which is created there is connected
		first := status(t, r, "first")
		require.NoError(t, store.Delete(ctx, "first", first.Serial))
		other, err := registry.New(registry.WithStore(store))
		require.NoError(t, err)
		_, err = other.CreateConnector(ctx, connector("fourth", server.URL))
		require.NoError(t, err)
		require.Eventually(t, func() bool { return status(t, r, "fourth").Connected }, 5*time.Second, 5*time.Millisecond)
	})
}

func Test_RunStops(t *testing.T) {
	// Once it's stopped, every server is disconnected
	setInterval(t, &registry.RetryInterval, 20*time.Millisecond)
	setInterval(t, &registry.ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	server := test.NewServer(t)
	r, err := registry.New()
	require.NoError(t, err)
	_, err = r.CreateConnector(ctx, connector("test", server.URL))
	require.NoError(t, err)

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error)
	go func() { done <- r.Run(runCtx, nil) }()
	require.Eventually(t, func() bool { return status(t, r, "test").Connected }, 5*time.Second, 5*time.Millisecond)
	cancel()
	require.NoError(t, <-done)
	assert.False(t, status(t, r, "test").Connected)
}

func Test_RunAtOnce(t *testing.T) {
	// A connector which is created, or updated, is connected at once, as
	// the registry's own broadcaster says it changed, rather than once it's
	// retried, whether or not it's given one for its connections
	setInterval(t, &registry.RetryInterval, time.Hour)
	setInterval(t, &registry.ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	server := test.NewServer(t)
	for name, opts := range map[string][]registry.Opt{
		"Own":         nil,
		"Connections": {registry.WithBroadcaster(broadcaster.New())},
	} {
		t.Run(name, func(t *testing.T) {
			r, err := registry.New(opts...)
			require.NoError(t, err)
			run(t, r)
			_, err = r.CreateConnector(ctx, connector("test", server.URL))
			require.NoError(t, err)
			require.Eventually(t, func() bool { return status(t, r, "test").Connected }, 5*time.Second, 5*time.Millisecond)
			_, err = r.UpdateConnector(ctx, "test", schema.ConnectorMeta{URL: server.URL + "/"})
			require.NoError(t, err)
			require.Eventually(t, func() bool { return status(t, r, "test").Connected }, 5*time.Second, 5*time.Millisecond)
		})
	}
}
