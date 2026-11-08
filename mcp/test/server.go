// Package test has a mock MCP server, for tests of the MCP clients, and
// connectors
package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Server is a mock MCP server, over HTTP, which records the headers of the
// requests it's sent
type Server struct {
	*httptest.Server

	// The MCP server, which tools, prompts and resources can be added to
	MCP *sdkmcp.Server

	mu      sync.Mutex
	headers []http.Header

	// Its authorization server, when it needs to be authorized
	auth *auth
}

// Opt is an option of a mock server
type Opt func(*opt)

type opt struct {
	sse  bool
	ping json.RawMessage
	auth bool
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The name, and version, which the mock server says it is, as it's
// connected to
const (
	Name    = "test-server"
	Version = "1.0"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewServer starts a mock MCP server, with the streamable HTTP transport,
// or only SSE, with WithSSE, which is closed when the test ends
func NewServer(t *testing.T, opts ...Opt) *Server {
	t.Helper()
	var o opt
	for _, fn := range opts {
		fn(&o)
	}

	// The MCP server, and its transport's handler
	self := &Server{MCP: sdkmcp.NewServer(&sdkmcp.Implementation{Name: Name, Version: Version}, nil)}
	getServer := func(*http.Request) *sdkmcp.Server { return self.MCP }
	var handler http.Handler
	if o.sse {
		handler = sdkmcp.NewSSEHandler(getServer, nil)
	} else {
		handler = sdkmcp.NewStreamableHTTPHandler(getServer, nil)
	}

	// Record the headers of each request, and serve it, or reply to a ping
	// with the result, when it's given
	if o.auth {
		self.auth = newAuth()
	}
	self.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		self.mu.Lock()
		self.headers = append(self.headers, r.Header.Clone())
		self.mu.Unlock()
		if self.auth != nil && !self.auth.serve(self.URL, w, r) {
			// Its authorization server's, or not authorized
			return
		}
		if o.ping != nil && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(body))
			var request struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
			}
			if json.Unmarshal(body, &request) == nil && request.Method == "ping" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": o.ping})
				return
			}
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(func() {
		// Its clients' connections are closed first, as a connected client
		// has a stream which lasts, which Close would wait for
		self.CloseClientConnections()
		self.Close()
	})
	return self
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithSSE serves only the SSE transport, of the 2024-11-05 protocol, rather
// than the streamable HTTP transport
func WithSSE() Opt {
	return func(o *opt) {
		o.sse = true
	}
}

// WithPingResult replies to a ping, with the streamable HTTP transport,
// with the result, such as [], which some servers reply with, rather than
// {}
func WithPingResult(result string) Opt {
	return func(o *opt) {
		o.ping = json.RawMessage(result)
	}
}

// WithAuth makes the server need to be authorized, with a bearer token,
// which its own authorization server issues: its metadata, and the
// resource's, are at their well-known paths, a client is registered, and a
// code, which Approve gives, as a person would, is exchanged for a token,
// with PKCE's S256, which is refreshed
func WithAuth() Opt {
	return func(o *opt) {
		o.auth = true
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Headers returns the headers of the requests the server has been sent, in
// the order they were sent
func (s *Server) Headers() []http.Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]http.Header(nil), s.headers...)
}
