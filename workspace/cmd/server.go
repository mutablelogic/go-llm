//go:build !client

// Package cmd has the workspaces' flags, and their registry, for the server
package cmd

import (
	// Packages
	workspace "github.com/mutablelogic/go-llm/workspace"
	httphandler "github.com/mutablelogic/go-llm/workspace/httphandler"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Flags are the workspaces' flags for the server, which has none yet
type Flags struct{}

// FilerFlags are the flags of the filer, which is a workspace, of the files
// of a folder, in the server, when its path is set
type FilerFlags struct {
	Path        string `name:"path" help:"Folder whose files are a workspace, or none, when it's not set." type:"path"`
	Description string `name:"description" help:"What the folder's files are, which models are told, or \"The files in\" its path, when it's not set."`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithWorkspaces creates the registry of the server's workspaces, with the
// options, such as the broadcaster its changes are published to, registers
// its HTTP handlers with the server, when there is one, and calls the
// function with it, returning any error. Its watches end when the function
// returns.
func (flags Flags) WithWorkspaces(ctx server.Cmd, srv *servercmd.RunServer, fn func(*workspace.Registry) error, opts ...workspace.Opt) error {
	registry, err := workspace.New(opts...)
	if err != nil {
		return err
	}
	defer registry.Close()

	// Register the HTTP handlers
	if srv != nil {
		srv.Register(func(router *httprouter.Router) error {
			ctx.Logger().DebugContext(ctx.Context(), "registering workspace http handlers")
			return httphandler.RegisterHandlers(registry, router)
		})
	}

	// Call the function with the registry
	return fn(registry)
}
