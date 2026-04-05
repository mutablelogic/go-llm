package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	// Packages
	authclient "github.com/djthorpe/go-auth/pkg/httpclient/auth"
	server "github.com/mutablelogic/go-llm/pkg/mcp/server"
)

// Test_connect_005: connectWithAuth returns UnauthorizedError when the server
// returns 401 and no authFn is configured.
func Test_connect_005(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="test"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	c, err := New(ts.URL, "test-client", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = c.connectWithAuth(ctx)
	if err := authclient.AsAuthError(err); err == nil {
		t.Fatalf("expected UnauthorizedError, got %v", err)
	}
}

// Test_connect_006: connectWithAuth calls authFn with the resolved discovery
// URL on 401, then retries the connection and succeeds.
func Test_connect_006(t *testing.T) {
	srv, err := server.New("auth-server", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	// Gate: before authFn runs every request returns 401; after it runs all
	// requests are forwarded to the real MCP server.
	// The handler must be created once so the SDK's session state is preserved
	// across the multiple HTTP round-trips that make up a single mc.Connect().
	mcpHandler := srv.Handler()
	var authDone atomic.Bool
	var discoveryHits atomic.Int32
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/oauth-authorization-server" {
			discoveryHits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q}`,
				ts.URL,
				ts.URL+"/authorize",
				ts.URL+"/token",
			)
			return
		}
		if authDone.Load() {
			mcpHandler.ServeHTTP(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate",
			fmt.Sprintf(`Bearer realm="test", authorization_server="%s"`, ts.URL))
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	var gotConfig *authclient.Config
	c, err := New(ts.URL, "test-client", "1.0.0", WithAuth(func(_ context.Context, config *authclient.Config) error {
		gotConfig = config
		authDone.Store(true) // open the gate before the retry
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := c.connectWithAuth(ctx)
	if err != nil {
		t.Fatalf("expected successful connect after auth, got %v", err)
	}
	session.Close()

	if gotConfig == nil {
		t.Fatal("expected authFn to receive discovery config")
	}
	if len(gotConfig.AuthorizationServers) != 1 {
		t.Fatalf("expected one authorization server, got %d", len(gotConfig.AuthorizationServers))
	}
	if gotConfig.AuthorizationServers[0].Issuer != ts.URL {
		t.Fatalf("authFn got issuer %q, want %q", gotConfig.AuthorizationServers[0].Issuer, ts.URL)
	}
	if discoveryHits.Load() == 0 {
		t.Fatal("expected authorization metadata discovery request")
	}
}

// Test_connect_007: connectWithAuth propagates errors returned by authFn.
func Test_connect_007(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="test"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	authErr := errors.New("auth failed")
	c, err := New(ts.URL, "test-client", "1.0.0", WithAuth(func(_ context.Context, _ *authclient.Config) error {
		return authErr
	}))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = c.connectWithAuth(ctx)
	if !errors.Is(err, authErr) {
		t.Errorf("expected authErr, got %v", err)
	}
}
