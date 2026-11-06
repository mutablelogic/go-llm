package toolkit

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Result is a tool's result with resources, such as images, or audio, which
// it returns, with a value, or without one, as MCP's tools return content
// and structured content. A tool can also return a resource, or resources,
// on their own.
type Result struct {
	// Value is what the tool returns with the resources, which is encoded as
	// JSON, or nil, for none
	Value any

	// Resources are the resources the tool returns
	Resources []llm.Resource
}

// StoreFunc keeps a resource which a tool returned, such as in the session
// the tool runs in, and returns a link to it, which the tool's result has,
// rather than its content
type StoreFunc func(context.Context, llm.Resource) (schema.ResourceLink, error)

// storeKey is the key of the store function in a context
type storeKey struct{}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// WithStore returns the context, with the function which keeps the
// resources which the tools run in it return, so their results have links
// to them, rather than their content. Without one, a resource's content is
// in the result, encoded as base64.
func WithStore(ctx context.Context, fn StoreFunc) context.Context {
	return context.WithValue(ctx, storeKey{}, fn)
}

// storeFrom returns the function which keeps the resources which tools
// return, or nil, when there's none
func storeFrom(ctx context.Context) StoreFunc {
	fn, _ := ctx.Value(storeKey{}).(StoreFunc)
	return fn
}
