package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools returns tools matching the given request parameters.
func (c *Client) ListTools(ctx context.Context, req schema.ToolListRequest) (*schema.ToolList, error) {
	var response schema.ToolList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("tool"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetTool returns metadata for a specific tool by name.
func (c *Client) GetTool(ctx context.Context, name string) (*schema.ToolMeta, error) {
	if name == "" {
		return nil, fmt.Errorf("tool name cannot be empty")
	}

	var response schema.ToolMeta
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("tool", name)); err != nil {
		return nil, err
	}

	return &response, nil
}

// CallTool executes a tool and returns the raw result as an llm.Resource.
func (c *Client) CallTool(ctx context.Context, name string, req schema.CallToolRequest) (llm.Resource, error) {
	if name == "" {
		return nil, fmt.Errorf("tool name cannot be empty")
	}

	// Create JSON request
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	// Get the resource
	resource := new(resource)
	err = c.DoWithContext(ctx, payload, resource, client.OptPath("tool", name))
	if err != nil {
		return nil, err
	}
	if resource.empty() {
		return nil, nil
	}

	// Return the resource
	return resource, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

type resource struct {
	uri         string
	name        string
	description string
	contentType string
	data        []byte
}

var _ client.Unmarshaler = (*resource)(nil)
var _ llm.Resource = (*resource)(nil)

func (r *resource) Unmarshal(header http.Header, body io.Reader) error {
	r.uri = header.Get(types.ContentPathHeader)
	r.name = header.Get(types.ContentNameHeader)
	r.description = header.Get(types.ContentDescriptionHeader)
	r.contentType = types.ContentTypeBinary
	if value := header.Get(types.ContentTypeHeader); value != "" {
		if parsed, err := types.ParseContentType(value); err == nil {
			r.contentType = parsed
		} else {
			r.contentType = value
		}
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	r.data = data
	return nil
}

func (r *resource) URI() string {
	return r.uri
}

func (r *resource) Name() string {
	return r.name
}

func (r *resource) Description() string {
	return r.description
}
func (r *resource) Type() string {
	return r.contentType
}

func (r *resource) Read(context.Context) ([]byte, error) {
	return r.data, nil
}

func (r *resource) empty() bool {
	return len(r.data) == 0
}
