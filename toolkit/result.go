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

// ResolveFunc returns the resources with the URIs, such as
// resource:///<hash>, which a tool is given, in the session the tool runs
// in, or on their own, whose content is read from where it's kept, once for
// each URI, and together within a limit, in the same order
type ResolveFunc func(ctx context.Context, uris ...string) ([]llm.Resource, error)

// resolveKey is the key of the resolve function in a context
type resolveKey struct{}

// WithResolver returns the context, with the function which returns the
// resources which the tools run in it are given, by their URIs
func WithResolver(ctx context.Context, fn ResolveFunc) context.Context {
	return context.WithValue(ctx, resolveKey{}, fn)
}

// Resource returns the resource with the URI, such as resource:///<hash>,
// which the tool running in the context is given, with the context's
// resolve function. Returns ErrNotImplemented when there's none, such as
// when a tool is called without a session, or the registry's resources,
// and the resolve function's errors, such as ErrNotFound.
func Resource(ctx context.Context, uri string) (llm.Resource, error) {
	resources, err := Resources(ctx, uri)
	if err != nil {
		return nil, err
	}
	return resources[0], nil
}

// Resources returns the resources with the URIs, which the tool running in
// the context is given, together, with the context's resolve function, so
// each is read once, and they're read within a limit, together, rather than
// each within it. Returns errors as Resource does.
func Resources(ctx context.Context, uris ...string) ([]llm.Resource, error) {
	fn, _ := ctx.Value(resolveKey{}).(ResolveFunc)
	if fn == nil {
		return nil, llm.ErrNotImplemented.With("resources can't be read here")
	}
	resources, err := fn(ctx, uris...)
	if err != nil {
		return nil, err
	} else if len(resources) != len(uris) {
		return nil, llm.ErrInternalServerError.Withf("%d resources resolved, for %d URIs", len(resources), len(uris))
	}
	return resources, nil
}
