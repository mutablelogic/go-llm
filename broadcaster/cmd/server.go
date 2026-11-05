//go:build !client

package cmd

import (
	// Packages
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	httphandler "github.com/mutablelogic/go-llm/broadcaster/httphandler"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Flags are the broadcaster's flags for the server, which has none yet
type Flags struct{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithBroadcaster creates a broadcaster for the server, registers its HTTP
// handlers with the server, when there is one, and then calls the function
// with the broadcaster, returning any error. The function runs it, so its
// watches end when the server stops.
func (flags Flags) WithBroadcaster(ctx server.Cmd, srv *servercmd.RunServer, fn func(*broadcaster.Broadcaster) error) error {
	b := broadcaster.New()

	// Register the HTTP handlers
	if srv != nil {
		srv.Register(func(router *httprouter.Router) error {
			ctx.Logger().DebugContext(ctx.Context(), "registering llm broadcaster http handlers")
			return httphandler.RegisterHandlers(b, router)
		})
	}

	// Call the function with the broadcaster
	return fn(b)
}
