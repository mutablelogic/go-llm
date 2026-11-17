//go:build !client

// Package cmd has the workspaces' flags, and their registry, for the server
package cmd

import (
	// Packages
	workspace "github.com/mutablelogic/go-llm/workspace"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Flags are the workspaces' flags for the server, which has none yet
type Flags struct{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithWorkspaces creates the registry of the server's workspaces, with the
// options, such as the broadcaster its changes are published to, and calls
// the function with it, returning any error. Its watches end when the
// function returns.
func (flags Flags) WithWorkspaces(ctx server.Cmd, srv *servercmd.RunServer, fn func(*workspace.Registry) error, opts ...workspace.Opt) error {
	registry, err := workspace.New(opts...)
	if err != nil {
		return err
	}
	defer registry.Close()
	ctx.Logger().DebugContext(ctx.Context(), "workspaces created")
	return fn(registry)
}
