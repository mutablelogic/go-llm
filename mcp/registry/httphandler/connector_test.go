package httphandler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	httphandler "github.com/mutablelogic/go-llm/mcp/registry/httphandler"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	mcptest "github.com/mutablelogic/go-llm/mcp/test"
	test "github.com/mutablelogic/go-llm/registry/test"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Connectors(t *testing.T) {
	r, err := registry.New()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, r.Close()) })
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(r, router)
	})
	connector := func(name, url string) schema.Connector {
		return schema.Connector{Name: name, ConnectorMeta: schema.ConnectorMeta{URL: url}}
	}

	t.Run("Empty", func(t *testing.T) {
		var list schema.ConnectorList
		assert.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, "mcp/connector", nil, &list))
		assert.Zero(t, list.Count)
		assert.Empty(t, list.Body)
	})

	t.Run("Create", func(t *testing.T) {
		// Connectors are created, with their serials, and listed, in order of
		// their names
		var created schema.Connector
		require.Equal(t, http.StatusCreated, server.JSON(t, http.MethodPost, "mcp/connector", connector("search", "https://mcp.example.com/search"), &created))
		assert.Equal(t, "search", created.Name)
		assert.Equal(t, uint64(1), created.Serial)
		require.Equal(t, http.StatusCreated, server.JSON(t, http.MethodPost, "mcp/connector", connector("github", "https://mcp.example.com/github"), nil))
		var list schema.ConnectorList
		require.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, "mcp/connector", nil, &list))
		assert.Equal(t, uint(2), list.Count)
		if assert.Len(t, list.Body, 2) {
			assert.Equal(t, "github", list.Body[0].Name)
			assert.Equal(t, "search", list.Body[1].Name)
		}

		// But not twice, nor with a name which isn't an identifier, or a URL
		// which isn't a remote server's
		assert.Equal(t, http.StatusConflict, server.JSON(t, http.MethodPost, "mcp/connector", connector("search", "https://mcp.example.com/other"), nil))
		assert.Equal(t, http.StatusBadRequest, server.JSON(t, http.MethodPost, "mcp/connector", connector("9lives", "https://mcp.example.com"), nil))
		assert.Equal(t, http.StatusBadRequest, server.JSON(t, http.MethodPost, "mcp/connector", connector("local", "stdio://server"), nil))
	})

	t.Run("Get", func(t *testing.T) {
		var got schema.Connector
		require.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, "mcp/connector/search", nil, &got))
		assert.Equal(t, "https://mcp.example.com/search", got.URL)
		assert.Equal(t, http.StatusNotFound, server.JSON(t, http.MethodGet, "mcp/connector/missing", nil, nil))
	})

	t.Run("Update", func(t *testing.T) {
		// A connector's URL is replaced, once its server is connected, and
		// its serial changes, but not with one which can't be
		mcp := mcptest.NewServer(t)
		var updated schema.Connector
		require.Equal(t, http.StatusOK, server.JSON(t, http.MethodPatch, "mcp/connector/search", schema.ConnectorMeta{URL: mcp.URL}, &updated))
		assert.Equal(t, "search", updated.Name)
		assert.Equal(t, mcp.URL, updated.URL)
		assert.Equal(t, uint64(2), updated.Serial)
		assert.True(t, updated.Connected)
		notFound := httptest.NewServer(http.NotFoundHandler())
		t.Cleanup(notFound.Close)
		assert.Equal(t, http.StatusServiceUnavailable, server.JSON(t, http.MethodPatch, "mcp/connector/search", schema.ConnectorMeta{URL: notFound.URL}, nil))
		assert.Equal(t, http.StatusBadRequest, server.JSON(t, http.MethodPatch, "mcp/connector/search", schema.ConnectorMeta{URL: "ftp://mcp.example.com"}, nil))
		assert.Equal(t, http.StatusNotFound, server.JSON(t, http.MethodPatch, "mcp/connector/missing", schema.ConnectorMeta{URL: "https://mcp.example.com"}, nil))
	})

	t.Run("Delete", func(t *testing.T) {
		var deleted schema.Connector
		require.Equal(t, http.StatusOK, server.JSON(t, http.MethodDelete, "mcp/connector/search", nil, &deleted))
		assert.NotEqual(t, "https://mcp.example.com/search", deleted.URL)
		assert.Equal(t, http.StatusNotFound, server.JSON(t, http.MethodGet, "mcp/connector/search", nil, nil))
		assert.Equal(t, http.StatusNotFound, server.JSON(t, http.MethodDelete, "mcp/connector/search", nil, nil))
		var list schema.ConnectorList
		require.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, "mcp/connector", nil, &list))
		assert.Equal(t, uint(1), list.Count)
	})
}
