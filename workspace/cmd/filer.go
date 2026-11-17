//go:build !client

package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	// Packages
	gofiler "github.com/mutablelogic/go-filer"
	filerfs "github.com/mutablelogic/go-filer/filer/fs"
	client "github.com/mutablelogic/go-llm/mcp/client"
	namespace "github.com/mutablelogic/go-llm/mcp/namespace"
	mcpserver "github.com/mutablelogic/go-llm/mcp/server"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	workspace "github.com/mutablelogic/go-llm/workspace"
	filer "github.com/mutablelogic/go-llm/workspace/filer"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// filerNamespace is the namespace of the filer's server, whose resources'
// changes are all of its backend's, as it's in the same process, rather
// than those of the resources its client subscribed to, which an MCP
// client's are, so a file which is edited is said to be, once it's watching
type filerNamespace struct {
	*namespace.Namespace
	workspace *filer.Workspace
}

var _ workspace.Watcher = (*filerNamespace)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// FilerName is the name of the workspace of the folder, which is its
	// namespace's, and its MCP server's
	FilerName = "files"

	// FilerTitle is the title of the workspace of the folder
	FilerTitle = "Files"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithFiler serves the folder, when its path is set, as an MCP server of its
// files, with the command's version, which its client is connected to, in
// memory, and whose namespace is added to the toolkit, so it's a workspace,
// and calls the function with the server, which it runs, or nil, when
// its path isn't set. Once the function returns, the namespace is removed,
// and the client disconnected.
func (flags FilerFlags) WithFiler(ctx server.Cmd, tools *toolkit.Toolkit, fn func(*filer.Server) error) error {
	if flags.Path == "" {
		return fn(nil)
	}
	dir, err := filepath.Abs(flags.Path)
	if err != nil {
		return err
	}
	description := strings.TrimSpace(flags.Description)
	if description == "" {
		description = fmt.Sprintf("The files in %s", dir)
	}

	// The backend of the folder, and its server
	backend, err := filerfs.New(FilerName, dir, nil, gofiler.Opts{Tracer: ctx.Tracer()})
	if err != nil {
		return err
	}
	defer backend.Close()
	srv, err := filer.NewServer(FilerName, FilerTitle, backend,
		mcpserver.WithVersion(ctx.Version()),
		mcpserver.WithLogger(ctx.Logger()),
		mcpserver.WithTracer(ctx.Tracer()),
	)
	if err != nil {
		return err
	}

	// Its client, in memory, and the namespace of its server, as the
	// connectors' are, which is a workspace, in the toolkit
	c, err := client.New("memory://"+FilerName, client.WithTransport(srv.InMemoryTransport), client.WithName(ctx.Name(), ctx.Version()))
	if err != nil {
		return err
	} else if err := c.Connect(ctx.Context()); err != nil {
		return err
	}
	defer c.Disconnect()
	ns, err := namespace.New(FilerName, description)
	if err != nil {
		return err
	} else if err := ns.Connect(ctx.Context(), c, description, nil, 1); err != nil {
		return err
	}
	files := &filerNamespace{Namespace: ns, workspace: srv.Workspace()}
	if err := tools.Add(files); err != nil {
		return err
	}
	defer tools.Remove(files)
	ctx.Logger().InfoContext(ctx.Context(), "workspace added", "namespace", FilerName, "dir", dir)

	// Call the function with the server
	return fn(srv)
}

///////////////////////////////////////////////////////////////////////////////
// FILER NAMESPACE

// WatchResources sends the backend's changes, as its workspace does
func (n *filerNamespace) WatchResources(ctx context.Context, ch chan<- schema.Change) error {
	return n.workspace.WatchResources(ctx, ch)
}
