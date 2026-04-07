package server

import (
	"net/http"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Handler returns an http.Handler that speaks the 2025-03-26 Streamable HTTP
// MCP transport. Mount this on an HTTP server at a path of your choice.
//
// Every request is served by the same underlying Server instance.
func (s *Server) Handler() http.Handler {
	return sdkmcp.NewStreamableHTTPHandler(func(_ *http.Request) *sdkmcp.Server {
		return s.server
	}, nil)
}
