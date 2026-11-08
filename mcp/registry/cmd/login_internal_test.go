package cmd

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/mcp/registry/httpclient"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Callback(t *testing.T) {
	// The server's own callback is used, when the browser can be
	// redirected to it, as it's https, or a loopback address
	assert.Equal(t, "https://llm.example.ts.net/api/mcp/callback", callback("https://llm.example.ts.net/api"))
	assert.Equal(t, "http://localhost:8084/api/mcp/callback", callback("http://localhost:8084/api/"))
	assert.Equal(t, "http://127.0.0.1:9999/api/mcp/callback", callback("http://127.0.0.1:9999/api"))
	assert.Equal(t, "http://[::1]:9999/api/mcp/callback", callback("http://[::1]:9999/api"))

	// But not otherwise, so a loopback is
	assert.Empty(t, callback("http://nestor:9999/api"))
	assert.Empty(t, callback("http://192.168.1.10:9999/api"))
	assert.Empty(t, callback("unix:///tmp/llm.sock"))
}

func Test_Loopback(t *testing.T) {
	// The loopback passes the query it's redirected with on to the server's
	// callback, and shows the person whether they're signed in
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/mcp/callback", r.URL.Path)
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("error") != "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"code":400,"reason":"access_denied"}`)
			return
		}
		_, _ = io.WriteString(w, `{"name":"secure","url":"https://mcp.example.com"}`)
	}))
	t.Cleanup(server.Close)
	client, err := httpclient.New(server.URL + "/api")
	require.NoError(t, err)
	l, err := newLoopback(client)
	require.NoError(t, err)
	t.Cleanup(l.Close)
	l.expect("abc")
	assert.True(t, strings.HasPrefix(l.URL(), "http://127.0.0.1:"))
	assert.True(t, strings.HasSuffix(l.URL(), loopbackPath))

	get := func(rawQuery string) (int, string) {
		t.Helper()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, l.URL()+"?"+rawQuery, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, string(body)
	}
	code, body := get("state=abc&code=xyz")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Signed in to secure")
	assert.Equal(t, "code=xyz&state=abc", query)

	// A callback of another sign-in, or which isn't one, is refused, and
	// isn't passed on, nor ends the sign-in
	query = ""
	for _, rawQuery := range []string{"state=stale&code=xyz", "code=xyz", "state=&error=access_denied"} {
		code, body = get(rawQuery)
		assert.Equal(t, http.StatusBadRequest, code, rawQuery)
		assert.Contains(t, body, "the sign-in the command line is waiting for", rawQuery)
	}
	assert.Empty(t, query, "it isn't passed on")
	select {
	case err := <-l.failed:
		t.Errorf("the sign-in was ended by another's callback: %v", err)
	default:
	}

	// And says why they aren't, to the person, and the command
	code, body = get("state=abc&error=access_denied")
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, body, "Not signed in")
	select {
	case err := <-l.failed:
		assert.Error(t, err)
	default:
		t.Error("the command isn't told it wasn't signed in")
	}
}
