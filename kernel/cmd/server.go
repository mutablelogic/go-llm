package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"

	// Packages
	authmanager "github.com/djthorpe/go-auth/pkg/authmanager"
	authhanders "github.com/djthorpe/go-auth/pkg/httphandler/authmanager"
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	agent "github.com/mutablelogic/go-llm/etc/agent"
	heartbeat "github.com/mutablelogic/go-llm/heartbeat/connector"
	heartbeatmanager "github.com/mutablelogic/go-llm/heartbeat/manager"
	heartbeatschema "github.com/mutablelogic/go-llm/heartbeat/schema"
	homeassistant "github.com/mutablelogic/go-llm/homeassistant/connector"
	llmhandlers "github.com/mutablelogic/go-llm/kernel/httphandler"
	llmmanager "github.com/mutablelogic/go-llm/kernel/manager"
	memory "github.com/mutablelogic/go-llm/memory/manager"
	prompt "github.com/mutablelogic/go-llm/toolkit/prompt"
	pg "github.com/mutablelogic/go-pg"
	server "github.com/mutablelogic/go-server"
	cmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	errgroup "golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type RunServer struct {
	cmd.RunServer

	// Postgres connection flags
	PostgresFlags `embed:"" prefix:"pg."`

	// Schemas for tenancy
	Schema struct {
		Auth      string `name:"auth" help:"PostgreSQL schema for auth data." default:"auth"`
		LLM       string `name:"llm" help:"PostgreSQL schema for LLM data." default:"llm"`
		Memory    string `name:"memory" help:"PostgreSQL schema for memory data." default:"memory"`
		Heartbeat string `name:"heartbeat" help:"PostgreSQL schema for heartbeat data." default:"heartbeat"`
	} `embed:"" prefix:"schema."`

	// Home Assistant connector options
	HomeAssistant struct {
		Endpoint string `help:"Home Assistant endpoint URL." env:"HA_ENDPOINT"`
		APIKey   string `help:"Home Assistant long-lived access token." env:"HA_TOKEN"`
	} `embed:"" prefix:"homeassistant."`

	// Other flags
	Passphrases []string `name:"passphrase" env:"${ENV_NAME}_PASSPHRASES" help:"One or more passphrases used to encrypt credentials. "`
	Auth        bool     `name:"auth" help:"Enable authentication for protected endpoints." default:"true" negatable:""`
	Memory      bool     `name:"memory" help:"Enable memory and related endpoints." default:"true" negatable:""`
	Heartbeat   bool     `name:"heartbeat" help:"Enable heartbeat and related endpoints." default:"true" negatable:""`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (runner *RunServer) Run(ctx server.Cmd) error {
	// Connect to the database, if configured
	conn, err := runner.PostgresFlags.Connect(ctx)
	if err != nil {
		return err
	} else if conn == nil {
		return fmt.Errorf("database connection is required")
	}

	// Create the auth manager, run the server, and return any error
	ctx.Logger().InfoContext(ctx.Context(), "starting server", "name", ctx.Name(), "version", ctx.Version())
	return runner.withAuthManager(ctx, conn, func(authmanager *authmanager.Manager) error {
		opts := []llmmanager.Opt{}

		// Add the memory connector, which is used to store and retrieve facts linked to the
		// user and to sessions
		if runner.Memory {
			memory, err := memory.New(ctx.Context(), conn, memory.WithSchemas(runner.Schema.Memory, runner.Schema.LLM, runner.Schema.Auth), memory.WithTracer(ctx.Tracer()))
			if err != nil {
				return err
			} else {
				opts = append(opts, llmmanager.WithConnector("memory", memory))
			}
		}

		// Add the heartbeat connector, which is used to manage and fire scheduled callbacks
		if runner.Heartbeat {
			heartbeat, err := heartbeat.New(ctx.Context(), conn,
				heartbeatmanager.WithSchema(runner.Schema.Heartbeat, runner.Schema.LLM),
				heartbeatmanager.WithTracer(ctx.Tracer()),
				heartbeatmanager.WithOnFire(func(parent context.Context, heartbeat *heartbeatschema.Heartbeat) {
					ctx.Logger().InfoContext(parent, "heartbeat fired", "id", heartbeat.ID, "message", heartbeat.Message)
				}),
			)
			if err != nil {
				return err
			} else {
				opts = append(opts, llmmanager.WithConnector("heartbeat", heartbeat))
			}
		}

		// Add the home assistant connector, which is used to control a Home Assistant instance
		if runner.HomeAssistant.Endpoint != "" {
			clientopt := []client.ClientOpt{}
			conn, err := homeassistant.New(runner.HomeAssistant.Endpoint, runner.HomeAssistant.APIKey, clientopt...)
			if err != nil {
				return err
			} else {
				opts = append(opts, llmmanager.WithConnector("homeassistant", conn))
			}
		}

		return runner.withLLMManager(ctx, conn, opts, func(llmmanager *llmmanager.Manager) error {
			// Sync providers before starting the server so that any configured providers are available immediately
			if _, _, err := llmmanager.SyncProviders(ctx.Context()); err != nil {
				ctx.Logger().ErrorContext(ctx.Context(), "failed to sync llm providers before startup", "error", err.Error())
			}

			// Register handlers for authmanager and llmmanager
			runner.Register(func(router *httprouter.Router) error {
				ctx.Logger().DebugContext(ctx.Context(), "registering authmanager handlers")
				return authhanders.RegisterManagerHandlers(authmanager, router, runner.Auth)
			}).Register(func(router *httprouter.Router) error {
				ctx.Logger().DebugContext(ctx.Context(), "registering llmmanager handlers")
				return llmhandlers.RegisterHandlers(router, llmmanager, authmanager, runner.Auth)
			})

			// Create an error context - which will cancel any other goroutine on exit
			errorgroup := newErrorGroup(ctx)

			// Run the server
			errorgroup.Go(func() error {
				return runner.RunServer.Run(errorgroup)
			})

			// Run the authmanager background tasks
			errorgroup.Go(func() error {
				return authmanager.Run(errorgroup.Context())
			})

			// Run the llmmanager background tasks
			errorgroup.Go(func() error {
				return llmmanager.Run(errorgroup.Context(), ctx.Logger())
			})

			// Run the server
			return errorgroup.Wait()
		})
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - CANCEL CONTEXT

type errorgroup struct {
	server.Cmd
	*errgroup.Group
	ctx context.Context
}

var _ server.Cmd = (*errorgroup)(nil)

func newErrorGroup(cmd server.Cmd) *errorgroup {
	group, ctx := errgroup.WithContext(cmd.Context())
	return &errorgroup{Cmd: cmd, ctx: ctx, Group: group}
}

func (c *errorgroup) Context() context.Context {
	return c.ctx
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - AUTH MANAGER

func (server *RunServer) withAuthManager(ctx server.Cmd, conn pg.PoolConn, fn func(manager *authmanager.Manager) error) error {
	// Create the auth manager
	authmanager, err := authmanager.New(ctx.Context(), conn, server.authManagerOpts(ctx)...)
	if err != nil {
		return err
	}
	defer authmanager.Close()

	// Return nil on success
	return fn(authmanager)
}

func (server *RunServer) authManagerOpts(ctx server.Cmd) []authmanager.Opt {
	return []authmanager.Opt{
		authmanager.WithSchema(server.Schema.Auth),
		authmanager.WithTracer(ctx.Tracer()),
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - LLM MANAGER

type namedReader struct {
	*bytes.Reader
	name string
}

func (r *namedReader) Name() string {
	return r.name
}

func (server *RunServer) withLLMManager(ctx server.Cmd, conn pg.PoolConn, opts []llmmanager.Opt, fn func(manager *llmmanager.Manager) error) error {
	// Create the LLM manager
	llmmanager, err := llmmanager.New(ctx.Context(), ctx.Name(), ctx.Version(), conn, append(server.llmManagerOpts(ctx), opts...)...)
	if err != nil {
		return err
	}
	defer llmmanager.Close()

	// Return nil on success
	return fn(llmmanager)
}

func (server *RunServer) llmManagerPrompts() ([]llm.Prompt, error) {
	var prompts []llm.Prompt
	err := fs.WalkDir(agent.FS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(agent.FS, path)
		if err != nil {
			return err
		}
		if len(data) == 0 {
			return nil
		}
		// Read the prompt from the embedded filesystem and add it to the list of prompts
		prompt, err := prompt.Read(&namedReader{Reader: bytes.NewReader(data), name: path})
		if err != nil {
			return err
		} else {
			prompts = append(prompts, prompt)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return prompts, nil
}

func (server *RunServer) llmManagerOpts(ctx server.Cmd) []llmmanager.Opt {
	opts := []llmmanager.Opt{}

	// Set passphrases for credential encryption
	for i, passphrase := range server.Passphrases {
		opts = append(opts, llmmanager.WithPassphrase(uint64(i+1), passphrase))
	}

	// Get the client options from the environment
	_, clientopts, err := ctx.ClientEndpoint()
	if err != nil {
		return nil
	}

	// Get the prompts from the embedded filesystem and set them on the manager options
	prompts, err := server.llmManagerPrompts()
	if err != nil {
		return nil
	}
	opts = append(opts, llmmanager.WithPrompts(prompts...))

	// Return the options with the configured schemas and tracer
	return append(opts,
		llmmanager.WithSchemas(server.Schema.LLM, server.Schema.Auth),
		llmmanager.WithTracer(ctx.Tracer()),
		llmmanager.WithMeter(ctx.Meter()),
		llmmanager.WithClientOpts(clientopts...),
	)
}
