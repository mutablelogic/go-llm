//go:build !client

package cmd

import (
	// Packages
	memory "github.com/mutablelogic/go-llm/memory"
	httphandler "github.com/mutablelogic/go-llm/memory/httphandler"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Flags are the memory's flags for the server, which has none yet
type Flags struct{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithMemory creates the memory, with the details in the store, and the
// options, such as the broadcaster it publishes its changes to, registers
// its HTTP handlers with the server, when there is one, and then calls the
// function with the memory, returning any error. The function adds it to a
// toolkit, so models have its system prompt, and tools.
func (flags Flags) WithMemory(ctx server.Cmd, srv *servercmd.RunServer, store memory.Store, fn func(*memory.Memory) error, opts ...memory.Opt) error {
	// The server's options come first, so the options are used instead
	opts = append([]memory.Opt{memory.WithTracer(ctx.Tracer())}, opts...)
	m, err := memory.New(ctx.Context(), store, opts...)
	if err != nil {
		return err
	}

	// Register the HTTP handlers
	if srv != nil {
		srv.Register(func(router *httprouter.Router) error {
			ctx.Logger().DebugContext(ctx.Context(), "registering memory http handlers")
			return httphandler.RegisterHandlers(m, router)
		})
	}

	// Call the function with the memory
	return fn(m)
}
