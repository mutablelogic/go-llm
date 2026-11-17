package workspace

import (
	"context"
	"errors"
	"path"
	"strings"
	"sync"
	"time"

	// Packages
	jsonrpc "github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	client "github.com/mutablelogic/go-llm/mcp/client"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// clientWorkspace is a workspace which is an MCP server's resources, which
// are listed, and read, with its client
type clientWorkspace struct {
	client   *client.Client
	rootOnce sync.Once
	root     string
}

// contents is a resource's content, as an MCP server returned it
type contents struct {
	uri, name, mimeType string
	data                []byte
}

var _ Reader = (*clientWorkspace)(nil)
var _ Subscriber = (*clientWorkspace)(nil)
var _ Rooted = (*clientWorkspace)(nil)
var _ Tree = (*clientWorkspace)(nil)
var _ llm.Resource = (*contents)(nil)

// rootTimeout is how long the server's resource templates are waited for,
// to find its root
const rootTimeout = 10 * time.Second

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
	response := &schema.ListResponse{Body: make([]schema.ResourceMeta, 0, len(result.Resources)), Next: result.NextCursor, Count: schema.CountOf(result)}
	for _, resource := range result.Resources {
		if resource != nil {
			response.Body = append(response.Body, schema.ResourceMetaOf(resource))
		}
	}
	return response, nil
}

// Tree returns true when the server says its folders are listed on their
// own, in its workspace capability
func (w *clientWorkspace) Tree() bool {
	return IsTree(w.client)
}

// IsTree returns true when the client's server says its folders are listed
// on their own, in its workspace capability
func IsTree(c *client.Client) bool {
	tree, _ := c.Experimental(schema.Capability)[schema.CapabilityTree].(bool)
	return tree
}

// Root returns the root of the server's resources' URIs, of its resource
// template which is one, such as file:///{+path}, which is listed once, or
// nothing, when it has none
func (w *clientWorkspace) Root() string {
	w.rootOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), rootTimeout)
		defer cancel()
		if templates, err := w.client.ListResourceTemplates(ctx); err == nil {
			w.root = schema.RootOf(templates)
		}
	})
	return w.root
}

// Read returns the server's resource, or artwork, with the URI, as its
// first contents are, which are text, or data, a chunk at a time, as it's
// read, when the server is a workspace, and it's larger than a chunk.
// Returns ErrNotFound when the server returns no contents.
func (w *clientWorkspace) Read(ctx context.Context, uri string) (llm.Resource, error) {
	// The first chunk, which is all of it, when it's small, or the server
	// isn't a workspace, which returns all of it, without its size
	data, info, err := readChunk(ctx, w.client, uri, 0)
	if err != nil {
		return nil, err
	}
	// It's all of it, when it isn't a chunk, or it's as large as its size,
	// or, when its size isn't known, it's smaller than a chunk
	name := path.Base(strings.TrimSuffix(info.uri, "/"))
	if !info.chunks || (info.size >= 0 && info.size <= int64(len(data))) || (info.size < 0 && len(data) < ChunkSize) {
		return &contents{uri: info.uri, name: name, mimeType: info.mimeType, data: data}, nil
	}

	// Otherwise the others are read as they're read
	info.name, info.first = name, data
	return info, nil
}

// Subscribe asks the server for the resource with the URI's updates
func (w *clientWorkspace) Subscribe(ctx context.Context, uri string) error {
	return w.client.Subscribe(ctx, uri)
}

// Unsubscribe stops asking the server for the resource with the URI's
// updates
func (w *clientWorkspace) Unsubscribe(ctx context.Context, uri string) error {
	return w.client.Unsubscribe(ctx, uri)
}

// WatchResources sends the server's resources which are updated, which were
// subscribed to, with Subscribe, and, without a URI, that its
// resources changed, when it says its list did, until the context is
// cancelled, when it returns nil
func (w *clientWorkspace) WatchResources(ctx context.Context, ch chan<- schema.Change) error {
	if ch == nil {
		return llm.ErrBadParameter.With("missing channel")
	}
	uris := make(chan string)
	done := make(chan error, 1)
	go func() { done <- w.client.WatchResources(ctx, uris) }()
	for {
		select {
		case uri := <-uris:
			select {
			case ch <- schema.Change{Action: eventschema.ChangeUpdate, URI: uri}:
			case <-ctx.Done():
			}
		case err := <-done:
			return err
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// notFound returns MCP's resource not found error as one which is
// ErrNotFound, as well as itself, as a Reader's is, or the error, as it is
func notFound(err error, uri string) error {
	var wire *jsonrpc.Error
	if errors.As(err, &wire) && wire.Code == sdkmcp.CodeResourceNotFound {
		return llm.ErrNotFound.With(uri)
	}
	return err
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
