// Package test has a mock MCP server, for tests of the MCP clients, and
// connectors
package test

import (
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
}

// Opt is an option of a mock server
type Opt func(*opt)

type opt struct {
	sse bool
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

	// Record the headers of each request, and serve it
	self.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		self.mu.Lock()
		self.headers = append(self.headers, r.Header.Clone())
		self.mu.Unlock()
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(self.Close)
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

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Headers returns the headers of the requests the server has been sent, in
// the order they were sent
func (s *Server) Headers() []http.Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]http.Header(nil), s.headers...)
}
