package client

import (
	"strings"

	// Packages
	goclient "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option of a client
type Opt func(*opt) error

type opt struct {
	// The name, and version, of the client, which the server is told, as it
	// connects
	name, version string

	// The options of go-client, which apply to the client's requests
	clientopts []goclient.ClientOpt
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
	o.name = ""
	o.version = ""
	o.clientopts = nil
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithName is the name, and version, of the client, which the server is
// told, as it connects, which a client needs. The version can be empty.
func WithName(name, version string) Opt {
	return func(o *opt) error {
		if name = strings.TrimSpace(name); name == "" {
			return llm.ErrBadParameter.With("name is required")
		}
		o.name, o.version = name, strings.TrimSpace(version)
		return nil
	}
}

// WithClientOpts are options of go-client, such as goclient.OptHeader,
// goclient.OptUserAgent, goclient.OptReqToken, or goclient.OptTrace, which
// apply to the client's requests, and so to those of the MCP SDK. Its
// timeout, with goclient.OptTimeout, applies to its own requests, but not
// to an MCP session's, whose streams last as long as the session, and are
// cancelled with its context.
func WithClientOpts(opts ...goclient.ClientOpt) Opt {
	return func(o *opt) error {
		o.clientopts = append(o.clientopts, opts...)
		return nil
	}
}
