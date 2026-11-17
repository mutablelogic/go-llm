package client

import (
	"context"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
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

// ReadResource returns the contents of the server's resource with the URI.
// Returns ErrServiceUnavailable when it isn't connected.
func (c *Client) ReadResource(ctx context.Context, uri string) (*sdkmcp.ReadResourceResult, error) {
	session, err := c.connected()
	if err != nil {
		return nil, err
	}
	return session.ReadResource(ctx, &sdkmcp.ReadResourceParams{URI: uri})
}
