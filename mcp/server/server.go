package server

import (
	"context"
	"fmt"
	"sync"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Server wraps the official MCP SDK server and exposes methods to run it on
// various transports (stdio, HTTP streamable, SSE).
type Server struct {
	sdkmcp.Implementation
	server *sdkmcp.Server
	mu     sync.Mutex
	uris   map[string]struct{} // URIs currently registered
	tracer trace.Tracer
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

	// Enable resource subscription support so that ResourceUpdated
	// notifications reach subscribed clients. The handlers are no-ops;
	// the SDK itself tracks which sessions are subscribed per URI.
	o.sdkOpts.SubscribeHandler = func(_ context.Context, _ *sdkmcp.SubscribeRequest) error { return nil }
	o.sdkOpts.UnsubscribeHandler = func(_ context.Context, _ *sdkmcp.UnsubscribeRequest) error { return nil }

	s := &Server{
		Implementation: o.impl,
		uris:           make(map[string]struct{}),
		tracer:         o.tracer,
	}
	s.server = sdkmcp.NewServer(&s.Implementation, &o.sdkOpts)
	return s, nil
}
