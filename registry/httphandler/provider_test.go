package httphandler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	// Packages
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// newServer returns a server of the registry's API, under /api, whose
// requests and responses are checked against its OpenAPI specification when
// the test ends, and the registry, which has no providers
func newServer(t *testing.T) (*test.Server, *registry.Registry) {
	t.Helper()
	r := test.NewRegistry(t, nil)
	return test.NewServer(t, r), r
}

// do makes a request to the server, with the body as JSON when it isn't
// nil, and returns the status, and decodes a successful response into v,
// when it isn't nil
func do(t *testing.T, server *test.Server, method, path string, body, v any) int {
	t.Helper()
	return server.JSON(t, method, path, body, v)
}

// newOllama returns a stub Ollama server, which replies to pings, and
// counts those with the key as a bearer token
func newOllama(t *testing.T, key string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var authorized atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer "+key {
			authorized.Add(1)
		}
		if strings.HasSuffix(r.URL.Path, "/version") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"version":"0.0.1"}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	return server, &authorized
}

func Test_Providers(t *testing.T) {
	server, r := newServer(t)

	t.Run("Empty", func(t *testing.T) {
		var list schema.ProviderList
		assert.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/provider", nil, &list))
		assert.Zero(t, list.Count)
		assert.Empty(t, list.Body)
	})

	t.Run("Create", func(t *testing.T) {
		// A provider is created with its type's name, and what it can do
		var provider schema.Provider
		assert.Equal(t, http.StatusCreated, do(t, server, http.MethodPost, "llm/provider", schema.CreateProviderRequest{Provider: "eliza"}, &provider))
		assert.Equal(t, "eliza", provider.Name)
		assert.Equal(t, "eliza", provider.Provider)
		assert.True(t, provider.Generator)
		assert.False(t, provider.Embedder)
		assert.NotNil(t, r.Provider("eliza"))

		// And another of its type, with a name
		assert.Equal(t, http.StatusCreated, do(t, server, http.MethodPost, "llm/provider", schema.CreateProviderRequest{Name: "second", Provider: "eliza"}, &provider))
		assert.Equal(t, "second", provider.Name)
		assert.Equal(t, "eliza", provider.Provider)
	})

	t.Run("CreateOllama", func(t *testing.T) {
		// An Ollama server, whose API key is sent, and isn't returned
		ollama, authorized := newOllama(t, "secret")
		var provider schema.Provider
		assert.Equal(t, http.StatusCreated, do(t, server, http.MethodPost, "llm/provider", schema.CreateProviderRequest{Name: "home", Provider: "ollama", URL: ollama.URL, APIKey: "secret"}, &provider))
		assert.Equal(t, "ollama", provider.Provider)
		assert.True(t, provider.Downloader)
		assert.Contains(t, provider.URL, ollama.URL)
		assert.NotZero(t, authorized.Load())
		data, err := json.Marshal(provider)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "secret")
	})

	t.Run("CreateErrors", func(t *testing.T) {
		assert.Equal(t, http.StatusConflict, do(t, server, http.MethodPost, "llm/provider", schema.CreateProviderRequest{Provider: "eliza"}, nil))
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPost, "llm/provider", schema.CreateProviderRequest{Provider: "unknown"}, nil))
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPost, "llm/provider", schema.CreateProviderRequest{Name: "a:b", Provider: "eliza"}, nil))
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPost, "llm/provider", schema.CreateProviderRequest{Name: "e", Provider: "eliza", URL: "http://localhost"}, nil))

		// A provider which can't be reached isn't added
		unreachable := httptest.NewServer(http.NotFoundHandler())
		unreachable.Close()
		assert.Equal(t, http.StatusServiceUnavailable, do(t, server, http.MethodPost, "llm/provider", schema.CreateProviderRequest{Name: "away", Provider: "ollama", URL: unreachable.URL}, nil))
		assert.Nil(t, r.Provider("away"))
	})

	t.Run("List", func(t *testing.T) {
		// In order of their names, or of a type
		var list schema.ProviderList
		assert.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/provider", nil, &list))
		var names []string
		for _, provider := range list.Body {
			names = append(names, provider.Name)
		}
		assert.Equal(t, []string{"eliza", "home", "second"}, names)
		assert.Equal(t, uint(3), list.Count)

		assert.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/provider?provider=ollama", nil, &list))
		if assert.Len(t, list.Body, 1) {
			assert.Equal(t, "home", list.Body[0].Name)
		}
	})

	t.Run("Get", func(t *testing.T) {
		var provider schema.Provider
		assert.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/provider/second", nil, &provider))
		assert.Equal(t, "second", provider.Name)
		assert.Empty(t, provider.Error)
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodGet, "llm/provider/missing", nil, nil))
	})

	t.Run("Unreachable", func(t *testing.T) {
		// A provider which can't be reached any more has an error, once it's
		// pinged
		ollama, _ := newOllama(t, "")
		var provider schema.Provider
		require.Equal(t, http.StatusCreated, do(t, server, http.MethodPost, "llm/provider", schema.CreateProviderRequest{Name: "gone", Provider: "ollama", URL: ollama.URL}, &provider))
		ollama.Close()

		// It's pinged again, rather than with the result of its last ping,
		// which is used within the ping interval
		interval := registry.PingInterval
		registry.PingInterval = 0
		defer func() { registry.PingInterval = interval }()
		assert.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/provider/gone", nil, &provider))
		assert.NotEmpty(t, provider.Error)
	})

	t.Run("Delete", func(t *testing.T) {
		var provider schema.Provider
		assert.Equal(t, http.StatusOK, do(t, server, http.MethodDelete, "llm/provider/second", nil, &provider))
		assert.Equal(t, "second", provider.Name)
		assert.Nil(t, r.Provider("second"))
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodDelete, "llm/provider/second", nil, nil))
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodGet, "llm/provider/second", nil, nil))
	})
}
