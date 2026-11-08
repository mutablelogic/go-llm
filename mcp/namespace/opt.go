package namespace

import (
	"time"

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

	// How long one of its tools has, or zero for the toolkit's default
	timeout time.Duration
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
	o.timeout = 0
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

// WithTimeout is how long one of the server's tools has, as it's called,
// rather than the toolkit's default, which it has when it's zero
func WithTimeout(timeout time.Duration) Opt {
	return func(o *opt) error {
		if timeout < 0 {
			return llm.ErrBadParameter.Withf("timeout %v is negative", timeout)
		}
		o.timeout = timeout
		return nil
	}
}
