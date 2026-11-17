package namespace

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	client "github.com/mutablelogic/go-llm/mcp/client"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var _ workspace.Reader = (*Namespace)(nil)
var _ workspace.Available = (*Namespace)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Available returns true when the namespace's server is a workspace, which
// it is while it's connected, and says it has resources, as an MCP server's
// resources are listed, and read, by folder, or all of them, when it
// ignores the folder
func (ns *Namespace) Available() bool {
	c := ns.current()
	return c != nil && c.Connected() && c.HasResources()
}

// List returns a page of the server's folders, and resources, as
// workspace.NewClient's workspace does. Returns ErrServiceUnavailable when
// it isn't connected.
func (ns *Namespace) List(ctx context.Context, req schema.ListRequest) (*schema.ListResponse, error) {
	ws, err := ns.workspace()
	if err != nil {
		return nil, err
	}
	return ws.List(ctx, req)
}

// Read returns the server's resource, or artwork, with the URI, as
// workspace.NewClient's workspace does. Returns ErrServiceUnavailable when
// it isn't connected.
func (ns *Namespace) Read(ctx context.Context, uri string) (llm.Resource, error) {
	ws, err := ns.workspace()
	if err != nil {
		return nil, err
	}
	return ws.Read(ctx, uri)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// current returns the client the namespace is connected with, or nil
func (ns *Namespace) current() *client.Client {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.client
}

// workspace returns the workspace of the server the namespace is connected
// to. Returns ErrServiceUnavailable when it isn't.
func (ns *Namespace) workspace() (workspace.Reader, error) {
	c := ns.current()
	if c == nil {
		return nil, llm.ErrServiceUnavailable.Withf("namespace %q isn't connected", ns.name)
	}
	return workspace.NewClient(c)
}
