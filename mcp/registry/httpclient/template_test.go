package httpclient_test

import (
	"testing"

	// Packages
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	httpclient "github.com/mutablelogic/go-llm/mcp/registry/httpclient"
	httphandler "github.com/mutablelogic/go-llm/mcp/registry/httphandler"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Templates(t *testing.T) {
	// The templates which are offered are listed, without the names of
	// credentials which aren't the template's
	r, err := registry.New(registry.WithTemplates(schema.ConnectorTemplate{Name: "Example", Description: "An example", URL: "https://mcp.example.com/mcp"}))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, r.Close()) })
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(r, router)
	})
	c, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)
	list, err := c.ListTemplates(t.Context())
	require.NoError(t, err)
	assert.Equal(t, uint(1), list.Count)
	if assert.Len(t, list.Body, 1) {
		assert.Equal(t, "Example", list.Body[0].Name)
		assert.Equal(t, "https://mcp.example.com/mcp", list.Body[0].URL)
		assert.Empty(t, list.Body[0].ClientIDCredential)
	}
}
