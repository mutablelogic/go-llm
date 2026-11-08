//go:build !client

// Package cmd has the commands of the MCP connectors, such as the server's
package cmd

import (
	"errors"

	// Packages
	client "github.com/mutablelogic/go-client"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	httphandler "github.com/mutablelogic/go-llm/mcp/registry/httphandler"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Flags are the connectors' flags for the server, which has none yet
type Flags struct{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithConnectorRegistry creates the registry of the connectors, with the
// command's client options, such as its tracer, and user agent, which the
// clients of the connectors' servers use, the broadcaster it publishes its
// changes to, when there is one, and the options, such as its store,
// registers its HTTP handlers with the server, when there is one, and then
// calls the function with the registry, returning any error, once its
// connectors' servers are disconnected
func (flags Flags) WithConnectorRegistry(ctx server.Cmd, srv *servercmd.RunServer, b *broadcaster.Broadcaster, fn func(*registry.Registry) error, opts ...registry.Opt) error {
	// The clients' options are the command's, with its user agent
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
	serverOpts := []registry.Opt{registry.WithClientOpts(clientOpts...)}
	if b != nil {
		serverOpts = append(serverOpts, registry.WithBroadcaster(b))
	}
	r, err := registry.New(append(serverOpts, opts...)...)
	if err != nil {
		return err
	}

	// Register the HTTP handlers
	if srv != nil {
		srv.Register(func(router *httprouter.Router) error {
			ctx.Logger().DebugContext(ctx.Context(), "registering mcp connector http handlers")
			return httphandler.RegisterHandlers(r, router)
		})
	}

	// Call the function with the registry, and disconnect the connectors'
	// servers, once it returns, such as when the server stops
	return errors.Join(fn(r), r.Close())
}
