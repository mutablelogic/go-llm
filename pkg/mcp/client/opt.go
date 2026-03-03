package client

import (
	"io"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ClientOpt is a functional option for configuring a Client.
type ClientOpt func(*Client)

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// OptTrace enables HTTP request/response logging to w for all HTTP calls
// (both MCP transport and OAuth). Pass os.Stderr for terminal debug output.
func OptTrace(w io.Writer) ClientOpt {
	return func(c *Client) { c.trace = w }
}
