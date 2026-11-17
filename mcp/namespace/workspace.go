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
var _ workspace.Rooted = (*Namespace)(nil)

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
// once it's watching the client, or there's none, so no update is missed,
// and again once it's watching each client after.
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
	ready := func() bool { return send(listAgain) }
	for {
		ns.mu.RLock()
		c, changed := ns.client, ns.clientChanged
		ns.mu.RUnlock()

		// Without a client, there's nothing to miss, so it's watching, and
		// waits for one
		if c == nil {
			if !ready() {
				return nil
			}
			select {
			case <-changed:
				continue
			case <-ctx.Done():
				return nil
			}
		}

		// With one, subscribe to the resources which are subscribed to, and
		// watch it, and say the resources changed, once it's watching, until
		// it changes
		if !ns.watchClient(ctx, c, changed, ready, send) {
			return nil
		}
	}
}

// Subscribe asks for the resource with the URI's updates, from the server
// the namespace is connected to, and each it's connected to after
func (ns *Namespace) Subscribe(ctx context.Context, uri string) error {
	ns.subscribing.Lock()
	defer ns.subscribing.Unlock()
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
	ns.subscribing.Lock()
	defer ns.subscribing.Unlock()
	ns.mu.Lock()
	delete(ns.subscribed, uri)
	c := ns.client
	ns.mu.Unlock()
	if c == nil {
		return nil
	}
	return c.Unsubscribe(ctx, uri)
}

// Root returns the root of the server's resources' URIs, of its resource
// template which is one, such as file:///{+path}, or nothing, when it has
// none, or isn't connected
func (ns *Namespace) Root() string {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.root
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
// to, and watches it, and calls ready once it's watching, or its watch
// ended, so no change is missed, and sends its changes, until the client
// changes, when it returns true, or the context is cancelled, or ready, or
// send, fails, when it returns false. Its requests are cancelled as the
// client changes, so a server which doesn't answer isn't waited for.
func (ns *Namespace) watchClient(ctx context.Context, c *client.Client, changed <-chan struct{}, ready func() bool, send func(schema.Change) bool) bool {
	clientCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-changed:
			cancel()
		case <-clientCtx.Done():
		}
	}()
	ns.resubscribe(clientCtx, c)

	updated := make(chan string)
	done := make(chan error, 1)
	go func() { done <- c.WatchResources(clientCtx, updated) }()
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
			if !watching {
				// It's watching, which it says first
				watching = true
				if !ready() {
					return false
				} else if uri == "" {
					continue
				}
			}
			if uri != "" && !ns.isSubscribed(uri) {
				// Unsubscribed, which a server can be told of after it's
				// said it's updated
				continue
			}
			if !send(schema.Change{Action: eventschema.EventUpdate, URI: uri}) {
				return false
			}
		case <-done:
			// Its watch ended, so wait for the client to change
			ended = true
			if !watching && !ready() {
				return false
			}
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

// isSubscribed returns true when the resource with the URI is subscribed to
func (ns *Namespace) isSubscribed(uri string) bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.subscribed[uri]
}

// resubscribe subscribes the client to the resources which are subscribed
// to, while none is subscribed to, or unsubscribed from
func (ns *Namespace) resubscribe(ctx context.Context, c *client.Client) {
	ns.subscribing.Lock()
	defer ns.subscribing.Unlock()
	ns.mu.RLock()
	uris := make([]string, 0, len(ns.subscribed))
	for uri := range ns.subscribed {
		uris = append(uris, uri)
	}
	ns.mu.RUnlock()
	for _, uri := range uris {
		if ctx.Err() != nil {
			return
		}
		_ = c.Subscribe(ctx, uri)
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
