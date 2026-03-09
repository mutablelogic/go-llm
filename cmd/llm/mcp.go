//go:build !client

package main

import (
	"fmt"
	"os"
	"path/filepath"

	// Packages
	heartbeat "github.com/mutablelogic/go-llm/pkg/heartbeat"
	mcpserver "github.com/mutablelogic/go-llm/pkg/mcp/server"
	server "github.com/mutablelogic/go-server"
	gocmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	version "github.com/mutablelogic/go-server/pkg/version"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type MCPCommands struct {
	HeartbeatMCP HeartbeatMCPCommand `cmd:"" name:"heartbeat" help:"Run the heartbeat MCP server." group:"SERVER"`
}

type HeartbeatMCPCommand struct {
	gocmd.RunServer
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *HeartbeatMCPCommand) Run(ctx server.Cmd) error {
	// Create the file-backed heartbeat store
	store, err := cmd.HeartbeatStore(ctx.Name())
	if err != nil {
		return err
	}

	// Create the heartbeat manager
	mgr, err := heartbeat.New(store)
	if err != nil {
		return fmt.Errorf("heartbeat manager: %w", err)
	}

	// Create the MCP server
	srv, err := mcpserver.New("heartbeat", version.Version())
	if err != nil {
		return fmt.Errorf("mcp server: %w", err)
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

	// Run the heartbeat manager alongside the HTTP server
	go func() {
		if err := mgr.Run(ctx.Context()); err != nil {
			ctx.Logger().ErrorContext(ctx.Context(), "heartbeat manager error", "error", err)
		}
	}()

	return cmd.RunServer.Run(ctx)
}

// HeartbeatStore returns the heartbeat store, creating it lazily.
// Heartbeats are stored in the user's cache directory.
func (cmd *HeartbeatMCPCommand) HeartbeatStore(execName string) (*heartbeat.Store, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine cache directory: %w", err)
	}
	store, err := heartbeat.NewStore(filepath.Join(cache, execName, "heartbeats"))
	if err != nil {
		return nil, fmt.Errorf("failed to create heartbeat store: %w", err)
	}
	return store, nil
}
