package httphandler_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

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

func Test_Authorize(t *testing.T) {
	r, err := registry.New()
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- r.Run(ctx, nil) }()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-done)
		assert.NoError(t, r.Close())
	})
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(r, router)
	})
	mcp := mcptest.NewServer(t, mcptest.WithAuth())

	// A connector whose server needs to be authorized is created, and says
	// so, once it's connected to
	var created schema.Connector
	require.Equal(t, http.StatusCreated, server.JSON(t, http.MethodPost, "mcp/connector", schema.Connector{Name: "secure", ConnectorMeta: schema.ConnectorMeta{URL: mcp.URL}}, &created))
	require.Eventually(t, func() bool {
		var got schema.Connector
		return server.JSON(t, http.MethodGet, "mcp/connector/secure", nil, &got) == http.StatusOK && got.Auth == schema.AuthRequired
	}, 5*time.Second, 10*time.Millisecond)

	// And it's saved, with its URL as it was, without being connected to
	// first, so it still needs to be
	var saved schema.Connector
	require.Equal(t, http.StatusOK, server.JSON(t, http.MethodPatch, "mcp/connector/secure", schema.ConnectorMeta{URL: mcp.URL, Include: []string{"echo"}}, &saved))
	assert.Equal(t, []string{"echo"}, saved.Include)
	require.Eventually(t, func() bool {
		var got schema.Connector
		return server.JSON(t, http.MethodGet, "mcp/connector/secure", nil, &got) == http.StatusOK && got.Auth == schema.AuthRequired
	}, 5*time.Second, 10*time.Millisecond)

	t.Run("Origin", func(t *testing.T) {
		// Its callback is the API's, of the browser's origin, when the
		// request has no redirect
		req, err := http.NewRequest(http.MethodPost, server.URL+test.Prefix+"/mcp/connector/secure/authorize", strings.NewReader("{}"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://llm.example.ts.net")
		resp, err := server.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var authorize schema.AuthorizeResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&authorize))
		assert.Equal(t, "https://llm.example.ts.net"+test.Prefix+registry.CallbackPath, query(t, authorize.URL, "redirect_uri"))

		// But it's needed, when it has no origin
		assert.Equal(t, http.StatusBadRequest, server.JSON(t, http.MethodPost, "mcp/connector/secure/authorize", schema.AuthorizeRequest{}, nil))
		assert.Equal(t, http.StatusNotFound, server.JSON(t, http.MethodPost, "mcp/connector/missing/authorize", schema.AuthorizeRequest{Redirect: "http://localhost/cb"}, nil))
	})

	t.Run("Callback", func(t *testing.T) {
		// It's authorized, with the redirect, and the callback connects it
		callback := server.URL + test.Prefix + registry.CallbackPath
		var authorize schema.AuthorizeResponse
		require.Equal(t, http.StatusOK, server.JSON(t, http.MethodPost, "mcp/connector/secure/authorize", schema.AuthorizeRequest{Redirect: callback}, &authorize))
		redirect, err := mcp.Approve(authorize.URL)
		require.NoError(t, err)
		u, err := url.Parse(redirect)
		require.NoError(t, err)
		var connector schema.Connector
		require.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, "mcp/callback?"+u.RawQuery, nil, &connector))
		assert.Equal(t, "secure", connector.Name)
		require.Eventually(t, func() bool {
			var got schema.Connector
			return server.JSON(t, http.MethodGet, "mcp/connector/secure", nil, &got) == http.StatusOK && got.Connected && got.Auth == schema.AuthAuthorized
		}, 5*time.Second, 10*time.Millisecond)

		// But not twice, nor once a person says no
		assert.Equal(t, http.StatusNotFound, server.JSON(t, http.MethodGet, "mcp/callback?"+u.RawQuery, nil, nil))
		require.Equal(t, http.StatusOK, server.JSON(t, http.MethodPost, "mcp/connector/secure/authorize", schema.AuthorizeRequest{Redirect: callback}, &authorize))
		assert.Equal(t, http.StatusBadRequest, server.JSON(t, http.MethodGet, "mcp/callback?error=access_denied&state="+query(t, authorize.URL, "state"), nil, nil))
	})

	t.Run("Page", func(t *testing.T) {
		// A person's browser is shown a page, rather than JSON
		resp, err := http.Get(server.URL + test.Prefix + registry.CallbackPath + "?state=missing&code=x")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.True(t, strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html"))
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "Not signed in")
	})
}

// query returns the URL's query parameter
func query(t *testing.T, raw, key string) string {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u.Query().Get(key)
}

func Test_AuthorizePublicURL(t *testing.T) {
	// A server which says its URL is https, as it's behind a proxy, whose
	// name it was given, sends a browser back to its own URL, rather than
	// the request's origin, which the proxy may say is its own, but one
	// which says it's http, at its listen address, doesn't
	r, err := registry.New()
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- r.Run(ctx, nil) }()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-done)
		assert.NoError(t, r.Close())
	})
	var serverURL *url.URL
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(r, router, httphandler.WithURL(func() *url.URL { return serverURL }))
	})
	mcp := mcptest.NewServer(t, mcptest.WithAuth())
	require.Equal(t, http.StatusCreated, server.JSON(t, http.MethodPost, "mcp/connector", schema.Connector{Name: "secure", ConnectorMeta: schema.ConnectorMeta{URL: mcp.URL}}, nil))
	require.Eventually(t, func() bool {
		var got schema.Connector
		return server.JSON(t, http.MethodGet, "mcp/connector/secure", nil, &got) == http.StatusOK && got.Auth == schema.AuthRequired
	}, 5*time.Second, 10*time.Millisecond)

	authorize := func(t *testing.T) string {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, server.URL+test.Prefix+"/mcp/connector/secure/authorize", strings.NewReader("{}"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://llm-http-lan")
		resp, err := server.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var response schema.AuthorizeResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&response))
		return query(t, response.URL, "redirect_uri")
	}

	serverURL = &url.URL{Scheme: "https", Host: "llm.example.com", Path: "/api"}
	assert.Equal(t, "https://llm.example.com/api"+registry.CallbackPath, authorize(t))
	serverURL = &url.URL{Scheme: "http", Host: "0.0.0.0:8086", Path: "/api"}
	assert.Equal(t, "https://llm-http-lan"+test.Prefix+registry.CallbackPath, authorize(t))
}
