package client

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// LogMessage is a log notification sent by the server via the MCP
// notifications/message notification.
type LogMessage struct {
	// Level is the severity (debug, info, notice, warning, error, critical,
	// alert, emergency).
	Level sdkmcp.LoggingLevel

	// Logger is the optional name of the logger that emitted the message.
	Logger string

	// Data is the payload — typically a string, but any JSON-serialisable value
	// is permitted by the spec.
	Data any
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Messages returns a snapshot of all log messages received from the server
// since the last successful Connect call, oldest first.
func (c *Client) Messages() []LogMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]LogMessage, len(c.messages))
	copy(out, c.messages)
	return out
}
