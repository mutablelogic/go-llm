//go:build !client

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	// Packages
	heartbeat "github.com/mutablelogic/go-llm/pkg/heartbeat"
	heartbeatfile "github.com/mutablelogic/go-llm/pkg/heartbeat/file"
	heartbeatpg "github.com/mutablelogic/go-llm/pkg/heartbeat/pg"
	mcpserver "github.com/mutablelogic/go-llm/pkg/mcp/server"
	server "github.com/mutablelogic/go-server"
	gocmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	errgroup "golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type MCPCommands struct {
	HeartbeatMCP HeartbeatMCPCommand `cmd:"" name:"heartbeat" help:"Run the heartbeat MCP server." group:"SERVER"`
}

type HeartbeatMCPCommand struct {
	gocmd.RunServer
	PostgresFlags `embed:"" prefix:"pg."`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *HeartbeatMCPCommand) Run(ctx server.Cmd) error {
	// Optionally connect to the database
	pool, err := cmd.Connect(ctx)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	} else if pool != nil {
		ctx.Logger().InfoContext(ctx.Context(), "connected to database")
	}
	defer func() {
		if pool != nil {
			pool.Close()
		}
	}()

	// Create the heartbeat store: prefer database-backed when a pool is available.
	var store heartbeat.Store
	if pool != nil {
		store, err = heartbeatpg.NewStore(ctx.Context(), pool)
		if err != nil {
			return fmt.Errorf("failed to create pg heartbeat store: %w", err)
		}
	} else {
		store, err = cmd.HeartbeatStore(ctx.Name())
		if err != nil {
			return err
		}
	}

	// Create the MCP server
	srv, err := mcpserver.New(ctx.Name()+"/heartbeat", ctx.Version())
	if err != nil {
		return fmt.Errorf("mcp server: %w", err)
	}

	// Create the heartbeat manager; on each fire, upsert a resource so connected
	// clients receive notifications/resources/list_changed automatically.
	var mgrOpts []heartbeat.Opt
	mgrOpts = append(mgrOpts, heartbeat.WithLogger(ctx.Logger()))
	if t := ctx.Tracer(); t != nil {
		mgrOpts = append(mgrOpts, heartbeat.WithTracer(t))
	}
	mgrOpts = append(mgrOpts, heartbeat.WithOnFire(func(_ context.Context, h *heartbeat.Heartbeat) {
		u, _ := url.Parse("heartbeat:" + h.ID)
		raw, _ := json.Marshal(h)
		srv.AddResources(&heartbeatResource{
			uri:  u.String(),
			name: h.Message,
			data: raw,
		}) //nolint:errcheck
	}))
	mgr, err := heartbeat.New(store, mgrOpts...)
	if err != nil {
		return fmt.Errorf("heartbeat manager: %w", err)
	}

	// Register the heartbeat tools with the MCP server
	tools, err := mgr.ListTools(ctx.Context())
	if err != nil {
		return fmt.Errorf("listing heartbeat tools: %w", err)
	}
	if err := srv.AddTools(tools...); err != nil {
		return fmt.Errorf("registering heartbeat tools: %w", err)
	}

	// Mount the MCP handler on the router at the HTTP prefix
	cmd.Register(func(router *httprouter.Router, _ server.Cmd) error {
		return router.RegisterFunc("", srv.Handler().ServeHTTP, false, nil)
	})

	// Run the heartbeat manager and HTTP server concurrently; wait for both
	// before returning so the deferred pool.Close() doesn't race with an
	// in-progress tick.
	eg, egCtx := errgroup.WithContext(ctx.Context())
	eg.Go(func() error {
		return mgr.Run(egCtx)
	})
	eg.Go(func() error {
		return cmd.RunServer.Run(ctx)
	})

	// Wait for both the manager and server to exit (e.g. on context cancellation)
	return eg.Wait()
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE TYPES

// heartbeatResource implements llm.Resource for a fired heartbeat, exposing
// the message as the name and the marshalled JSON as readable content.
type heartbeatResource struct {
	uri  string
	name string
	data []byte
}

func (r *heartbeatResource) URI() string         { return r.uri }
func (r *heartbeatResource) Name() string        { return r.name }
func (r *heartbeatResource) Description() string { return "" }
func (r *heartbeatResource) MIMEType() string    { return "application/json" }
func (r *heartbeatResource) Read(_ context.Context) ([]byte, error) {
	return r.data, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// HeartbeatStore returns the heartbeat store, creating it lazily.
// Heartbeats are stored in the user's cache directory.
func (cmd *HeartbeatMCPCommand) HeartbeatStore(execName string) (heartbeat.Store, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine cache directory: %w", err)
	}
	store, err := heartbeatfile.NewStore(filepath.Join(cache, execName, "heartbeats"))
	if err != nil {
		return nil, fmt.Errorf("failed to create heartbeat store: %w", err)
	}
	return store, nil
}
