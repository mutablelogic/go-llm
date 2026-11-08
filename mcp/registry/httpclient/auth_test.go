package httpclient_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	// Packages
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	httpclient "github.com/mutablelogic/go-llm/mcp/registry/httpclient"
	httphandler "github.com/mutablelogic/go-llm/mcp/registry/httphandler"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	mcptest "github.com/mutablelogic/go-llm/mcp/test"
	test "github.com/mutablelogic/go-llm/registry/test"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Authorize(t *testing.T) {
	ctx := t.Context()
	r, err := registry.New()
	require.NoError(t, err)
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- r.Run(runCtx, nil) }()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-done)
		assert.NoError(t, r.Close())
	})
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(r, router)
	})
	c, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)
	mcp := mcptest.NewServer(t, mcptest.WithAuth())

	// A connector whose server needs to be signed in to is signed in to,
	// with a redirect, such as a loopback's, which passes the query it's
	// redirected with on to the callback, so it's connected
	_, err = c.CreateConnector(ctx, schema.Connector{Name: "secure", ConnectorMeta: schema.ConnectorMeta{URL: mcp.URL}})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		connector, err := c.GetConnector(ctx, "secure")
		return err == nil && connector.Auth == schema.AuthRequired
	}, 5*time.Second, 10*time.Millisecond)
	authorize, err := c.Authorize(ctx, "secure", "http://127.0.0.1:8977/callback")
	require.NoError(t, err)
	redirect, err := mcp.Approve(authorize.URL)
	require.NoError(t, err)
	u, err := url.Parse(redirect)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8977", u.Host)
	connector, err := c.Callback(ctx, u.Query())
	require.NoError(t, err)
	assert.Equal(t, "secure", connector.Name)
	require.Eventually(t, func() bool {
		connector, err := c.GetConnector(ctx, "secure")
		return err == nil && connector.Connected && connector.Auth == schema.AuthAuthorized
	}, 5*time.Second, 10*time.Millisecond)

	// But not twice, nor once a person says no
	_, err = c.Callback(ctx, u.Query())
	assert.Equal(t, http.StatusNotFound, status(t, err))
	authorize, err = c.Authorize(ctx, "secure", "http://127.0.0.1:8977/callback")
	require.NoError(t, err)
	state := mustState(t, authorize.URL)
	_, err = c.Callback(ctx, url.Values{"state": {state}, "error": {"access_denied"}})
	assert.Equal(t, http.StatusBadRequest, status(t, err))
	_, err = c.Authorize(ctx, "missing", "http://127.0.0.1:8977/callback")
	assert.Equal(t, http.StatusNotFound, status(t, err))
}

// mustState returns the state of the authorization URL
func mustState(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u.Query().Get("state")
}
