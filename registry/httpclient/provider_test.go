package httpclient_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// newClient returns a client of a test server of a registry, with eliza,
// whose requests and responses are checked against the API's OpenAPI
// specification when the test ends
func newClient(t *testing.T) *httpclient.Client {
	t.Helper()
	provider, err := eliza.New()
	require.NoError(t, err)
	server := test.NewServer(t, test.NewRegistry(t, []llm.Provider{provider}))
	client, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)
	return client
}

// newOllama returns a stub Ollama server, which replies to pings
func newOllama(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/version") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"version":"0.0.1"}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	return server
}

// status returns the HTTP status of an error from the client
func status(t *testing.T, err error) int {
	t.Helper()
	var response httpresponse.ErrResponse
	require.True(t, errors.As(err, &response), "error is %T: %v", err, err)
	return response.Code
}

func Test_Providers(t *testing.T) {
	ctx := t.Context()
	client := newClient(t)
	ollama := newOllama(t)

	t.Run("Create", func(t *testing.T) {
		// A provider is created with its name, and what it can do
		provider, err := client.CreateProvider(ctx, schema.CreateProviderRequest{Name: "home", Provider: llm.Ollama, URL: ollama.URL})
		require.NoError(t, err)
		assert.Equal(t, "home", provider.Name)
		assert.Equal(t, llm.Ollama, provider.Provider)
		assert.True(t, provider.Downloader)
		assert.Empty(t, provider.Error)
	})

	t.Run("CreateErrors", func(t *testing.T) {
		_, err := client.CreateProvider(ctx, schema.CreateProviderRequest{Provider: llm.Eliza})
		assert.Equal(t, http.StatusConflict, status(t, err))
		_, err = client.CreateProvider(ctx, schema.CreateProviderRequest{Provider: "unknown"})
		assert.Equal(t, http.StatusBadRequest, status(t, err))
		_, err = client.CreateProvider(ctx, schema.CreateProviderRequest{Name: "1st", Provider: llm.Eliza})
		assert.Equal(t, http.StatusBadRequest, status(t, err))
	})

	t.Run("List", func(t *testing.T) {
		// Every provider, in order of their names
		list, err := client.ListProviders(ctx, schema.ProviderListRequest{})
		require.NoError(t, err)
		assert.Equal(t, uint(2), list.Count)
		if assert.Len(t, list.Body, 2) {
			assert.Equal(t, "eliza", list.Body[0].Name)
			assert.Equal(t, "home", list.Body[1].Name)
		}

		// Or those of a type
		list, err = client.ListProviders(ctx, schema.ProviderListRequest{Provider: llm.Ollama})
		require.NoError(t, err)
		if assert.Len(t, list.Body, 1) {
			assert.Equal(t, "home", list.Body[0].Name)
		}
		assert.Equal(t, llm.Ollama, list.Provider)
	})

	t.Run("Get", func(t *testing.T) {
		provider, err := client.GetProvider(ctx, "eliza")
		require.NoError(t, err)
		assert.Equal(t, "eliza", provider.Name)
		assert.True(t, provider.Generator)

		_, err = client.GetProvider(ctx, "missing")
		assert.Equal(t, http.StatusNotFound, status(t, err))
	})

	t.Run("Delete", func(t *testing.T) {
		provider, err := client.DeleteProvider(ctx, "home")
		require.NoError(t, err)
		assert.Equal(t, "home", provider.Name)

		_, err = client.GetProvider(ctx, "home")
		assert.Equal(t, http.StatusNotFound, status(t, err))
		_, err = client.DeleteProvider(ctx, "home")
		assert.Equal(t, http.StatusNotFound, status(t, err))
	})
}
