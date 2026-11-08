package client

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	// Packages
	client "github.com/mutablelogic/go-client"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// tokenServer returns a server which counts its requests, and those with
// the bearer token, and redirects to the URL, when it's set
func tokenServer(t *testing.T, redirect func() string) (*httptest.Server, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	var requests, tokens atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") != "" {
			tokens.Add(1)
		}
		if redirect != nil && r.URL.Path == "/redirect" {
			http.Redirect(w, r, redirect(), http.StatusFound)
		}
	}))
	t.Cleanup(server.Close)
	return server, &requests, &tokens
}

func Test_SameOrigin(t *testing.T) {
	other, otherRequests, otherTokens := tokenServer(t, nil)
	var server *httptest.Server
	server, _, tokens := tokenServer(t, func() string { return other.URL + "/stolen" })
	c, err := New(server.URL+"/mcp", WithClientOpts(client.OptReqToken(client.Token{Scheme: "Bearer", Value: "secret"})))
	require.NoError(t, err)
	httpClient := c.httpClient()

	t.Run("Origin", func(t *testing.T) {
		// The server's origin is sent the token
		response, err := httpClient.Get(server.URL + "/mcp")
		require.NoError(t, err)
		response.Body.Close()
		assert.Equal(t, int32(1), tokens.Load())
	})

	t.Run("Other", func(t *testing.T) {
		// But another origin, such as an SSE message endpoint, which the
		// server says, isn't sent anything
		_, err := httpClient.Post(other.URL+"/messages", "application/json", nil)
		assert.Error(t, err)
		assert.Zero(t, otherRequests.Load())
	})

	t.Run("Redirect", func(t *testing.T) {
		// Nor is it redirected to, with the token
		_, err := httpClient.Get(server.URL + "/redirect")
		assert.Error(t, err)
		assert.Zero(t, otherRequests.Load())
		assert.Zero(t, otherTokens.Load())
	})

	t.Run("SameOriginRedirect", func(t *testing.T) {
		// A redirect to the server's origin is followed
		same, _, sameTokens := tokenServer(t, nil)
		var to string
		redirecting, _, _ := tokenServer(t, func() string { return to })
		to = redirecting.URL + "/mcp"
		c, err := New(redirecting.URL, WithClientOpts(client.OptReqToken(client.Token{Scheme: "Bearer", Value: "secret"})))
		require.NoError(t, err)
		response, err := c.httpClient().Get(redirecting.URL + "/redirect")
		require.NoError(t, err)
		response.Body.Close()
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Zero(t, sameTokens.Load(), same.URL)
	})
}

func Test_Origin(t *testing.T) {
	// An origin is a URL's scheme, host, and port, with its default port
	assert.Equal(t, "https://example.com:443", originOf("https://Example.com/mcp"))
	assert.Equal(t, originOf("https://example.com:443/a"), originOf("https://example.com/b"))
	assert.Equal(t, originOf("http://example.com:80"), originOf("http://example.com"))
	assert.NotEqual(t, originOf("http://example.com"), originOf("https://example.com"))
	assert.NotEqual(t, originOf("https://example.com"), originOf("https://example.com:8443"))
	assert.NotEqual(t, originOf("https://example.com"), originOf("https://evil.example.com"))
	assert.Empty(t, originOf("https://:443/mcp"))
}

func Test_Options(t *testing.T) {
	t.Run("Endpoint", func(t *testing.T) {
		// go-client's endpoint is the URL, whatever its options
		server, requests, _ := tokenServer(t, nil)
		c, err := New(server.URL, WithClientOpts(client.OptEndpoint("http://localhost:1")))
		require.NoError(t, err)
		require.NoError(t, c.DoWithContext(t.Context(), client.MethodGet, nil, client.OptPath("ping")))
		assert.Equal(t, int32(1), requests.Load())
	})

	t.Run("Name", func(t *testing.T) {
		// An option which is used at once, more than once, is safe
		opt := WithName(" shared ", " 1.0 ")
		var wg sync.WaitGroup
		for range 10 {
			wg.Go(func() {
				c, err := New("https://mcp.example.com", opt)
				if assert.NoError(t, err) {
					assert.Equal(t, "shared", c.implementation.Name)
				}
			})
		}
		wg.Wait()
	})
}
