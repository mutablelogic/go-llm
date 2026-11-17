package workspace

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-server/broadcaster"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option of a registry
type Opt func(*opt) error

type opt struct {
	// The broadcaster the registry's changes are published to, or nil
	broadcaster *broadcaster.Broadcaster

	// The tracer of the registry's operations, or nil
	tracer trace.Tracer
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (o *opt) apply(opts []Opt) error {
	for _, fn := range opts {
		if err := fn(o); err != nil {
			return err
		}
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithTracer traces the registry's operations, such as listing, and reading,
// its workspaces' resources
func WithTracer(tracer trace.Tracer) Opt {
	return func(o *opt) error {
		o.tracer = tracer
		return nil
	}
}

// WithBroadcaster publishes the registry's changes: a workspace which is
// added, or removed, in WorkspaceTable, whose row is its name, and the
// folders and resources of a Watcher which change, in ResourceTable, whose
// row is the workspace's name, and whose data is the change's URI
func WithBroadcaster(b *broadcaster.Broadcaster) Opt {
	return func(o *opt) error {
		if b == nil {
			return llm.ErrBadParameter.With("missing broadcaster")
		}
		o.broadcaster = b
		return nil
	}
}
