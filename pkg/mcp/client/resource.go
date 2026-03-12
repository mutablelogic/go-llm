package client

import (
	"context"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// clientResource wraps an *sdkmcp.Resource received from a server and implements
// llm.Resource. Read issues a resources/read call against the server on demand.
type clientResource struct {
	sess *sdkmcp.ClientSession
	r    *sdkmcp.Resource
}

// Ensure clientResource implements llm.Resource at compile time.
var _ llm.Resource = (*clientResource)(nil)

///////////////////////////////////////////////////////////////////////////////
// llm.Resource INTERFACE

func (r *clientResource) URI() string         { return r.r.URI }
func (r *clientResource) Name() string        { return r.r.Name }
func (r *clientResource) Description() string { return r.r.Description }
func (r *clientResource) Type() string        { return r.r.MIMEType }

func (r *clientResource) Read(ctx context.Context) ([]byte, error) {
	result, err := r.sess.ReadResource(ctx, &sdkmcp.ReadResourceParams{URI: r.r.URI})
	if err != nil {
		return nil, err
	}
	for _, c := range result.Contents {
		if c.Blob != nil {
			return c.Blob, nil
		}
		return []byte(c.Text), nil
	}
	return nil, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListResources returns the cached list of resources advertised by the
// connected MCP server. The cache is populated on connect and refreshed
// automatically on each ResourceListChanged notification.
// Returns ErrNotConnected if no session is active.
func (c *Client) ListResources(_ context.Context) ([]llm.Resource, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil, ErrNotConnected
	}
	return c.resources, nil
}

// readResource fetches a single resource by URI from the server and wraps it
// in a clientResource. Returns nil if not connected or the read fails.
func (c *Client) readResource(ctx context.Context, uri string) llm.Resource {
	sess, err := c.getSession()
	if err != nil {
		return nil
	}
	// Look up the resource metadata from the cache.
	c.mu.Lock()
	var meta *sdkmcp.Resource
	for _, r := range c.resources {
		if cr, ok := r.(*clientResource); ok && cr.r.URI == uri {
			meta = cr.r
			break
		}
	}
	c.mu.Unlock()
	if meta == nil {
		// URI not in the cached list — synthesise minimal metadata.
		meta = &sdkmcp.Resource{URI: uri}
	}
	return &clientResource{sess: sess, r: meta}
}

// refreshResources fetches the full resource list from the server, stores it
// in the cache and invokes onResourceListChanged if set.
func (c *Client) refreshResources(ctx context.Context) {
	sess, err := c.getSession()
	if err != nil {
		return
	}
	c.mu.Lock()
	resources := make([]llm.Resource, 0, len(c.resources)+10)
	c.mu.Unlock()
	for cursor := ""; ; {
		result, err := sess.ListResources(ctx, &sdkmcp.ListResourcesParams{Cursor: cursor})
		if err != nil {
			return
		}
		for _, r := range result.Resources {
			resources = append(resources, &clientResource{sess: sess, r: r})
		}
		if cursor = result.NextCursor; cursor == "" {
			break
		}
	}
	c.mu.Lock()
	c.resources = resources
	fn := c.onResourceListChanged
	updatedFn := c.onResourceUpdated
	// Collect URIs that need subscribing (only when onResourceUpdated is wired).
	var toSubscribe []string
	if updatedFn != nil {
		if c.subscribed == nil {
			c.subscribed = make(map[string]struct{})
		}
		for _, r := range resources {
			if _, ok := c.subscribed[r.URI()]; !ok {
				c.subscribed[r.URI()] = struct{}{}
				toSubscribe = append(toSubscribe, r.URI())
			}
		}
	}
	c.mu.Unlock()
	if fn != nil {
		fn(ctx)
	}
	// For each newly-seen URI: subscribe (so subsequent ResourceUpdated
	// notifications reach us) and fire onResourceUpdated immediately
	// (the resource just appeared — its content is new to us).
	for _, uri := range toSubscribe {
		sess.Subscribe(ctx, &sdkmcp.SubscribeParams{URI: uri}) //nolint:errcheck
		if updatedFn != nil {
			updatedFn(ctx, c.readResource(ctx, uri))
		}
	}
}
