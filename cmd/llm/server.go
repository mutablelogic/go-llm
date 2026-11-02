//go:build !client

package main

import (
	// Packages
	registry "github.com/mutablelogic/go-llm/registry"
	registrycmd "github.com/mutablelogic/go-llm/registry/cmd"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CLI struct {
	Server   ServerCommands             `embed:""`
	Registry registrycmd.ClientCommands `embed:""`
}

type ServerCommands struct {
	RunServer RunServer `cmd:"" name:"run" help:"Run the server." group:"SERVER"`
	servercmd.OpenAPICommands
}

type RunServer struct {
	Registry registrycmd.Flags `embed:"" prefix:"llm."`
	servercmd.RunServer
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run runs the server, with a registry of providers, which are added with
// its API, and its API, under the server's prefix, until it's stopped
func (runner *RunServer) Run(ctx server.Cmd) error {
	ctx.Logger().InfoContext(ctx.Context(), "starting server", "name", ctx.Name(), "version", ctx.Version())
	return runner.Registry.WithRegistry(ctx, &runner.RunServer, func(*registry.Registry) error {
		return runner.RunServer.Run(ctx)
	})
}
