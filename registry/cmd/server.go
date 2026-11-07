//go:build !client

// Package cmd has the registry's commands and flags
package cmd

import (
	"context"
	"time"
	"uuid"
	// Packages
	client "github.com/mutablelogic/go-client"
	registry "github.com/mutablelogic/go-llm/registry"
	httphandler "github.com/mutablelogic/go-llm/registry/httphandler"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Flags are the registry's flags for the server: the most events which run
// at once. Providers are added with the registry's API, or in code, and
// kept in a store, when there's one.
type Flags struct {
	MaxEvents uint `name:"max-events" help:"The most events, such as heartbeats which fire, and events which are triggered, which run at once." default:"2" env:"LLM_MAX_EVENTS"`
}

// SessionFlags are the sessions' flags for the server: how long a session
// is kept after it last changed, such as when the person last chatted in
// it, which is in days, such as 14d, or hours, or zero, to keep sessions
// until they're deleted
type SessionFlags struct {
	Retention time.Duration `name:"retention" help:"How long a session is kept after it last changed, such as when it was last chatted in, unless it has a heartbeat, which fires again, or fired lately, such as 14d, or 0 to keep sessions until they're deleted." default:"14d" env:"LLM_SESSION_RETENTION"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Opt returns the option which deletes the sessions which haven't changed
// for longer than they're kept, unless the function keeps one, such as one
// with a heartbeat
func (flags SessionFlags) Opt(keep func(context.Context, uuid.UUID) (bool, error)) registry.Opt {
	return registry.WithSessionRetention(flags.Retention, keep)
}

// WithRegistry creates a registry for the server, with the server's tracer,
// and the server's client options, such as its tracer and timeout, with
// its name and version as the user agent, for the providers' clients, and
// then the options, such as its store of providers, such as its credentials provider, registers its HTTP
// handlers with the server, when there is one, and then calls the function
// with the registry, returning any error
func (flags Flags) WithRegistry(ctx server.Cmd, srv *servercmd.RunServer, fn func(*registry.Registry) error, opts ...registry.Opt) error {
	_, clientOpts, err := ctx.ClientEndpoint()
	if err != nil {
		return err
	}
	agent := ctx.Name()
	if version := ctx.Version(); version != "" {
		agent += "/" + version
	}
	clientOpts = append(clientOpts, client.OptUserAgent(agent))

	// The server's options come first, so the options are used instead
	opts = append([]registry.Opt{
		registry.WithTracer(ctx.Tracer()),
		registry.WithClientOpts(clientOpts...),
	}, opts...)
	if flags.MaxEvents > 0 {
		opts = append(opts, registry.WithMaxEvents(flags.MaxEvents))
	}
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
