package client

import (
	"strings"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
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
	clientopts []client.ClientOpt

	// Called when the client is connected, or disconnected, or nil
	change ChangeFunc

	// Called when the server says its tools changed, or nil
	toolsChanged func()
}

// ChangeFunc is called, without a lock, when the client is connected, with
// the server, as it said, or disconnected, with nil, and why, when it
// wasn't disconnected, such as when the server closed the session
type ChangeFunc func(server *sdkmcp.Implementation, err error)

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
	o.name = schema.DefaultName
	o.version = schema.DefaultVersion
	o.clientopts = nil
	o.change = nil
	o.toolsChanged = nil
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithName is the name, and version, of the client, which the server is
// told, as it connects, rather than schema.DefaultName, and
// schema.DefaultVersion. The version can be empty.
func WithName(name, version string) Opt {
	return func(o *opt) error {
		if name = strings.TrimSpace(name); name == "" {
			return llm.ErrBadParameter.With("name is required")
		}
		o.name, o.version = name, strings.TrimSpace(version)
		return nil
	}
}

// WithChange is called when the client is connected, or disconnected,
// without a lock, which it isn't when a connection fails, or it's
// disconnected when it isn't connected
func WithChange(fn ChangeFunc) Opt {
	return func(o *opt) error {
		o.change = fn
		return nil
	}
}

// WithToolsChanged is called when the server says its tools changed, so
// they're listed again, without a lock
func WithToolsChanged(fn func()) Opt {
	return func(o *opt) error {
		o.toolsChanged = fn
		return nil
	}
}

// WithClientOpts are options of go-client, such as client.OptHeader,
// client.OptUserAgent, client.OptReqToken, or client.OptTrace, which
// apply to the client's requests, and so to those of the MCP SDK. Its
// timeout, with client.OptTimeout, applies to its own requests, but not
// to an MCP session's, whose streams last as long as the session, and are
// cancelled with its context.
func WithClientOpts(opts ...client.ClientOpt) Opt {
	return func(o *opt) error {
		o.clientopts = append(o.clientopts, opts...)
		return nil
	}
}
