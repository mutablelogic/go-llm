package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	// Packages
	jsonrpc "github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
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
// changes, or nil, when it doesn't. A workspace which is a Subscriber, such
// as an MCP server's, is subscribed to, once, while any of the server's
// clients are.
func (s *Server) subscribe() func(context.Context, *sdkmcp.SubscribeRequest) error {
	if _, ok := s.workspace.(workspace.Watcher); !ok {
		return nil
	}
	return func(ctx context.Context, req *sdkmcp.SubscribeRequest) error {
		subscriber, ok := s.workspace.(workspace.Subscriber)
		if !ok || req.Params == nil {
			return nil
		}
		uri := req.Params.URI
		s.subscribing.Lock()
		defer s.subscribing.Unlock()
		if s.subscriptions[uri] == 0 {
			if err := subscriber.Subscribe(ctx, uri); err != nil {
				return err
			}
		}
		s.subscriptions[uri]++
		return nil
	}
}

// unsubscribe returns the handler of a client's unsubscription, as
// subscribe does, which unsubscribes from a Subscriber once none of the
// server's clients are subscribed
func (s *Server) unsubscribe() func(context.Context, *sdkmcp.UnsubscribeRequest) error {
	if _, ok := s.workspace.(workspace.Watcher); !ok {
		return nil
	}
	return func(ctx context.Context, req *sdkmcp.UnsubscribeRequest) error {
		subscriber, ok := s.workspace.(workspace.Subscriber)
		if !ok || req.Params == nil {
			return nil
		}
		uri := req.Params.URI
		s.subscribing.Lock()
		defer s.subscribing.Unlock()
		switch s.subscriptions[uri] {
		case 0:
			return nil
		case 1:
			delete(s.subscriptions, uri)
			return subscriber.Unsubscribe(ctx, uri)
		default:
			s.subscriptions[uri]--
			return nil
		}
	}
}

// changed tells the clients of the workspace's change: a resource which is
// updated, to those which subscribed to it, and one which is created, or
// deleted, or a change without a URI, that the list changed, to all of them
func (s *Server) changed(ctx context.Context, change schema.Change) {
	if change.URI != "" && change.Action == eventschema.ChangeUpdate {
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

// MaxChunk is the most bytes of a resource's content which are read at
// once, as a chunk, as its JSON-RPC frame, in base64, is at most the SDK's
// DefaultMaxLineLength
const MaxChunk = 8 * 1024 * 1024

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

	// A chunk of it, as the request's _meta asks, with the size of all of
	// it, so a large resource is read a chunk at a time, or all of it
	result := &sdkmcp.ReadResourceResult{}
	offset, length, chunked := schema.ReadRangeOf(r.Params)
	var data []byte
	if chunked {
		length = min(length, MaxChunk)
		if data, err = readChunk(ctx, resource, offset, length); err == nil {
			result.Meta = sdkmcp.Meta{schema.MetaOffset: offset}
			if size, ok := llm.ContentLength(resource); ok {
				result.Meta[schema.MetaSize] = size
			}
		}
	} else if size, ok := llm.ContentLength(resource); ok && size > MaxChunk {
		return nil, tooLarge(uri, size)
	} else if data, err = llm.ReadAll(ctx, resource); err == nil && int64(len(data)) > MaxChunk {
		return nil, tooLarge(uri, int64(len(data)))
	}
	if errors.Is(err, llm.ErrNotFound) {
		return nil, sdkmcp.ResourceNotFoundError(uri)
	} else if err != nil {
		return nil, err
	}

	// A chunk is its data, as it can end in the middle of a character, and
	// all of it is text, when it's text
	contents := &sdkmcp.ResourceContents{URI: uri, MIMEType: resource.Type()}
	if mimeType := resource.Type(); !chunked && (strings.HasPrefix(mimeType, "text/") || mimeType == types.ContentTypeJSON) {
		contents.Text = string(data)
	} else {
		contents.Blob = data
	}
	result.Contents = []*sdkmcp.ResourceContents{contents}
	return result, nil
}

// tooLarge returns the error of a read of all of a resource which is larger
// than a chunk, whose JSON-RPC frame would be larger than the SDK reads,
// which closes the session, so it's read a chunk at a time
func tooLarge(uri string, size int64) error {
	return &jsonrpc.Error{
		Code:    jsonrpc.CodeInvalidParams,
		Message: fmt.Sprintf("resource %q is %d bytes, which is more than %d bytes, so read it in chunks, with the %q and %q of the request's _meta", uri, size, MaxChunk, schema.MetaOffset, schema.MetaLength),
	}
}

// readChunk reads the chunk of the resource's content from the offset, of
// at most the length, seeking to it, when its reader can, or reading past
// it, when it can't. The chunk is empty, beyond the end of its content.
func readChunk(ctx context.Context, resource llm.Resource, offset, length int64) ([]byte, error) {
	read, err := resource.Read(ctx)
	if err != nil {
		return nil, err
	}
	var data []byte
	err = read(ctx, func(r io.Reader) error {
		if seeker, ok := r.(io.Seeker); ok {
			if _, err := seeker.Seek(offset, io.SeekStart); err != nil {
				return err
			}
		} else if _, err := io.CopyN(io.Discard, r, offset); err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		var err error
		data, err = io.ReadAll(io.LimitReader(r, length))
		return err
	})
	if data == nil {
		data = []byte{}
	}
	return data, err
}
