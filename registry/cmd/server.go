//go:build !client

// Package cmd has the registry's commands and flags
package cmd

import (
	// Packages
	registry "github.com/mutablelogic/go-llm/registry"
	httphandler "github.com/mutablelogic/go-llm/registry/httphandler"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Flags are the registry's flags for the server, which has none yet:
// providers are added with the registry's API, or in code
type Flags struct{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithRegistry creates a registry for the server, with the server's tracer,
// and then the options, such as its credentials provider, registers its
// HTTP handlers with the server, when there is one, and then calls the
// function with the registry, returning any error
func (flags Flags) WithRegistry(ctx server.Cmd, srv *servercmd.RunServer, fn func(*registry.Registry) error, opts ...registry.Opt) error {
	opts = append([]registry.Opt{registry.WithTracer(ctx.Tracer())}, opts...)
	r, err := registry.New(opts...)
	if err != nil {
		return err
	}

	// Register the HTTP handlers
	if srv != nil {
		srv.Register(func(router *httprouter.Router) error {
			ctx.Logger().DebugContext(ctx.Context(), "registering llm registry http handlers")
			return httphandler.RegisterHandlers(r, router)
		})
	}

	// Call the function with the registry
	return fn(r)
}
