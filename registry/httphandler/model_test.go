package httphandler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// newModelServer returns a server of a registry with two eliza providers,
// eliza and second, whose German model is excluded, and the registry
func newModelServer(t *testing.T) (*test.Server, *registry.Registry) {
	t.Helper()
	first, err := eliza.New()
	require.NoError(t, err)
	second, err := eliza.New()
	require.NoError(t, err)
	r := test.NewRegistry(t, []llm.Provider{first})
	require.NoError(t, r.Add("second", second, registry.WithExclude("-de$")))
	return test.NewServer(t, r), r
}

// names returns the canonical names of the models in the list
func names(list schema.ModelList) []string {
	var result []string
	for _, model := range list.Body {
		result = append(result, model.Canonical)
	}
	return result
}

func Test_ListModels(t *testing.T) {
	server, r := newModelServer(t)

	t.Run("List", func(t *testing.T) {
		// The models of every provider, which match their patterns, in
		// order of their providers' names, and then their names
		var list schema.ModelList
		assert.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/model", nil, &list))
		assert.Equal(t, []string{
			"eliza:eliza-1966-de", "eliza:eliza-1966-en", "eliza:eliza-1966-fr",
			"second:eliza-1966-en", "second:eliza-1966-fr",
		}, names(list))
		assert.Equal(t, uint(5), list.Count)
		if assert.NotEmpty(t, list.Body) {
			model := list.Body[0]
			assert.Equal(t, "eliza-1966-de", model.Name)
			assert.Equal(t, "eliza", model.Provider)
			assert.Equal(t, `"de"`, string(model.Meta["language"]))
			if assert.NotNil(t, model.Capabilities.Completion) {
				assert.True(t, *model.Capabilities.Completion)
			}
		}
	})

	t.Run("Provider", func(t *testing.T) {
		// The models of a provider
		var list schema.ModelList
		assert.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/model?provider=second", nil, &list))
		assert.Equal(t, []string{"second:eliza-1966-en", "second:eliza-1966-fr"}, names(list))
		assert.Equal(t, "second", list.Provider)

		// Which must exist
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodGet, "llm/model?provider=missing", nil, nil))
	})

	t.Run("Unreachable", func(t *testing.T) {
		// The models of a provider which can't be reached are left out
		closed := httptest.NewServer(http.NotFoundHandler())
		closed.Close()
		client, err := ollama.New(closed.URL)
		require.NoError(t, err)
		require.NoError(t, r.Add("away", client))
		defer r.Remove("away")
		var list schema.ModelList
		assert.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/model", nil, &list))
		assert.Equal(t, uint(5), list.Count)

		// And when there are only its models, it's an error
		assert.Equal(t, http.StatusServiceUnavailable, do(t, server, http.MethodGet, "llm/model?provider=away", nil, nil))
	})
}

func Test_GetModel(t *testing.T) {
	server, r := newModelServer(t)

	t.Run("Canonical", func(t *testing.T) {
		var model schema.Model
		assert.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/model/second:eliza-1966-fr", nil, &model))
		assert.Equal(t, "eliza-1966-fr", model.Name)
		assert.Equal(t, "second", model.Provider)
		assert.Equal(t, "second:eliza-1966-fr", model.Canonical)
	})

	t.Run("Name", func(t *testing.T) {
		// A name which one provider has, as the other excludes it
		var model schema.Model
		assert.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/model/eliza-1966-de", nil, &model))
		assert.Equal(t, "eliza:eliza-1966-de", model.Canonical)
	})

	t.Run("Ambiguous", func(t *testing.T) {
		// A name, or alias, which both providers have
		assert.Equal(t, http.StatusConflict, do(t, server, http.MethodGet, "llm/model/eliza-1966-en", nil, nil))
		assert.Equal(t, http.StatusConflict, do(t, server, http.MethodGet, "llm/model/eliza", nil, nil))
	})

	t.Run("Errors", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodGet, "llm/model/missing", nil, nil))
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodGet, "llm/model/second:eliza-1966-de", nil, nil))
	})

	t.Run("Unreachable", func(t *testing.T) {
		// A model which no provider has, when one which could can't be
		// reached
		closed := httptest.NewServer(http.NotFoundHandler())
		closed.Close()
		client, err := ollama.New(closed.URL)
		require.NoError(t, err)
		require.NoError(t, r.Add("away", client))
		defer r.Remove("away")
		assert.Equal(t, http.StatusServiceUnavailable, do(t, server, http.MethodGet, "llm/model/llama3.2:latest", nil, nil))
		assert.Equal(t, http.StatusServiceUnavailable, do(t, server, http.MethodGet, "llm/model/away:llama3.2:latest", nil, nil))

		// But a model which a provider has is found
		assert.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/model/eliza:eliza-1966-de", nil, nil))
	})
}
