package server

import (
	"context"
	"fmt"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Resource is an alias for llm.Resource.
type Resource = llm.Resource

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// AddResources registers one or more Resource values on the server. Each
// resource is reachable by its URI. Returns an error for the first resource
// whose URI is invalid; resources registered before the error are still active.
// AddResources panics if the URI is not absolute (has an empty scheme) — this
// mirrors the SDK's own behaviour.
func (s *Server) AddResources(resources ...Resource) error {
	for _, r := range resources {
		if r.URI() == "" {
			return fmt.Errorf("resource name %q: URI is required", r.Name())
		}
		s.server.AddResource(sdkResourceFromResource(r), sdkResourceHandlerFromResource(r))
	}
	return nil
}

// RemoveResources removes the resources with the given URIs from the server.
// Unknown URIs are silently ignored.
func (s *Server) RemoveResources(uris ...string) {
	s.server.RemoveResources(uris...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func sdkResourceFromResource(r Resource) *sdkmcp.Resource {
	return &sdkmcp.Resource{
		URI:         r.URI(),
		Name:        r.Name(),
		Description: r.Description(),
		MIMEType:    r.MIMEType(),
	}
}

func sdkResourceHandlerFromResource(r Resource) sdkmcp.ResourceHandler {
	return func(ctx context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		data, err := r.Read(ctx)
		if err != nil {
			return nil, err
		}
		contents := &sdkmcp.ResourceContents{
			URI:      req.Params.URI,
			MIMEType: r.MIMEType(),
		}
		// Store as text if the content is valid UTF-8, otherwise as blob.
		if isValidUTF8(data) {
			contents.Text = string(data)
		} else {
			contents.Blob = data
		}
		return &sdkmcp.ReadResourceResult{
			Contents: []*sdkmcp.ResourceContents{contents},
		}, nil
	}
}

// isValidUTF8 reports whether data is valid UTF-8 and contains no null bytes.
func isValidUTF8(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	// strings.ToValidUTF8 would mutate; use a fast path via range over string.
	for _, r := range string(data) {
		if r == '\uFFFD' {
			return false
		}
	}
	return true
}
