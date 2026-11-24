// Package registry is the registry of webhooks: they're registered, for a
// session, with the protocol of the service which calls them, and listed,
// changed, and deleted, and their calls are received, and kept, in its store
package registry

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
	store "github.com/mutablelogic/go-llm/webhook/store"
	broadcaster "github.com/mutablelogic/go-server/broadcaster"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Registry is the registry of webhooks, of its store
type Registry struct {
	store       store.Store
	broadcaster *broadcaster.Broadcaster
	tracer      trace.Tracer
	maxBody     int64
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the registry of the store, with the options, such as its
// tracer
func New(store store.Store, opts ...Opt) (*Registry, error) {
	var o opt
	if store == nil {
		return nil, llm.ErrBadParameter.With("missing webhook store")
	} else if err := o.apply(opts); err != nil {
		return nil, err
	}
	if o.maxBody == 0 {
		o.maxBody = DefaultMaxBody
	}
	return &Registry{store: store, broadcaster: o.broadcaster, tracer: o.tracer, maxBody: o.maxBody}, nil
}
