package httpclient_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/auth/httpclient"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// newServer returns a server of a resource, at /mcp, which isn't authorized,
// with a challenge, whose metadata, at its well-known path, with its path,
// says its authorization server is the server, whose metadata is at the
// path, which can be OAuth's, or OpenID Connect's
func newServer(t *testing.T, serverPath string) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="mcp", resource_metadata="`+server.URL+`/.well-known/oauth-protected-resource/mcp", error="invalid_token", error_description="The token \"expired\""`)
		w.WriteHeader(http.StatusUnauthorized)
	})
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"resource": server.URL + "/mcp", "authorization_servers": []string{server.URL + "/auth"}, "scopes_supported": []string{"read"}})
	})
	mux.HandleFunc("/"+serverPath+"/auth", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                           server.URL + "/auth",
			"authorization_endpoint":           server.URL + "/auth/authorize",
			"token_endpoint":                   server.URL + "/auth/token",
			"registration_endpoint":            server.URL + "/auth/register",
			"code_challenge_methods_supported": []string{"S256"},
		})
	})
	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// writeJSON writes the value, as JSON
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func Test_Challenge(t *testing.T) {
	// A challenge's scheme, and parameters, quoted, or not, with escapes
	header := http.Header{}
	header.Set("WWW-Authenticate", `Bearer realm="example", error=invalid_token, error_description="said \"no\""`)
	challenge := httpclient.ParseChallenge(header)
	require.NotNil(t, challenge)
	assert.Equal(t, "Bearer", challenge.Scheme)
	assert.Equal(t, "example", challenge.Get("realm"))
	assert.Equal(t, "invalid_token", challenge.Get("error"))
	assert.Equal(t, `said "no"`, challenge.Get("error_description"))
	assert.Equal(t, `unauthorized: invalid_token: said "no"`, challenge.Error())

	// And none, without one
	assert.Nil(t, httpclient.ParseChallenge(http.Header{}))
	header.Set("WWW-Authenticate", "Bearer")
	assert.Equal(t, "unauthorized", httpclient.ParseChallenge(header).Error())
}

func Test_Discover(t *testing.T) {
	ctx := t.Context()

	for name, path := range map[string]string{"OAuth": ".well-known/oauth-authorization-server", "OIDC": ".well-known/openid-configuration"} {
		t.Run(name, func(t *testing.T) {
			server := newServer(t, path)
			c, err := httpclient.New(server.URL + "/mcp")
			require.NoError(t, err)
			assert.Equal(t, server.URL+"/mcp", c.Endpoint())

			// A request which isn't authorized is an AuthError, with its
			// challenge
			err = c.DoAuthWithContext(ctx, client.MethodGet, nil)
			authErr := httpclient.AsAuthError(err)
			require.NotNil(t, authErr, "%v", err)
			assert.Equal(t, "mcp", authErr.Get("realm"))

			// From which how a token is got is discovered, and from the
			// resource, as well
			for _, discover := range []func() (*httpclient.Metadata, error){
				func() (*httpclient.Metadata, error) { return c.DiscoverWithError(ctx, err) },
				func() (*httpclient.Metadata, error) { return c.Discover(ctx, "") },
			} {
				metadata, err := discover()
				require.NoError(t, err)
				assert.Equal(t, server.URL+"/mcp", metadata.Resource.Resource)
				assert.Equal(t, []string{"read"}, metadata.Resource.ScopesSupported)
				require.Len(t, metadata.AuthorizationServers, 1)
				as := metadata.AuthorizationServers[0]
				assert.Equal(t, server.URL+"/auth", as.Issuer)
				assert.Equal(t, server.URL+"/auth/authorize", as.AuthorizationEndpoint)
				assert.Equal(t, server.URL+"/auth/token", as.TokenEndpoint)
				assert.Equal(t, server.URL+"/auth/register", as.RegistrationEndpoint)
				assert.Equal(t, []string{"S256"}, as.CodeChallengeMethodsSupported)
			}
		})
	}

	t.Run("NoMetadata", func(t *testing.T) {
		// A resource without metadata, nor an authorization server, has none
		server := httptest.NewServer(http.NotFoundHandler())
		t.Cleanup(server.Close)
		c, err := httpclient.New(server.URL + "/mcp")
		require.NoError(t, err)
		_, err = c.Discover(ctx, "")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = c.DiscoverWithError(ctx, llm.ErrBadParameter)
		assert.ErrorIs(t, err, llm.ErrBadParameter, "which isn't an AuthError")
	})

	t.Run("Failures", func(t *testing.T) {
		// An authorization server whose metadata fails, or isn't metadata,
		// says why, rather than only that there's none
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/.well-known/oauth-authorization-server":
				http.Error(w, "broken", http.StatusInternalServerError)
			case "/.well-known/openid-configuration":
				writeJSON(w, map[string]any{"issuer": "nothing useful"})
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(server.Close)
		c, err := httpclient.New(server.URL + "/mcp")
		require.NoError(t, err)
		_, err = c.Discover(ctx, "")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		assert.Contains(t, err.Error(), "500")
		assert.Contains(t, err.Error(), "has no token_endpoint")
		assert.NotContains(t, err.Error(), "404", "what's missing isn't a failure")
	})

	t.Run("Origin", func(t *testing.T) {
		// A resource without metadata has its origin as its authorization
		// server
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/.well-known/oauth-authorization-server" {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, map[string]any{"issuer": server.URL, "token_endpoint": server.URL + "/token"})
		}))
		t.Cleanup(server.Close)
		c, err := httpclient.New(server.URL + "/mcp")
		require.NoError(t, err)
		metadata, err := c.Discover(ctx, "")
		require.NoError(t, err)
		require.Len(t, metadata.AuthorizationServers, 1)
		assert.Equal(t, server.URL+"/token", metadata.AuthorizationServers[0].TokenEndpoint)
	})
}
