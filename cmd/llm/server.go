//go:build !client

package main

import (
	"os"
	"path/filepath"
	"time"

	// Packages
	agents "github.com/mutablelogic/go-llm/agents"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	broadcastercmd "github.com/mutablelogic/go-llm/broadcaster/cmd"
	frontendcmd "github.com/mutablelogic/go-llm/frontend/cmd"
	heartbeat "github.com/mutablelogic/go-llm/heartbeat"
	heartbeatcmd "github.com/mutablelogic/go-llm/heartbeat/cmd"
	memory "github.com/mutablelogic/go-llm/memory"
	memorycmd "github.com/mutablelogic/go-llm/memory/cmd"
	registry "github.com/mutablelogic/go-llm/registry"
	registrycmd "github.com/mutablelogic/go-llm/registry/cmd"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolkitcmd "github.com/mutablelogic/go-llm/toolkit/cmd"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	errgroup "golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CLI struct {
	Server      ServerCommands                `embed:""`
	Registry    registrycmd.ClientCommands    `embed:""`
	Toolkit     toolkitcmd.ClientCommands     `embed:""`
	Broadcaster broadcastercmd.ClientCommands `embed:""`
	Memory      memorycmd.ClientCommands      `embed:""`
	Heartbeat   heartbeatcmd.ClientCommands   `embed:""`
}

type ServerCommands struct {
	RunServer RunServer `cmd:"" name:"run" help:"Run the server." group:"SERVER"`
	servercmd.OpenAPICommands
}

type RunServer struct {
	Registry    registrycmd.Flags    `embed:"" prefix:"llm."`
	Toolkit     toolkitcmd.Flags     `embed:"" prefix:"llm."`
	Broadcaster broadcastercmd.Flags `embed:"" prefix:"llm."`
	Memory      memorycmd.Flags      `embed:"" prefix:"memory."`
	Heartbeat   heartbeatcmd.Flags   `embed:"" prefix:"heartbeat."`
	Frontend    frontendcmd.Flags    `embed:""`
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
	memories, err := newServerMemoryStore(ctx)
	if err != nil {
		return err
	}
	heartbeats, err := newServerHeartbeatStore(ctx)
	if err != nil {
		return err
	}

	// Serve the web UI, when it's built in, and its configuration, alongside
	// the API, grouping its operations with the server's OpenAPI
	// documentation
	runner.RunServer.Register(func(router *httprouter.Router) error {
		router.Spec().AddTagGroup("UI", "OpenAPI", "UI")
		return runner.Frontend.RegisterUI(ctx.Context(), ctx.Logger(), router, runner.RunServer.OpenAPI)
	})
	return runner.Broadcaster.WithBroadcaster(ctx, &runner.RunServer, func(b *broadcaster.Broadcaster) error {
		return runner.Memory.WithMemory(ctx, &runner.RunServer, memories, func(mem *memory.Memory) error {
			// The heartbeats' schedules follow the person's time zone, as the
			// memory has it
			zone := func() *time.Location {
				return mem.Details().Zone()
			}
			return runner.Heartbeat.WithHeartbeats(ctx, &runner.RunServer, heartbeats, func(h *heartbeat.Heartbeats) error {
				return runner.withToolkit(ctx, b, mem, h, store, sessions, agents)
			}, heartbeat.WithBroadcaster(b), heartbeat.WithZone(zone))
		}, memory.WithBroadcaster(b))
	})
}

// withToolkit runs the server, with a toolkit, which has the memory, and
// the heartbeats, as well as the tools whose credentials are set, and a
// registry, with the stores, and fires the heartbeats
func (runner *RunServer) withToolkit(ctx server.Cmd, b *broadcaster.Broadcaster, mem *memory.Memory, h *heartbeat.Heartbeats, store registry.ProviderStore, sessions *sessionStore, agents *agentStore) error {
	return runner.Toolkit.WithToolkit(ctx, &runner.RunServer, func(tools *toolkit.Toolkit) error {
		// The memory, whose system prompt has the time, and what's
		// remembered, and whose tools remember it
		if err := tools.Add(mem); err != nil {
			return err
		}
		ctx.Logger().InfoContext(ctx.Context(), "tools added", "namespace", mem.Name(), "tools", len(mem.Tools()))

		// The heartbeats, whose tool schedules them
		if err := tools.Add(h); err != nil {
			return err
		}
		ctx.Logger().InfoContext(ctx.Context(), "tools added", "namespace", h.Name(), "tools", len(h.Tools()))

		return runner.Registry.WithRegistry(ctx, &runner.RunServer, func(r *registry.Registry) error {
			// The example agents, as they are, before the server starts
			createAgents(ctx, r)

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
				return h.Run(runCtx, ctx.Logger())
			})
			group.Go(func() error {
				return runner.RunServer.Run(ctx.WithContext(runCtx))
			})
			return group.Wait()
		}, registry.WithBroadcaster(b), registry.WithProviderStore(store), registry.WithSessionStore(sessions), registry.WithAgentStore(agents), registry.WithToolkit(tools))
	}, toolkit.WithBroadcaster(b))
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

// createAgents creates the example agents, or replaces those which have
// changed, so they're as they are in this version. An agent which can't be
// created is logged, rather than stopping the server.
func createAgents(ctx server.Cmd, r *registry.Registry) {
	all, err := agents.All()
	if err != nil {
		ctx.Logger().ErrorContext(ctx.Context(), "agents not added", "error", err)
		return
	}
	for _, meta := range all {
		if _, _, err := r.CreateAgent(ctx.Context(), meta); err != nil {
			ctx.Logger().WarnContext(ctx.Context(), "agent not added", "agent", meta.Name, "error", err.Error())
		}
	}
	ctx.Logger().InfoContext(ctx.Context(), "agents added", "agents", len(all))
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

// newServerMemoryStore returns the store of the memory, whose details are a
// file of their own, memory.json, and whose memories are files of Markdown,
// in the directory of the command's defaults
func newServerMemoryStore(ctx server.Cmd) (*memoryStore, error) {
	dir, err := serverDir(ctx)
	if err != nil {
		return nil, err
	}
	file, err := newJSONFile(filepath.Join(dir, memoryFile))
	if err != nil {
		return nil, err
	}
	return newMemoryStore(file, dir), nil
}

// newServerHeartbeatStore returns the store of the heartbeats, in files in
// the directory of the command's defaults, beside the sessions
func newServerHeartbeatStore(ctx server.Cmd) (*heartbeatStore, error) {
	dir, err := serverDir(ctx)
	if err != nil {
		return nil, err
	}
	return newHeartbeatStore(dir)
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
