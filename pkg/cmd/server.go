package cmd

import (
	"context"
	"fmt"

	// Packages
	authmanager "github.com/djthorpe/go-auth/pkg/authmanager"
	authhanders "github.com/djthorpe/go-auth/pkg/httphandler/authmanager"
	llmhandlers "github.com/mutablelogic/go-llm/pkg/httphandler-new"
	llmmanager "github.com/mutablelogic/go-llm/pkg/llmmanager"
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
		Auth string `name:"auth" help:"PostgreSQL schema for auth data." default:"auth"`
		LLM  string `name:"llm" help:"PostgreSQL schema for LLM data." default:"llm"`
	} `embed:"" prefix:"schema."`

	// Other flags
	Passphrases []string `name:"passphrase" env:"${ENV_NAME}_PASSPHRASES" help:"One or more passphrases used to encrypt credentials. "`
	Auth        bool     `name:"auth" help:"Enable authentication for protected endpoints." default:"true" negatable:""`
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
	return runner.withAuthManager(ctx, conn, func(authmanager *authmanager.Manager) error {
		return runner.withLLMManager(ctx, conn, func(llmmanager *llmmanager.Manager) error {
			if updates, deletes, err := llmmanager.SyncProviders(ctx.Context()); err != nil {
				ctx.Logger().ErrorContext(ctx.Context(), "failed to sync llm providers before startup", "error", err.Error())
			} else {
				if len(updates) > 0 {
					ctx.Logger().InfoContext(ctx.Context(), "updated providers", "providers", updates)
				}
				if len(deletes) > 0 {
					ctx.Logger().InfoContext(ctx.Context(), "deleted providers", "providers", deletes)
				}
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

func (server *RunServer) withLLMManager(ctx server.Cmd, conn pg.PoolConn, fn func(manager *llmmanager.Manager) error) error {
	// Create the LLM manager
	llmmanager, err := llmmanager.New(ctx.Context(), conn, server.llmManagerOpts(ctx)...)
	if err != nil {
		return err
	}
	defer llmmanager.Close()

	// Return nil on success
	return fn(llmmanager)
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

	// Return the options with the configured schemas and tracer
	return append(opts,
		llmmanager.WithSchemas(server.Schema.LLM, server.Schema.Auth),
		llmmanager.WithTracer(ctx.Tracer()),
		llmmanager.WithMeter(ctx.Meter()),
		llmmanager.WithClientOpts(clientopts...),
	)
}
