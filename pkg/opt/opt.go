package opt

import (
	"fmt"
	"net/url"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// A generic option type, which can set options on an agent or session
type Opt func(*opts) error

// set of options
type opts struct {
	url.Values
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Apply returns a structure of applied options
func Apply(o ...Opt) (*opts, error) {
	opts := &opts{Values: make(url.Values)}
	for _, opt := range o {
		if err := opt(opts); err != nil {
			return nil, err
		}
	}
	return opts, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (o *opts) Query(keys ...string) url.Values {
	query := make(url.Values)
	for _, key := range keys {
		if value, ok := o.Values[key]; ok {
			query[key] = value
		}
	}
	return query
}

////////////////////////////////////////////////////////////////////////////////
// OPTIONS

func WithString(key string, value ...string) Opt {
	return func(o *opts) error {
		for _, v := range value {
			o.Values.Add(key, v)
		}
		return nil
	}
}

func WithUint(key string, value ...uint) Opt {
	return func(o *opts) error {
		for _, v := range value {
			o.Values.Add(key, fmt.Sprintf("%d", v))
		}
		return nil
	}
}
