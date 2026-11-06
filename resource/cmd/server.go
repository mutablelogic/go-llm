//go:build !client

package cmd

import (
	"time"

	// Packages
	resource "github.com/mutablelogic/go-llm/resource"
	httphandler "github.com/mutablelogic/go-llm/resource/httphandler"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Flags are the resources' flags for the server: the largest content of a
// resource, and how long one on its own is kept, after it's last used
type Flags struct {
	MaxSize   int64         `name:"max-size" help:"Largest content of a resource, in bytes." default:"268435456" env:"LLM_RESOURCE_MAX_SIZE"`
	Retention time.Duration `name:"retention" help:"How long a resource on its own, without a session, is kept, after it was last used." default:"24h" env:"LLM_RESOURCE_RETENTION"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithResources creates the resources' manager, of the store, with the
// flags, and the options, such as the broadcaster it publishes their changes
// to, and which it watches for the sessions which are deleted, registers its
// HTTP handlers with the server, when there is one, and then calls the
// function with it, returning any error. The function runs it, so the
// resources are cleaned.
func (flags Flags) WithResources(ctx server.Cmd, srv *servercmd.RunServer, store resource.Store, fn func(*resource.Manager) error, opts ...resource.Opt) error {
	// The server's options come first, so the options are used instead
	opts = append([]resource.Opt{resource.WithTracer(ctx.Tracer()), resource.WithMaxSize(flags.MaxSize), resource.WithRetention(flags.Retention)}, opts...)
	r, err := resource.New(store, opts...)
	if err != nil {
		return err
	}

	// Register the HTTP handlers
	if srv != nil {
		srv.Register(func(router *httprouter.Router) error {
			ctx.Logger().DebugContext(ctx.Context(), "registering resource http handlers")
			return httphandler.RegisterHandlers(r, router)
		})
	}

	// Call the function with the manager
	return fn(r)
}
