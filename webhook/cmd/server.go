//go:build !client

// Package cmd has the webhooks' flags, and their registry, for the server
package cmd

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
	httphandler "github.com/mutablelogic/go-llm/webhook/httphandler"
	registry "github.com/mutablelogic/go-llm/webhook/registry"
	store "github.com/mutablelogic/go-llm/webhook/store"
	server "github.com/mutablelogic/go-server"
	broadcaster "github.com/mutablelogic/go-server/broadcaster"
	config "github.com/mutablelogic/go-server/config"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Flags are the webhooks' flags for the server, which has none yet
type Flags struct{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithWebhook creates the registry of webhooks, of the store, whose changes
// it publishes to the broadcaster, with the options, after the server's,
// such as its tracer, registers its HTTP handlers with the server, when
// there is one, whose webhooks' URLs are of the server's public endpoint,
// and then calls the function with it, returning any error. The function
// runs it, so its changes are logged.
func (flags Flags) WithWebhook(ctx server.Cmd, srv *servercmd.RunServer, store store.Store, b *broadcaster.Broadcaster, fn func(*registry.Registry) error, opts ...registry.Opt) error {
	if store == nil {
		return llm.ErrBadParameter.With("missing webhook store")
	} else if b == nil {
		return llm.ErrBadParameter.With("missing broadcaster")
	}

	// The server's options come first, so the options are used instead
	opts = append([]registry.Opt{registry.WithTracer(ctx.Tracer()), registry.WithBroadcaster(b)}, opts...)
	r, err := registry.New(store, opts...)
	if err != nil {
		return err
	}

	// Register the HTTP handlers, whose webhooks' URLs are of the server's
	// public endpoint, as it is when each is created
	if srv != nil {
		srv.Register(func(router *httprouter.Router) error {
			ctx.Logger().DebugContext(ctx.Context(), "registering webhook http handlers")
			return httphandler.RegisterHandlers(r, router, func() string { return config.PublicEndpoint(ctx.URL()) })
		})
	}

	// Call the function with the registry
	return fn(r)
}
