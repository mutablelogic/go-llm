package client

import (
	"strings"

	// Packages
	goclient "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option of a client, which New applies
type Opt func(*Client) error

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithClientOpts are options of go-client, such as goclient.OptHeader,
// goclient.OptUserAgent, goclient.OptReqToken, or goclient.OptTrace, which
// apply to the client's requests, and so to those of the MCP SDK. Its
// timeout, with goclient.OptTimeout, applies to its own requests, but not
// to an MCP session's, whose streams last as long as the session, and are
// cancelled with its context.
func WithClientOpts(opts ...goclient.ClientOpt) Opt {
	return func(c *Client) error {
		c.opts = append(c.opts, opts...)
		return nil
	}
}

// WithName is the name, and version, of the client, which the server is
// told, as it connects, which a client needs. The version can be empty.
func WithName(name, version string) Opt {
	return func(c *Client) error {
		if name = strings.TrimSpace(name); name == "" {
			return llm.ErrBadParameter.With("name is required")
		}
		c.implementation.Name, c.implementation.Version = name, strings.TrimSpace(version)
		return nil
	}
}
