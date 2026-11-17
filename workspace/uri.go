package workspace

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// resource is a workspace's resource, with the workspace's URI of it,
// rather than its own
type resource struct {
	llm.Resource
	uri string
}

var _ llm.Sized = (*resource)(nil)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// withURI returns the resource, with the URI
func withURI(r llm.Resource, uri string) llm.Resource {
	if r == nil {
		return nil
	}
	return &resource{Resource: r, uri: uri}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// URI returns the workspace's URI of the resource
func (r *resource) URI() string { return r.uri }

// ContentLength returns the length of the resource's content, or -1, when
// it isn't known without reading it
func (r *resource) ContentLength() int64 {
	if n, ok := llm.ContentLength(r.Resource); ok {
		return n
	}
	return -1
}
