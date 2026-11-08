//go:build !client

package cmd

import (
	"errors"
	"time"

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

// Flags are the connectors' flags for the server: how long a connector's
// server is waited for, as it's connected, and its tools are listed, and
// how long one of its tools has, as it's called
type Flags struct {
	ConnectTimeout time.Duration `name:"connect-timeout" help:"How long a connector's MCP server is waited for, as it's connected, and its tools are listed." default:"30s" env:"LLM_MCP_CONNECT_TIMEOUT"`
	ToolTimeout    time.Duration `name:"tool-timeout" help:"How long a connector's tool has, as it's called, or 0 for the default of every tool." default:"0" env:"LLM_MCP_TOOL_TIMEOUT"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithConnectorRegistry creates the registry of the connectors, with the
// command's client options, such as its tracer, and user agent, which the
// clients of the connectors' servers use, and its name, and version, which
// the servers are told, the broadcaster it publishes its
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

	// The server's options come first, so the options are used instead:
	// its client options, and its name, and version, which servers are
	// told, as they're connected
	serverOpts := []registry.Opt{registry.WithClientOpts(clientOpts...), registry.WithToolTimeout(flags.ToolTimeout)}
	if flags.ConnectTimeout > 0 {
		serverOpts = append(serverOpts, registry.WithConnectTimeout(flags.ConnectTimeout))
	}
	if name := ctx.Name(); name != "" {
		serverOpts = append(serverOpts, registry.WithName(name, ctx.Version()))
	}
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
