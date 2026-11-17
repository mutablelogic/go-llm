package namespace

import (
	"context"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	client "github.com/mutablelogic/go-llm/mcp/client"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var _ workspace.Reader = (*Namespace)(nil)
var _ workspace.Available = (*Namespace)(nil)
var _ workspace.Describer = (*Namespace)(nil)
var _ workspace.Subscriber = (*Namespace)(nil)

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

// WatchResources sends the server's resources which are updated, which are
// subscribed to, with Subscribe, and, without a URI, that its resources
// changed, as it says, or as the namespace is connected to another server,
// or the same again, to the channel, until the context is cancelled, when it
// returns nil. It follows the client the namespace is connected with, which
// changes, as it's connected again. It sends a change without a URI first,
// once it's watching.
func (ns *Namespace) WatchResources(ctx context.Context, ch chan<- schema.Change) error {
	if ch == nil {
		return llm.ErrBadParameter.With("missing channel")
	}
	listAgain := schema.Change{Action: eventschema.EventUpdate}
	send := func(change schema.Change) bool {
		select {
		case ch <- change:
			return true
		case <-ctx.Done():
			return false
		}
	}
	if !send(listAgain) {
		return nil
	}
	for first := true; ; first = false {
		ns.mu.RLock()
		c, changed := ns.client, ns.clientChanged
		ns.mu.RUnlock()

		// The resources changed, as the client did
		if !first && !send(listAgain) {
			return nil
		}

		// Without a client, wait for one
		if c == nil {
			select {
			case <-changed:
				continue
			case <-ctx.Done():
				return nil
			}
		}

		// With one, subscribe to the resources which are subscribed to, and
		// watch it, until it changes
		if !ns.watchClient(ctx, c, changed, send) {
			return nil
		}
	}
}

// Subscribe asks for the resource with the URI's updates, from the server
// the namespace is connected to, and each it's connected to after
func (ns *Namespace) Subscribe(ctx context.Context, uri string) error {
	ns.mu.Lock()
	ns.subscribed[uri] = true
	c := ns.client
	ns.mu.Unlock()
	if c == nil {
		return nil
	}
	return c.Subscribe(ctx, uri)
}

// Unsubscribe stops asking for the resource with the URI's updates
func (ns *Namespace) Unsubscribe(ctx context.Context, uri string) error {
	ns.mu.Lock()
	delete(ns.subscribed, uri)
	c := ns.client
	ns.mu.Unlock()
	if c == nil {
		return nil
	}
	return c.Unsubscribe(ctx, uri)
}

// Title returns the title of the server the namespace is connected to, or
// its name, when it has none, or nothing, when it isn't connected
func (ns *Namespace) Title() string {
	if server := ns.server(); server != nil {
		if server.Title != "" {
			return server.Title
		}
		return server.Name
	}
	return ""
}

// Version returns the version of the server the namespace is connected to,
// or nothing, when it isn't connected
func (ns *Namespace) Version() string {
	if server := ns.server(); server != nil {
		return server.Version
	}
	return ""
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

// watchClient subscribes the client to the resources which are subscribed
// to, and sends its changes, other than the first, which says it's
// watching, until the client changes, when it returns true, or the context
// is cancelled, when it returns false
func (ns *Namespace) watchClient(ctx context.Context, c *client.Client, changed <-chan struct{}, send func(schema.Change) bool) bool {
	ns.mu.RLock()
	uris := make([]string, 0, len(ns.subscribed))
	for uri := range ns.subscribed {
		uris = append(uris, uri)
	}
	ns.mu.RUnlock()
	for _, uri := range uris {
		_ = c.Subscribe(ctx, uri)
	}

	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	updated := make(chan string)
	done := make(chan error, 1)
	go func() { done <- c.WatchResources(watchCtx, updated) }()
	ended := false
	defer func() {
		cancel()
		if !ended {
			<-done
		}
	}()
	watching := false
	for {
		select {
		case uri := <-updated:
			if !watching && uri == "" {
				watching = true
				continue
			}
			watching = true
			if !send(schema.Change{Action: eventschema.EventUpdate, URI: uri}) {
				return false
			}
		case <-done:
			// Its watch ended, so wait for the client to change
			ended = true
			select {
			case <-changed:
				return true
			case <-ctx.Done():
				return false
			}
		case <-changed:
			return true
		case <-ctx.Done():
			return false
		}
	}
}

// server returns the server the namespace is connected to, as it said, or
// nil, when it isn't connected
func (ns *Namespace) server() *sdkmcp.Implementation {
	c := ns.current()
	if c == nil {
		return nil
	}
	server, _ := c.Status()
	return server
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
