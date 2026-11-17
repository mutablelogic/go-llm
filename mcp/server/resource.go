package server

import (
	"context"
	"errors"
	"strings"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// capabilities returns what the server says it has: tools, which change,
// even while it has none, so a client which connects while it has none
// lists them once it's told they changed, logging, as the SDK's servers
// have, and, with a workspace, resources, and the experimental capability
// which says it's a workspace, which lists its resources by folder
func (s *Server) capabilities() *sdkmcp.ServerCapabilities {
	caps := &sdkmcp.ServerCapabilities{
		Logging: &sdkmcp.LoggingCapabilities{},
		Tools:   &sdkmcp.ToolCapabilities{ListChanged: true},
	}
	if s.workspace != nil {
		caps.Resources = &sdkmcp.ResourceCapabilities{}
		caps.Experimental = map[string]any{schema.Capability: map[string]any{}}
	}
	return caps
}

// listResources returns a page of the workspace's folders and resources, by
// the folder, and cursor, of the request
func (s *Server) listResources(ctx context.Context, req sdkmcp.Request) (sdkmcp.Result, error) {
	var params *sdkmcp.ListResourcesParams
	if r, ok := req.(*sdkmcp.ListResourcesRequest); ok {
		params = r.Params
	}
	list, err := s.workspace.List(ctx, schema.ListRequestOf(params))
	if err != nil {
		return nil, err
	}
	result := &sdkmcp.ListResourcesResult{Resources: make([]*sdkmcp.Resource, 0, len(list.Body)), NextCursor: list.Next}
	for _, r := range list.Body {
		result.Resources = append(result.Resources, r.MCP())
	}
	return result, nil
}

// readResource returns the workspace's resource, or artwork, with the URI
// of the request, as text, when it's text, or its data. Returns MCP's
// resource not found error, when the workspace has none, or doesn't read
// its resources.
func (s *Server) readResource(ctx context.Context, req sdkmcp.Request) (sdkmcp.Result, error) {
	r, ok := req.(*sdkmcp.ReadResourceRequest)
	if !ok || r.Params == nil || r.Params.URI == "" {
		return nil, llm.ErrBadParameter.With("missing uri")
	}
	uri := r.Params.URI
	reader, ok := s.workspace.(workspace.Reader)
	if !ok {
		return nil, sdkmcp.ResourceNotFoundError(uri)
	}
	resource, err := reader.Read(ctx, uri)
	if errors.Is(err, llm.ErrNotFound) {
		return nil, sdkmcp.ResourceNotFoundError(uri)
	} else if err != nil {
		return nil, err
	}
	data, err := llm.ReadAll(ctx, resource)
	if err != nil {
		return nil, err
	}
	contents := &sdkmcp.ResourceContents{URI: uri, MIMEType: resource.Type()}
	if mimeType := resource.Type(); strings.HasPrefix(mimeType, "text/") || mimeType == types.ContentTypeJSON {
		contents.Text = string(data)
	} else {
		contents.Blob = data
	}
	return &sdkmcp.ReadResourceResult{Contents: []*sdkmcp.ResourceContents{contents}}, nil
}
