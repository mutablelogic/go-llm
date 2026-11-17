package client

import (
	"context"
	"sync"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// HasResources returns true when the server says it has resources, which
// are listed, and read
func (c *Client) HasResources() bool {
	session, err := c.connected()
	if err != nil {
		return false
	}
	result := session.InitializeResult()
	return result != nil && result.Capabilities != nil && result.Capabilities.Resources != nil
}

// ListResources returns a page of the server's resources, from the params'
// cursor, with its _meta, which a server can use, such as to list a folder,
// or ignore, and the cursor of the next page, which is empty when there
// isn't one. Returns no resources when the server doesn't say it has any,
// and ErrServiceUnavailable when it isn't connected.
func (c *Client) ListResources(ctx context.Context, params *sdkmcp.ListResourcesParams) (*sdkmcp.ListResourcesResult, error) {
	session, err := c.connected()
	if err != nil {
		return nil, err
	} else if !c.HasResources() {
		return &sdkmcp.ListResourcesResult{}, nil
	}
	if params == nil {
		params = new(sdkmcp.ListResourcesParams)
	}
	return session.ListResources(ctx, params)
}

// ListResourceTemplates returns the server's resource templates, of each
// page, or none, when it doesn't say it has resources. Returns
// ErrServiceUnavailable when it isn't connected.
func (c *Client) ListResourceTemplates(ctx context.Context) ([]*sdkmcp.ResourceTemplate, error) {
	session, err := c.connected()
	if err != nil {
		return nil, err
	} else if !c.HasResources() {
		return nil, nil
	}
	var result []*sdkmcp.ResourceTemplate
	for template, err := range session.ResourceTemplates(ctx, nil) {
		if err != nil {
			return nil, err
		}
		result = append(result, template)
	}
	return result, nil
}

// ReadResource returns the contents of the server's resource with the URI.
// Returns ErrServiceUnavailable when it isn't connected.
func (c *Client) ReadResource(ctx context.Context, uri string) (*sdkmcp.ReadResourceResult, error) {
	session, err := c.connected()
	if err != nil {
		return nil, err
	}
	return session.ReadResource(ctx, &sdkmcp.ReadResourceParams{URI: uri})
}

// HasExperimental returns true when the server says it has the experimental
// capability with the name, such as a workspace's
func (c *Client) HasExperimental(name string) bool {
	session, err := c.connected()
	if err != nil {
		return false
	}
	result := session.InitializeResult()
	if result == nil || result.Capabilities == nil {
		return false
	}
	_, exists := result.Capabilities.Experimental[name]
	return exists
}

// WatchResources sends the URI of each of the server's resources which is
// updated, which it was subscribed to, and an empty URI when the server says
// its list of resources changed, so it's listed again, to the channel,
// until the context is cancelled, when it returns nil. It sends an empty
// URI first, once it's watching, so a change after that isn't missed. Each
// is waited for the channel to receive. Returns ErrBadParameter when
// there's no channel.
func (c *Client) WatchResources(ctx context.Context, ch chan<- string) error {
	if ch == nil {
		return llm.ErrBadParameter.With("missing channel")
	}
	w := &resourceWatch{ctx: ctx, ch: ch}
	c.resources.mu.Lock()
	if c.resources.watches == nil {
		c.resources.watches = make(map[*resourceWatch]struct{})
	}
	c.resources.watches[w] = struct{}{}
	c.resources.mu.Unlock()

	// It's watching, which is said
	select {
	case ch <- "":
	case <-ctx.Done():
	}
	<-ctx.Done()
	c.resources.mu.Lock()
	delete(c.resources.watches, w)
	c.resources.mu.Unlock()
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// RESOURCE WATCHES

// resourceWatches are the watches of a server's resources
type resourceWatches struct {
	mu      sync.Mutex
	watches map[*resourceWatch]struct{}
}

// resourceWatch is a channel which is sent the URIs of resources which
// change, until its context is cancelled
type resourceWatch struct {
	ctx context.Context
	ch  chan<- string
}

// changed sends the URI to each watch, which it waits for, until it's
// received, or the watch is cancelled
func (w *resourceWatches) changed(uri string) {
	w.mu.Lock()
	watches := make([]*resourceWatch, 0, len(w.watches))
	for watch := range w.watches {
		watches = append(watches, watch)
	}
	w.mu.Unlock()
	for _, watch := range watches {
		select {
		case watch.ch <- uri:
		case <-watch.ctx.Done():
		}
	}
}

// ReadResourceParams returns the contents of the server's resource, as the
// params ask, such as a chunk of it, in their _meta. Returns
// ErrServiceUnavailable when it isn't connected.
func (c *Client) ReadResourceParams(ctx context.Context, params *sdkmcp.ReadResourceParams) (*sdkmcp.ReadResourceResult, error) {
	session, err := c.connected()
	if err != nil {
		return nil, err
	}
	return session.ReadResource(ctx, params)
}
