//go:build !client

package main

import (
	"os"
	"path/filepath"

	// Packages
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	broadcastercmd "github.com/mutablelogic/go-llm/broadcaster/cmd"
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
	Server      ServerCommands                `embed:""`
	Registry    registrycmd.ClientCommands    `embed:""`
	Toolkit     toolkitcmd.ClientCommands     `embed:""`
	Broadcaster broadcastercmd.ClientCommands `embed:""`
}

type ServerCommands struct {
	RunServer RunServer `cmd:"" name:"run" help:"Run the server." group:"SERVER"`
	servercmd.OpenAPICommands
}

type RunServer struct {
	Registry    registrycmd.Flags    `embed:"" prefix:"llm."`
	Toolkit     toolkitcmd.Flags     `embed:"" prefix:"llm."`
	Broadcaster broadcastercmd.Flags `embed:"" prefix:"llm."`
	servercmd.RunServer
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run runs the server, with a registry of providers, which are added with
// its API, and sessions, which are kept in the command's defaults, so
// they're kept when it restarts, and a toolkit of the tools whose credentials are set, which the
// registry's models can call, and their APIs, under the server's prefix, and
// keeps the providers' state and models current, and streams the changes to
// them, and to the sessions, as events, until it's stopped
func (runner *RunServer) Run(ctx server.Cmd) error {
	ctx.Logger().InfoContext(ctx.Context(), "starting server", "name", ctx.Name(), "version", ctx.Version())
	store, err := newServerProviderStore(ctx)
	if err != nil {
		return err
	}
	sessions, err := newServerSessionStore(ctx)
	if err != nil {
		return err
	}
	agents, err := newServerAgentStore(ctx)
	if err != nil {
		return err
	}
	return runner.Broadcaster.WithBroadcaster(ctx, &runner.RunServer, func(b *broadcaster.Broadcaster) error {
		return runner.Toolkit.WithToolkit(ctx, &runner.RunServer, func(tools *toolkit.Toolkit) error {
			return runner.Registry.WithRegistry(ctx, &runner.RunServer, func(r *registry.Registry) error {
				// They all stop when any does, such as when the server is
				// stopped, when the broadcaster's watches end, so the
				// server isn't kept waiting for its streams of events
				group, runCtx := errgroup.WithContext(ctx.Context())
				group.Go(func() error {
					return b.Run(runCtx)
				})
				group.Go(func() error {
					return r.Run(runCtx, ctx.Logger())
				})
				group.Go(func() error {
					return runner.RunServer.Run(ctx.WithContext(runCtx))
				})
				return group.Wait()
			}, registry.WithBroadcaster(b), registry.WithProviderStore(store), registry.WithSessionStore(sessions), registry.WithAgentStore(agents), registry.WithToolkit(tools))
		})
	})
}

// newServerProviderStore returns the store of the providers, in a file of
// its own, providers.json, in the directory of the command's defaults,
// rather than in the defaults, which a client writes too, as a whole, so
// would replace the providers which the server added since the client read
// them. The providers which the defaults have are moved to it, unless it
// exists.
func newServerProviderStore(ctx server.Cmd) (registry.ProviderStore, error) {
	dir, err := serverDir(ctx)
	if err != nil {
		return nil, err
	}
	file, err := newJSONFile(filepath.Join(dir, providersFile))
	if err != nil {
		return nil, err
	} else if err := migrateProviders(file, ctx); err != nil {
		return nil, err
	}
	return newProviderStore(file)
}

// newServerSessionStore returns the store of the sessions, in files in the
// directory of the command's defaults, rather than in the defaults, which a
// client writes too, with the sessions which the defaults have moved to it
func newServerSessionStore(ctx server.Cmd) (*sessionStore, error) {
	dir, err := serverDir(ctx)
	if err != nil {
		return nil, err
	}
	store, err := newSessionStore(dir)
	if err != nil {
		return nil, err
	} else if err := store.migrate(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

// newServerAgentStore returns the store of the agents, in files in the
// directory of the command's defaults, beside the sessions
func newServerAgentStore(ctx server.Cmd) (*agentStore, error) {
	dir, err := serverDir(ctx)
	if err != nil {
		return nil, err
	}
	return newAgentStore(dir)
}

// serverDir returns the directory of the command's defaults, which the
// server keeps its providers and sessions in
func serverDir(ctx server.Cmd) (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, ctx.Name()), nil
}
