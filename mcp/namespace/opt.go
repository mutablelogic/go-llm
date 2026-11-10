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
	// How long one of its tools has, or zero for the toolkit's default
	timeout time.Duration

	// How its server's prompts are run, as agents, or nil, when they aren't
	run RunFunc
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
	o.timeout = 0
	o.run = nil
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithAgents makes the server's prompts agents, which are tools, beside its
// tools, run with the function, as the registry runs its agents. Without it,
// they aren't.
func WithAgents(run RunFunc) Opt {
	return func(o *opt) error {
		o.run = run
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
