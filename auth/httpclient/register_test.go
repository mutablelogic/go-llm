package httpclient_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/auth/httpclient"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Register(t *testing.T) {
	ctx := t.Context()
	// A server which doesn't register a client with scopes, as some don't,
	// even those they say they support, and does without
	var scopes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Scope        string   `json:"scope"`
			RedirectURIs []string `json:"redirect_uris"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		scopes = append(scopes, req.Scope)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/broken":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_redirect_uri"})
		case req.Scope != "":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_client_metadata", "error_description": "Requested scopes are invalid or not supported"})
		default:
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "client-1", "redirect_uris": req.RedirectURIs})
		}
	}))
	t.Cleanup(server.Close)
	c, err := httpclient.New(server.URL + "/mcp")
	require.NoError(t, err)

	// It's registered again, without its scopes, which it's asked for as
	// it's authorized
	registration, err := c.Register(ctx, httpclient.ServerMetadata{Issuer: server.URL, RegistrationEndpoint: server.URL + "/register"}, "test", []string{"http://localhost/cb"}, "read", "write")
	require.NoError(t, err)
	assert.Equal(t, "client-1", registration.ClientID)
	assert.Equal(t, []string{"read write", ""}, scopes)

	// But not again, when it isn't registered without them, either, nor
	// when it's without them already
	scopes = nil
	_, err = c.Register(ctx, httpclient.ServerMetadata{Issuer: server.URL, RegistrationEndpoint: server.URL + "/broken"}, "test", []string{"http://localhost/cb"}, "read")
	assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	assert.Equal(t, []string{"read", ""}, scopes)
	scopes = nil
	_, err = c.Register(ctx, httpclient.ServerMetadata{Issuer: server.URL, RegistrationEndpoint: server.URL + "/broken"}, "test", []string{"http://localhost/cb"})
	assert.Error(t, err)
	assert.Equal(t, []string{""}, scopes)

	// And a server which doesn't register clients needs one's identifier
	_, err = c.Register(ctx, httpclient.ServerMetadata{Issuer: server.URL}, "test", []string{"http://localhost/cb"})
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
}
