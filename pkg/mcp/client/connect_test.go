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
	server "github.com/mutablelogic/go-llm/pkg/mcp/server"
)

// Test_connect_001: resolveURL resolves a relative path against the base.
func Test_connect_001(t *testing.T) {
	got := resolveURL("https://example.com/mcp", "/.well-known/oauth")
	want := "https://example.com/.well-known/oauth"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Test_connect_002: resolveURL uses an absolute URL as-is.
func Test_connect_002(t *testing.T) {
	got := resolveURL("https://example.com/mcp", "https://auth.example.com/meta")
	want := "https://auth.example.com/meta"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Test_connect_003: resolveURL falls back to base when ref resolves to a non-http scheme.
func Test_connect_003(t *testing.T) {
	base := "https://example.com/mcp"
	got := resolveURL(base, "ftp://other.com/path")
	if got != base {
		t.Errorf("expected fallback to base %q, got %q", base, got)
	}
}

// Test_connect_004: resolveURL falls back to base when base is malformed.
func Test_connect_004(t *testing.T) {
	base := "://bad-url"
	got := resolveURL(base, "/path")
	if got != base {
		t.Errorf("expected fallback to base %q, got %q", base, got)
	}
}

// Test_connect_005: connectWithAuth returns UnauthorizedError when the server
// returns 401 and no authFn is configured.
func Test_connect_005(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="test"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	c, err := New(ts.URL, "test-client", "1.0.0", nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = c.connectWithAuth(ctx)
	if !IsUnauthorized(err) {
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
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authDone.Load() {
			mcpHandler.ServeHTTP(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate",
			fmt.Sprintf(`Bearer realm="test", resource_metadata="%s/meta"`, ts.URL))
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	var gotDiscoveryURL string
	c, err := New(ts.URL, "test-client", "1.0.0", func(_ context.Context, discoveryURL string) error {
		gotDiscoveryURL = discoveryURL
		authDone.Store(true) // open the gate before the retry
		return nil
	})
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

	wantURL := ts.URL + "/meta"
	if gotDiscoveryURL != wantURL {
		t.Errorf("authFn got discoveryURL %q, want %q", gotDiscoveryURL, wantURL)
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
	c, err := New(ts.URL, "test-client", "1.0.0", func(_ context.Context, _ string) error {
		return authErr
	})
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
