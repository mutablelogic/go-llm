//go:build !client

package main

import (
	// Packages
	registry "github.com/mutablelogic/go-llm/registry"
	registrycmd "github.com/mutablelogic/go-llm/registry/cmd"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolkitcmd "github.com/mutablelogic/go-llm/toolkit/cmd"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	errgroup "golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CLI struct {
	Server   ServerCommands             `embed:""`
	Registry registrycmd.ClientCommands `embed:""`
	Toolkit  toolkitcmd.ClientCommands  `embed:""`
}

type ServerCommands struct {
	RunServer RunServer `cmd:"" name:"run" help:"Run the server." group:"SERVER"`
	servercmd.OpenAPICommands
}

type RunServer struct {
	Registry registrycmd.Flags `embed:"" prefix:"llm."`
	Toolkit  toolkitcmd.Flags  `embed:"" prefix:"llm."`
	servercmd.RunServer
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run runs the server, with a registry of providers, which are added with
// its API, and kept in the command's defaults, so they're kept when it
// restarts, and a toolkit of the tools whose credentials are set, and their
// APIs, under the server's prefix, and keeps the providers' state and models
// current, until it's stopped
func (runner *RunServer) Run(ctx server.Cmd) error {
	ctx.Logger().InfoContext(ctx.Context(), "starting server", "name", ctx.Name(), "version", ctx.Version())
	store, err := newStore(ctx)
	if err != nil {
		return err
	}
	return runner.Toolkit.WithToolkit(ctx, &runner.RunServer, func(*toolkit.Toolkit) error {
		return runner.Registry.WithRegistry(ctx, &runner.RunServer, func(r *registry.Registry) error {
			// Both stop when either does, such as when the server is stopped
			group, runCtx := errgroup.WithContext(ctx.Context())
			group.Go(func() error {
				return r.Run(runCtx, ctx.Logger())
			})
			group.Go(func() error {
				return runner.RunServer.Run(ctx.WithContext(runCtx))
			})
			return group.Wait()
		}, registry.WithStore(store))
	})
}
