package server

import (
	"fmt"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Server wraps the official MCP SDK server and exposes methods to run it on
// various transports (stdio, HTTP streamable, SSE).
type Server struct {
	sdkmcp.Implementation
	server *sdkmcp.Server
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new MCP server with the given implementation name and version.
func New(name, version string, optFns ...ServerOpt) (*Server, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	o := opts{
		impl: sdkmcp.Implementation{Name: name, Version: version},
	}
	for _, opt := range optFns {
		if err := opt(&o); err != nil {
			return nil, err
		}
	}

	s := &Server{Implementation: o.impl}
	s.server = sdkmcp.NewServer(&s.Implementation, &o.sdkOpts)
	return s, nil
}
