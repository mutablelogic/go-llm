package workspace

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option of a registry
type Opt func(*opt) error

type opt struct {
	// The broadcaster the registry's changes are published to, or nil
	broadcaster *broadcaster.Broadcaster
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
