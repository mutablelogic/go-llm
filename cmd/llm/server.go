//go:build !client

package main

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	agents "github.com/mutablelogic/go-llm/agents"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	broadcastercmd "github.com/mutablelogic/go-llm/broadcaster/cmd"
	frontendcmd "github.com/mutablelogic/go-llm/frontend/cmd"
	heartbeat "github.com/mutablelogic/go-llm/heartbeat"
	heartbeatcmd "github.com/mutablelogic/go-llm/heartbeat/cmd"
	heartbeatschema "github.com/mutablelogic/go-llm/heartbeat/schema"
	opusenc "github.com/mutablelogic/go-llm/internal/opusenc"
	mcpregistry "github.com/mutablelogic/go-llm/mcp/registry"
	mcpcmd "github.com/mutablelogic/go-llm/mcp/registry/cmd"
	memory "github.com/mutablelogic/go-llm/memory"
	memorycmd "github.com/mutablelogic/go-llm/memory/cmd"
	messaging "github.com/mutablelogic/go-llm/messaging"
	registry "github.com/mutablelogic/go-llm/registry"
	registrycmd "github.com/mutablelogic/go-llm/registry/cmd"
	registryschema "github.com/mutablelogic/go-llm/registry/schema"
	resource "github.com/mutablelogic/go-llm/resource"
	resourcecmd "github.com/mutablelogic/go-llm/resource/cmd"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolkitcmd "github.com/mutablelogic/go-llm/toolkit/cmd"
	llmtools "github.com/mutablelogic/go-llm/toolkit/llm"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	types "github.com/mutablelogic/go-server/pkg/types"
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
	Resource    resourcecmd.ClientCommands    `embed:""`
	Connector   mcpcmd.ClientCommands         `embed:""`
}

type ServerCommands struct {
	RunServer RunServer `cmd:"" name:"run" help:"Run the server." group:"SERVER"`
	servercmd.OpenAPICommands
}

type RunServer struct {
	Registry    registrycmd.Flags        `embed:"" prefix:"llm."`
	Session     registrycmd.SessionFlags `embed:"" prefix:"session."`
	Toolkit     toolkitcmd.Flags         `embed:"" prefix:"llm."`
	Broadcaster broadcastercmd.Flags     `embed:"" prefix:"llm."`
	Memory      memorycmd.Flags          `embed:"" prefix:"memory."`
	Heartbeat   heartbeatcmd.Flags       `embed:"" prefix:"heartbeat."`
	Resource    resourcecmd.Flags        `embed:"" prefix:"resource."`
	Connector   mcpcmd.Flags             `embed:"" prefix:"mcp."`
	Frontend    frontendcmd.Flags        `embed:""`
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
	outcomes, err := newServerEventStore(ctx)
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
	resources, err := newServerResourceStore(ctx)
	if err != nil {
		return err
	}
	connectorStore, err := newServerConnectorStore(ctx)
	if err != nil {
		return err
	}

	// A session exists, when the session store has it
	sessionExists := func(ctx context.Context, id uuid.UUID) error {
		_, err := sessions.Get(ctx, id)
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
			// A heartbeat which fires is run by the registry, which is made
			// once the heartbeats are, as it has their tools, in a session of
			// its own
			var events atomic.Pointer[registry.Registry]
			build := eventBuilder(mem.Details)
			run := func(runCtx context.Context, fire heartbeatschema.FireEvent) error {
				r := events.Load()
				if r == nil {
					return llm.ErrServiceUnavailable.With("registry isn't running")
				}
				// How it's run, as an event which the API is given is, in
				// the heartbeat's session
				req, err := build(runCtx, registryschema.EventRequest{
					Session:   fire.Session,
					Title:     fire.Title,
					Prompt:    fire.Prompt,
					Scheduled: fire.Scheduled,
					Fired:     fire.Fired,
				})
				if err != nil {
					return err
				}
				response, err := r.RunEvent(runCtx, fire.Heartbeat, req, nil)
				if err != nil {
					return err
				}
				ctx.Logger().InfoContext(runCtx, "heartbeat run", "heartbeat", fire.Heartbeat, "title", fire.Title, "session", response.Session.ID, "model", types.Value(response.Session.Model), "reply", string(response.Run.Reply))
				return nil
			}
			// The resources, which are in sessions which exist, or on their
			// own, and the heartbeats, which are for sessions which exist
			return runner.Resource.WithResources(ctx, &runner.RunServer, resources, func(res *resource.Manager) error {
				return runner.Heartbeat.WithHeartbeats(ctx, &runner.RunServer, heartbeats, func(h *heartbeat.Heartbeats) error {
					return runner.withToolkit(ctx, b, mem, h, res, &events, build, store, connectorStore, sessions, agents, outcomes)
				}, heartbeat.WithBroadcaster(b), heartbeat.WithZone(zone), heartbeat.WithRun(run), heartbeat.WithSessions(sessionExists))
			}, resource.WithBroadcaster(b), resource.WithSessions(sessionExists))
		}, memory.WithBroadcaster(b))
	})
}

// withToolkit runs the server, with a toolkit, which has the memory, and
// the heartbeats, as well as the tools whose credentials are set, and a
// registry, with the stores, which runs the heartbeats which fire, once
// it's set to, and fires them
func (runner *RunServer) withToolkit(ctx server.Cmd, b *broadcaster.Broadcaster, mem *memory.Memory, h *heartbeat.Heartbeats, res *resource.Manager, events *atomic.Pointer[registry.Registry], build registry.EventBuilder, store registry.ProviderStore, connectorStore *connectorStore, sessions *sessionStore, agents *agentStore, outcomes *eventStore) error {
	return runner.Toolkit.WithToolkit(ctx, &runner.RunServer, func(tools *toolkit.Toolkit) error {
		// The memory, whose system prompt has what's remembered, and whose
		// tools remember it, and which, when a model can call them, has it
		// told when each of the person's messages was sent
		if err := tools.Add(mem); err != nil {
			return err
		}
		ctx.Logger().InfoContext(ctx.Context(), "tools added", "namespace", mem.Name(), "tools", len(mem.Tools()))

		// The heartbeats, whose tool schedules them
		if err := tools.Add(h); err != nil {
			return err
		}
		ctx.Logger().InfoContext(ctx.Context(), "tools added", "namespace", h.Name(), "tools", len(h.Tools()))

		// The llm namespace, whose tools ask the registry's models, once it's
		// made, to speak text, read the text in images and documents, and
		// transcribe speech, with the model the model which calls them gives,
		// which the person chooses, and the model remembers, and whose
		// resource tool lists a chat's files, and reads them, as Markdown
		llmOpts := []llmtools.Opt{llmtools.WithResources(res)}
		if converter, err := opusenc.New(""); err == nil {
			// Speech is converted to a voice note, when it's asked for as
			// one, and its model can't speak one
			llmOpts = append(llmOpts, llmtools.WithVoiceNoteConverter(converter))
			ctx.Logger().InfoContext(ctx.Context(), "voice notes are converted", "opusenc", converter.Path())
		} else {
			ctx.Logger().InfoContext(ctx.Context(), "voice notes are only spoken by models which can, without opusenc, of opus-tools")
		}
		ns, err := llmtools.New(func() llmtools.Models {
			if r := events.Load(); r != nil {
				return r
			}
			return nil
		}, llmOpts...)
		if err != nil {
			return err
		}
		if err := tools.Add(ns); err != nil {
			return err
		}
		ctx.Logger().InfoContext(ctx.Context(), "tools added", "namespace", ns.Name(), "tools", len(ns.Tools()))

		// Messaging, when a provider's credentials are set, such as
		// Telegram's, whose chats are connected to sessions, as channels,
		// and whose tools connect them
		m, err := newServerMessaging(ctx, tools, events, res, func() *time.Location { return mem.Details().Zone() }, messaging.WithBroadcaster(b), messaging.WithFiles(openFile(res)), messaging.WithSessions(func(ctx context.Context, id uuid.UUID) error {
			_, err := sessions.Get(ctx, id)
			return err
		}))
		if err != nil {
			return err
		} else if m != nil {
			if err := tools.Add(m); err != nil {
				return err
			}
			ctx.Logger().InfoContext(ctx.Context(), "tools added", "namespace", m.Name(), "tools", len(m.Tools()))
		}

		// Each reply in a session, with the files it made, is sent to its
		// channels, in the order they're replied, as the chat waits for it to
		// be sent, for up to two minutes, as files can take a while
		replies := func(replyCtx context.Context, session uuid.UUID, reply registry.Reply) {
			if m == nil {
				return
			}
			replyCtx, cancel := context.WithTimeout(replyCtx, 2*time.Minute)
			defer cancel()
			out := messaging.Outgoing{Text: reply.Text, Files: replyFiles(reply)}
			if err := m.Reply(replyCtx, session, out); err != nil {
				ctx.Logger().ErrorContext(replyCtx, "reply not sent to channels", "session", session, "error", err.Error())
			}
		}

		// The connectors, which are remote MCP servers, once the toolkit,
		// and the resources, are made, as their servers' tools are added to
		// the toolkit, each in a namespace of its own, as it's connected,
		// and their resources will be added to the resources
		return runner.Connector.WithConnectorRegistry(ctx, &runner.RunServer, b, func(connectors *mcpregistry.Registry) error {
			return runner.withRegistry(ctx, b, mem, h, res, m, connectors, tools, events, build, store, sessions, agents, outcomes, replies)
		}, mcpregistry.WithStore(connectorStore), mcpregistry.WithCredentials(connectorStore), mcpregistry.WithToolkit(tools))
	}, toolkit.WithBroadcaster(b))
}

// withRegistry runs the server, with the registry, which has the stores,
// and the toolkit, and runs the heartbeats which fire, and the messaging's
// chats, the resources' cleaning, the connectors, and the broadcaster,
// until any stops
func (runner *RunServer) withRegistry(ctx server.Cmd, b *broadcaster.Broadcaster, mem *memory.Memory, h *heartbeat.Heartbeats, res *resource.Manager, m *messaging.Messaging, connectors *mcpregistry.Registry, tools *toolkit.Toolkit, events *atomic.Pointer[registry.Registry], build registry.EventBuilder, store registry.ProviderStore, sessions *sessionStore, agents *agentStore, outcomes *eventStore, replies registry.ReplyFunc) error {
	return runner.Registry.WithRegistry(ctx, &runner.RunServer, func(r *registry.Registry) error {
		// The example agents, as they are, before the server starts
		createAgents(ctx, r)

		// The registry runs the heartbeats which fire
		events.Store(r)

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
			// Heartbeats fire once the registry has its providers, and
			// their models, and agents, so one which is overdue, which
			// fires as they start, can be run
			select {
			case <-r.Ready():
			case <-runCtx.Done():
				return nil
			}
			return h.Run(runCtx, ctx.Logger())
		})
		if m != nil {
			group.Go(func() error {
				// Messages are received once the registry has its
				// providers, and models, which reply to them
				select {
				case <-r.Ready():
				case <-runCtx.Done():
					return nil
				}
				return m.Run(runCtx, ctx.Logger())
			})
		}
		group.Go(func() error {
			// The resources of sessions which are deleted are deleted,
			// and those which aren't needed are cleaned
			return res.Run(runCtx, ctx.Logger())
		})
		group.Go(func() error {
			// The connectors' servers are connected, as they're created,
			// and disconnected, as they're deleted
			return connectors.Run(runCtx, ctx.Logger())
		})
		group.Go(func() error {
			return runner.RunServer.Run(ctx.WithContext(runCtx))
		})
		return group.Wait()
	}, registry.WithBroadcaster(b), registry.WithProviderStore(store), registry.WithSessionStore(sessions), registry.WithAgentStore(agents), registry.WithEventStore(outcomes), registry.WithEventBuilder(build), registry.WithToolkit(tools), registry.WithResources(res), registry.WithTimes(func() *time.Location { return mem.Details().Zone() }, memory.Name), registry.WithReplies(replies), registry.WithCompactTools(memory.Name), runner.Session.Opt(hasHeartbeats(h)))
}

// hasHeartbeats returns the function which says whether a session is kept,
// however long since it changed, as it has a heartbeat: one which fires
// again, such as a reminder weeks away, or which fired, and may be waiting
// to run, or running, in it, as a heartbeat is kept for a while after it
// last fired, so its session is there for it
func hasHeartbeats(h *heartbeat.Heartbeats) func(context.Context, uuid.UUID) (bool, error) {
	return func(ctx context.Context, id uuid.UUID) (bool, error) {
		list, err := h.List(ctx, heartbeatschema.HeartbeatListRequest{Session: id.String(), Limit: types.Ptr(uint64(1))})
		if err != nil {
			return false, err
		}
		return list.Count > 0, nil
	}
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
		ctx.Logger().ErrorContext(ctx.Context(), "agents not added", "error", err.Error())
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

// newServerEventStore returns the store of the outcomes of events, in files
// in the directory of the command's defaults, beside the sessions
func newServerEventStore(ctx server.Cmd) (*eventStore, error) {
	dir, err := serverDir(ctx)
	if err != nil {
		return nil, err
	}
	return newEventStore(dir)
}

// newServerResourceStore returns the store of the resources, in a directory
// of their own, resource, in the directory of the command's defaults
func newServerResourceStore(ctx server.Cmd) (*resourceStore, error) {
	dir, err := serverDir(ctx)
	if err != nil {
		return nil, err
	}
	return newResourceStore(filepath.Join(dir, "resource"))
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
