package httpclient_test

import (
	"net/http"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Models(t *testing.T) {
	ctx := t.Context()
	client := newClient(t)

	t.Run("List", func(t *testing.T) {
		list, err := client.ListModels(ctx, schema.ModelListRequest{})
		require.NoError(t, err)
		assert.Equal(t, uint(3), list.Count)
		if assert.Len(t, list.Body, 3) {
			assert.Equal(t, "eliza:eliza-1966-de", list.Body[0].Canonical)
			assert.Equal(t, "eliza", list.Body[0].Provider)
		}
	})

	t.Run("Get", func(t *testing.T) {
		// By alias, or provider and name
		model, err := client.GetModel(ctx, "eliza")
		require.NoError(t, err)
		assert.Equal(t, "eliza:eliza-1966-en", model.Canonical)
		model, err = client.GetModel(ctx, "eliza:eliza-1966-fr")
		require.NoError(t, err)
		assert.Equal(t, "eliza-1966-fr", model.Name)

		_, err = client.GetModel(ctx, "missing")
		assert.Equal(t, http.StatusNotFound, status(t, err))
	})

	t.Run("Providers", func(t *testing.T) {
		// With another provider, whose French model is excluded, the models
		// of each provider are listed
		_, err := client.CreateProvider(ctx, schema.Provider{Name: "second", Type: llm.Eliza, Exclude: []string{"-fr$"}})
		require.NoError(t, err)
		list, err := client.ListModels(ctx, schema.ModelListRequest{Provider: "second"})
		require.NoError(t, err)
		var names []string
		for _, model := range list.Body {
			names = append(names, model.Canonical)
		}
		assert.Equal(t, []string{"second:eliza-1966-de", "second:eliza-1966-en"}, names)
		_, err = client.ListModels(ctx, schema.ModelListRequest{Provider: "missing"})
		assert.Equal(t, http.StatusNotFound, status(t, err))

		// And a name which both have needs its provider
		_, err = client.GetModel(ctx, "eliza")
		assert.Equal(t, http.StatusConflict, status(t, err))
		model, err := client.GetModel(ctx, "eliza-1966-fr")
		require.NoError(t, err)
		assert.Equal(t, "eliza:eliza-1966-fr", model.Canonical)
	})
}
