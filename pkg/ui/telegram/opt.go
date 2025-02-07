package telegram

import "context"

///////////////////////////////////////////////////////////////////////////////
// TYPES

// A generic option type, which can set options on an agent or session
type Opt func(*opts) error

// set of options
type opts struct {
	token    string
	callback CallbackFunc
	debug    bool
}

type CallbackFunc func(context.Context, Message) error

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// applyOpts returns a structure of options
func applyOpts(token string, opt ...Opt) (*opts, error) {
	o := new(opts)
	o.token = token
	for _, opt := range opt {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func WithCallback(fn CallbackFunc) Opt {
	return func(o *opts) error {
		o.callback = fn
		return nil
	}
}

func WithDebug(v bool) Opt {
	return func(o *opts) error {
		o.debug = v
		return nil
	}
}
