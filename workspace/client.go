package workspace

import (
	"context"
	"path"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	client "github.com/mutablelogic/go-llm/mcp/client"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// clientWorkspace is a workspace which is an MCP server's resources, which
// are listed, and read, with its client
type clientWorkspace struct {
	client *client.Client
}

// contents is a resource's content, as an MCP server returned it
type contents struct {
	uri, name, mimeType string
	data                []byte
}

var _ Reader = (*clientWorkspace)(nil)
var _ llm.Resource = (*contents)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewClient returns the workspace of the resources of the MCP server,
// whose client is connected. A server which is a workspace, as its
// capability says, lists its folders; one which isn't lists all its
// resources. Its name, title, and description are the server's, or its
// namespace's. Returns ErrBadParameter when there's no client.
func NewClient(c *client.Client) (Reader, error) {
	if c == nil {
		return nil, llm.ErrBadParameter.With("missing client")
	}
	return &clientWorkspace{client: c}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// List returns a page of the server's resources, in the folder, or under
// it, as the request's _meta says, which a server which isn't a workspace
// ignores
func (w *clientWorkspace) List(ctx context.Context, req schema.ListRequest) (*schema.ListResponse, error) {
	result, err := w.client.ListResources(ctx, req.MCP())
	if err != nil {
		return nil, err
	}
	response := &schema.ListResponse{Body: make([]schema.ResourceMeta, 0, len(result.Resources)), Next: result.NextCursor}
	for _, resource := range result.Resources {
		if resource != nil {
			response.Body = append(response.Body, schema.ResourceMetaOf(resource))
		}
	}
	return response, nil
}

// Read returns the server's resource, or artwork, with the URI, as its
// first contents are, which are text, or data. Returns ErrNotFound when the
// server returns no contents.
func (w *clientWorkspace) Read(ctx context.Context, uri string) (llm.Resource, error) {
	result, err := w.client.ReadResource(ctx, uri)
	if err != nil {
		return nil, err
	} else if len(result.Contents) == 0 || result.Contents[0] == nil {
		return nil, llm.ErrNotFound.Withf("resource %q has no contents", uri)
	}
	c := result.Contents[0]
	data := c.Blob
	if data == nil {
		data = []byte(c.Text)
	}
	if c.URI != "" {
		uri = c.URI
	}
	return &contents{uri: uri, name: path.Base(strings.TrimSuffix(uri, "/")), mimeType: c.MIMEType, data: data}, nil
}

///////////////////////////////////////////////////////////////////////////////
// CONTENTS

func (c *contents) URI() string         { return c.uri }
func (c *contents) Name() string        { return c.name }
func (c *contents) Description() string { return "" }
func (c *contents) Type() string        { return c.mimeType }
func (c *contents) ContentLength() int64 {
	return int64(len(c.data))
}
func (c *contents) Read(context.Context) (llm.ReadFunc, error) {
	return llm.ReadBytes(c.data), nil
}
