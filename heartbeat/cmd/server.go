//go:build !client

package cmd

import (
	// Packages
	heartbeat "github.com/mutablelogic/go-llm/heartbeat"
	httphandler "github.com/mutablelogic/go-llm/heartbeat/httphandler"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Flags are the heartbeats' flags for the server, which has none yet
type Flags struct{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithHeartbeats creates the heartbeats, in the store, with the options,
// such as the broadcaster they publish their changes to, and the person's
// time zone, registers their HTTP handlers with the server, when there is
// one, and then calls the function with them, returning any error. The
// function adds them to a toolkit, so models can schedule them, and runs
// them, so they fire.
func (flags Flags) WithHeartbeats(ctx server.Cmd, srv *servercmd.RunServer, store heartbeat.Store, fn func(*heartbeat.Heartbeats) error, opts ...heartbeat.Opt) error {
	// The server's options come first, so the options are used instead
	opts = append([]heartbeat.Opt{heartbeat.WithTracer(ctx.Tracer())}, opts...)
	h, err := heartbeat.New(store, opts...)
	if err != nil {
		return err
	}

	// Register the HTTP handlers
	if srv != nil {
		srv.Register(func(router *httprouter.Router) error {
			ctx.Logger().DebugContext(ctx.Context(), "registering heartbeat http handlers")
			return httphandler.RegisterHandlers(h, router)
		})
	}

	// Call the function with the heartbeats
	return fn(h)
}
