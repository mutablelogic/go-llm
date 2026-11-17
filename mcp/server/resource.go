package server

import (
	"context"
	"errors"
	"strings"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
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
	if _, ok := s.workspace.(workspace.Watcher); ok {
		caps.Resources.ListChanged, caps.Resources.Subscribe = true, true
	}
	return caps
}

// subscribe returns the handler of a client's subscription to a resource,
// which the SDK keeps, so it's told when it's updated, when the workspace
// changes, or nil, when it doesn't
func (s *Server) subscribe() func(context.Context, *sdkmcp.SubscribeRequest) error {
	if _, ok := s.workspace.(workspace.Watcher); !ok {
		return nil
	}
	return func(context.Context, *sdkmcp.SubscribeRequest) error { return nil }
}

// unsubscribe returns the handler of a client's unsubscription, as
// subscribe does
func (s *Server) unsubscribe() func(context.Context, *sdkmcp.UnsubscribeRequest) error {
	if _, ok := s.workspace.(workspace.Watcher); !ok {
		return nil
	}
	return func(context.Context, *sdkmcp.UnsubscribeRequest) error { return nil }
}

// changed tells the clients of the workspace's change: a resource which is
// updated, to those which subscribed to it, and one which is created, or
// deleted, or a change without a URI, that the list changed, to all of them
func (s *Server) changed(ctx context.Context, change schema.Change) {
	if change.URI != "" && change.Action == eventschema.EventUpdate {
		if err := s.server.ResourceUpdated(ctx, &sdkmcp.ResourceUpdatedNotificationParams{URI: change.URI}); err != nil {
			s.logger.Warn("resource update isn't sent", "uri", change.URI, "error", err.Error())
		}
		return
	}
	s.listChanged()
}

// listChanged tells the clients the list of resources changed. The SDK
// only says so when a resource is added to, or removed from, its own list,
// which a workspace's aren't in, so a placeholder is added, and removed,
// which it says once, as it waits a moment for more changes.
func (s *Server) listChanged() {
	s.server.AddResource(&sdkmcp.Resource{URI: listChangedURI, Name: "list-changed"}, func(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		return nil, sdkmcp.ResourceNotFoundError(listChangedURI)
	})
	s.server.RemoveResources(listChangedURI)
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// listChangedURI is the URI of the placeholder which says the list of
// resources changed, which is never listed, or read
const listChangedURI = "go-llm-workspace:///list-changed"

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
	if errors.Is(err, llm.ErrNotFound) {
		return nil, sdkmcp.ResourceNotFoundError(uri)
	} else if err != nil {
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
