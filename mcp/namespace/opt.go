package namespace

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option of a namespace
type Opt func(*opt) error

type opt struct {
	// Whether one of the server's tools is included, by its name
	match func(string) bool
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (o *opt) apply(opts []Opt) error {
	o.defaults()

	// Apply options
	for _, fn := range opts {
		if err := fn(o); err != nil {
			return err
		}
	}

	// Return success
	return nil
}

func (o *opt) defaults() {
	o.match = func(string) bool { return true }
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithMatch is the function which says whether one of the server's tools is
// included, by its name, such as a connector's schema.ConnectorMeta.Matcher,
// rather than every one
func WithMatch(fn func(string) bool) Opt {
	return func(o *opt) error {
		if fn == nil {
			return llm.ErrBadParameter.With("missing match")
		}
		o.match = fn
		return nil
	}
}
