package opt

///////////////////////////////////////////////////////////////////////////////
// TYPES

// A generic option type, which can set options on an agent or session
type Opt func(*Opts) error

// set of options
type Opts struct {
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// ApplyOpts returns a structure of options
func ApplyOpts(opts ...Opt) (*Opts, error) {
	o := new(Opts)
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}
