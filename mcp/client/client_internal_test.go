package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	goclient "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_New(t *testing.T) {
	t.Run("URL", func(t *testing.T) {
		// A server is at an http, or https, URL, with a host
		c, err := New(" https://mcp.example.com/mcp ", WithName("test", ""))
		require.NoError(t, err)
		assert.Equal(t, "https://mcp.example.com/mcp", c.Endpoint())
		assert.NotNil(t, c.Client)

		// And not a command, or a path
		for _, endpoint := range []string{"", "mcp.example.com", "/mcp", "stdio://server", "file:///usr/bin/server", "https:///mcp", "http://%zz"} {
			_, err := New(endpoint, WithName("test", ""))
			assert.ErrorIs(t, err, llm.ErrBadParameter, endpoint)
		}
	})

	t.Run("Name", func(t *testing.T) {
		// The server is told the client's name, and version, which it needs
		c, err := New("https://mcp.example.com/mcp", WithName(" assistant ", " 1.2.3 "))
		require.NoError(t, err)
		assert.Equal(t, sdkmcp.Implementation{Name: "assistant", Version: "1.2.3"}, c.implementation)
		_, err = New("https://mcp.example.com/mcp")
		assert.ErrorIs(t, err, llm.ErrBadParameter, "without a name")
		_, err = New("https://mcp.example.com/mcp", WithName(" ", "1.2.3"))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

func Test_Transports(t *testing.T) {
	// The MCP SDK's requests are made with go-client's client, with its
	// options, but without its timeout
	var got http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
	}))
	t.Cleanup(server.Close)
	c, err := New(server.URL+"/mcp", WithName("test", "1.0"), WithClientOpts(
		goclient.OptHeader("X-Test", "yes"),
		goclient.OptUserAgent("test-agent"),
		goclient.OptTimeout(time.Second),
	))
	require.NoError(t, err)
	assert.Equal(t, time.Second, c.Client.Timeout, "go-client's own requests are timed out")

	// The streamable HTTP transport is tried first, and then SSE, with the
	// same client
	transports := c.transports()
	require.Len(t, transports, 2)
	streamable, ok := transports[0].(*sdkmcp.StreamableClientTransport)
	require.True(t, ok)
	sse, ok := transports[1].(*sdkmcp.SSEClientTransport)
	require.True(t, ok)
	assert.Equal(t, server.URL+"/mcp", streamable.Endpoint)
	assert.Equal(t, server.URL+"/mcp", sse.Endpoint)
	assert.Same(t, streamable.HTTPClient, sse.HTTPClient)
	assert.Zero(t, streamable.HTTPClient.Timeout, "but not a session's, whose streams last")
	assert.NotSame(t, c.Client.Client, streamable.HTTPClient, "a copy, so go-client's is unchanged")

	// Its requests have go-client's headers, and user agent
	response, err := streamable.HTTPClient.Get(server.URL + "/mcp")
	require.NoError(t, err)
	response.Body.Close()
	assert.Equal(t, "yes", got.Get("X-Test"))
	assert.Equal(t, "test-agent", got.Get("User-Agent"))
}
