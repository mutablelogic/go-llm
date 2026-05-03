package cmd

import (
	"bytes"
	"fmt"
	"io/fs"

	// Packages
	httpclient "github.com/mutablelogic/go-auth/auth/httpclient"
	llm "github.com/mutablelogic/go-llm"
	agent "github.com/mutablelogic/go-llm/etc/agent"
	kernel "github.com/mutablelogic/go-llm/kernel/manager"
	manager "github.com/mutablelogic/go-llm/kernel/manager"
	prompt "github.com/mutablelogic/go-llm/toolkit/prompt"
	pg "github.com/mutablelogic/go-pg"
	pgcmd "github.com/mutablelogic/go-pg/pkg/cmd"
	server "github.com/mutablelogic/go-server"
	cmd "github.com/mutablelogic/go-server/pkg/cmd"
	"github.com/mutablelogic/go-server/pkg/httprouter"
	"golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type RunServer struct {
	cmd.RunServer
	pgcmd.PostgresFlags `embed:"" prefix:"pg."`

	// Schemas for tenancy
	Schema struct {
		LLM    string `name:"llm" help:"PostgreSQL schema for LLM data." default:"llm"`
		Auth   string `name:"auth" help:"PostgreSQL schema for auth data." default:"auth"`
		Memory string `name:"memory" help:"PostgreSQL schema for memory data." default:"memory"`
	} `embed:"" prefix:"schema."`

	// Other flags
	Passphrases []string `name:"passphrase" env:"${ENV_NAME}_PASSPHRASES" help:"One or more passphrases used to encrypt credentials."`
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

	// Create an auth client and manager, and run the server
	return WithAuth(ctx, func(auth *httpclient.Client, endpoint string) error {
		return runner.WithManager(ctx, conn, func(manager *kernel.Manager) error {
			// Sync providers before starting the server so that any configured providers are available immediately
			ctx.Logger().DebugContext(ctx.Context(), "syncing providers before server startup")
			if _, _, err := manager.SyncProviders(ctx.Context()); err != nil {
				ctx.Logger().ErrorContext(ctx.Context(), "failed to sync llm providers before startup", "error", err.Error())
			}

			// Register HTTP handlers
			runner.Register(func(router *httprouter.Router) error {
				ctx.Logger().DebugContext(ctx.Context(), "TODO: registering handlers")
				return nil
			})

			// Create an error group, so that the first error from any of the goroutines will
			// be returned and the others will be cancelled
			errgroup, errctx := errgroup.WithContext(ctx.Context())

			// Run the server
			errgroup.Go(func() error {
				return runner.RunServer.Run(ctx.WithContext(errctx))
			})

			// Run the kernel
			errgroup.Go(func() error {
				return manager.Run(errctx, ctx.Logger())
			})

			// Wait until cancelled
			return errgroup.Wait()
		})
	})
}

///////////////////////////////////////////////////////////////////////////////
// MANAGER WITH OPTIONS

func (server *RunServer) WithManager(ctx server.Cmd, conn pg.PoolConn, fn func(*kernel.Manager) error) error {
	// Gather manager options
	opts, err := server.Opts(ctx)
	if err != nil {
		return err
	}

	// Create an LLM kernel
	manager, err := kernel.New(ctx.Context(), ctx.Name(), ctx.Version(), conn, opts...)
	if err != nil {
		return err
	}

	// Call the function with the manager, and return any error
	return fn(manager)
}

func (server *RunServer) Opts(ctx server.Cmd) ([]manager.Opt, error) {
	opts := []manager.Opt{}

	// Set passphrases for credential encryption
	for i, passphrase := range server.Passphrases {
		opts = append(opts, manager.WithPassphrase(uint64(i+1), passphrase))
	}

	// Get the client options from the environment
	_, clientopts, err := ctx.ClientEndpoint()
	if err != nil {
		return nil, err
	}

	// Get the prompts from the embedded filesystem and set them on the manager options
	prompts, err := server.Prompts()
	if err != nil {
		return nil, err
	}
	opts = append(opts, manager.WithPrompts(prompts...))

	// Return the options with the configured schemas and tracer
	return append(opts,
		manager.WithSchemas(server.Schema.LLM, server.Schema.Auth),
		manager.WithTracer(ctx.Tracer()),
		manager.WithMeter(ctx.Meter()),
		manager.WithClientOpts(clientopts...),
	), nil
}

func (server *RunServer) Prompts() ([]llm.Prompt, error) {
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

type namedReader struct {
	*bytes.Reader
	name string
}

func (r *namedReader) Name() string {
	return r.name
}
