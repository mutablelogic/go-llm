package registry

import (
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	client "github.com/mutablelogic/go-llm/mcp/client"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	test "github.com/mutablelogic/go-llm/mcp/test"
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

func Test_Clients(t *testing.T) {
	ctx := t.Context()
	server := test.NewServer(t)
	r, err := New(WithName("test", "1.0"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, r.Close()) })
	_, err = r.CreateConnector(ctx, schema.Connector{Name: "test", ConnectorMeta: schema.ConnectorMeta{URL: server.URL}})
	require.NoError(t, err)

	t.Run("Once", func(t *testing.T) {
		// A connector's client is created once, as it's connected, and again
		// once its definition changes, when the one it replaces is
		// disconnected
		assert.Nil(t, clientOf(r, "test"), "not until it's connected")
		_, err := r.Connect(ctx, "test")
		require.NoError(t, err)
		c := clientOf(r, "test")
		require.NotNil(t, c)
		assert.True(t, c.Connected())
		assert.Equal(t, server.URL, c.Endpoint())
		_, err = r.Connect(ctx, "test")
		require.NoError(t, err)
		assert.Same(t, c, clientOf(r, "test"))

		_, err = r.UpdateConnector(ctx, "test", schema.ConnectorMeta{URL: server.URL + "/"})
		require.NoError(t, err)
		assert.False(t, c.Connected(), "the client it replaced")
		_, err = r.Connect(ctx, "test")
		require.NoError(t, err)
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

	t.Run("Close", func(t *testing.T) {
		// Closing the registry disconnects every server
		_, err := r.CreateConnector(ctx, schema.Connector{Name: "again", ConnectorMeta: schema.ConnectorMeta{URL: server.URL}})
		require.NoError(t, err)
		_, err = r.Connect(ctx, "again")
		require.NoError(t, err)
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
