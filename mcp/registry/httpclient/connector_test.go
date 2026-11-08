package httpclient_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	// Packages
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	httpclient "github.com/mutablelogic/go-llm/mcp/registry/httpclient"
	httphandler "github.com/mutablelogic/go-llm/mcp/registry/httphandler"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	mcptest "github.com/mutablelogic/go-llm/mcp/test"
	test "github.com/mutablelogic/go-llm/registry/test"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// status returns the HTTP status of the error of a request
func status(t *testing.T, err error) int {
	t.Helper()
	var response httpresponse.ErrResponse
	require.True(t, errors.As(err, &response), "error is %T: %v", err, err)
	return response.Code
}

func Test_Connectors(t *testing.T) {
	ctx := t.Context()
	r, err := registry.New()
	require.NoError(t, err)
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(r, router)
	})
	c, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)
	mcp := mcptest.NewServer(t)
	connector := func(name, url string) schema.Connector {
		return schema.Connector{Name: name, ConnectorMeta: schema.ConnectorMeta{URL: url}}
	}

	// None, and then those which are created, in order of their names
	list, err := c.ListConnectors(ctx)
	require.NoError(t, err)
	assert.Zero(t, list.Count)
	created, err := c.CreateConnector(ctx, connector("search", mcp.URL))
	require.NoError(t, err)
	assert.Equal(t, "search", created.Name)
	assert.Equal(t, uint64(1), created.Serial)
	_, err = c.CreateConnector(ctx, connector("github", "https://mcp.example.com/github"))
	require.NoError(t, err)
	list, err = c.ListConnectors(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint(2), list.Count)
	if assert.Len(t, list.Body, 2) {
		assert.Equal(t, "github", list.Body[0].Name)
		assert.Equal(t, "search", list.Body[1].Name)
	}

	// Which can't be created twice, or with a name, or URL, which isn't one
	_, err = c.CreateConnector(ctx, connector("search", mcp.URL))
	assert.Equal(t, http.StatusConflict, status(t, err))
	_, err = c.CreateConnector(ctx, connector("local", "stdio://server"))
	assert.Equal(t, http.StatusBadRequest, status(t, err))

	// A connector has its status, once the registry runs
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error)
	go func() { done <- r.Run(runCtx, nil) }()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-done)
	})
	require.Eventually(t, func() bool {
		got, err := c.GetConnector(ctx, "search")
		return err == nil && got.Connected
	}, 5*time.Second, 10*time.Millisecond)
	got, err := c.GetConnector(ctx, "search")
	require.NoError(t, err)
	assert.Equal(t, &schema.ConnectorServer{Name: mcptest.Name, Version: mcptest.Version}, got.Server)
	_, err = c.GetConnector(ctx, "missing")
	assert.Equal(t, http.StatusNotFound, status(t, err))

	// It's updated, and its serial changes
	updated, err := c.UpdateConnector(ctx, "github", schema.ConnectorMeta{URL: "https://mcp.example.com/v2"})
	require.NoError(t, err)
	assert.Equal(t, "https://mcp.example.com/v2", updated.URL)
	assert.Equal(t, uint64(2), updated.Serial)
	_, err = c.UpdateConnector(ctx, "github", schema.ConnectorMeta{URL: "ftp://mcp.example.com"})
	assert.Equal(t, http.StatusBadRequest, status(t, err))

	// And deleted
	deleted, err := c.DeleteConnector(ctx, "github")
	require.NoError(t, err)
	assert.Equal(t, "https://mcp.example.com/v2", deleted.URL)
	_, err = c.DeleteConnector(ctx, "github")
	assert.Equal(t, http.StatusNotFound, status(t, err))
}
