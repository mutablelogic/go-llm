package schema

import (
	"net/url"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ConnectorMeta are the fields of a connector which can be set
type ConnectorMeta struct {
	URL string `json:"url" help:"The URL of the connector's MCP server, of its streamable HTTP, or SSE, transport" example:"https://mcp.example.com/mcp"`
}

// Connector is a remote MCP server, as it's connected to: its name, its
// fields which can be set, and its serial, which changes each time its
// definition does. It's what's needed to create a client of its server, and
// what a store of connectors holds.
type Connector struct {
	Name string `json:"name" help:"Connector name, which is an identifier, starting with a letter, followed by letters, numbers, underscores or hyphens" example:"github"`
	ConnectorMeta
	Serial uint64 `json:"serial,omitempty" readonly:"" help:"Revision of the connector's definition, which changes each time it's changed" example:"1"`
}

// ConnectorList is a list of connectors, in order of their names
type ConnectorList struct {
	Count uint        `json:"count" help:"The number of connectors"`
	Body  []Connector `json:"body,omitempty" help:"The connectors"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Validate returns ErrBadParameter when the connector's name isn't an
// identifier, or its fields aren't valid
func (c Connector) Validate() error {
	if !types.IsIdentifier(c.Name) {
		return llm.ErrBadParameter.Withf("connector name %q isn't an identifier", c.Name)
	}
	return c.ConnectorMeta.Validate()
}

// Validate returns ErrBadParameter when the URL isn't an http, or https,
// URL with a host, as an MCP server which runs as a command, over stdio,
// can't be connected to
func (m ConnectorMeta) Validate() error {
	u, err := url.Parse(strings.TrimSpace(m.URL))
	if err != nil {
		return llm.ErrBadParameter.Withf("url %q: %v", m.URL, err)
	} else if u.Scheme != "http" && u.Scheme != "https" {
		return llm.ErrBadParameter.Withf("url %q isn't http, or https", m.URL)
	} else if u.Host == "" {
		return llm.ErrBadParameter.Withf("url %q has no host", m.URL)
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (c Connector) String() string {
	return types.Stringify(c)
}

func (m ConnectorMeta) String() string {
	return types.Stringify(m)
}

func (l ConnectorList) String() string {
	return types.Stringify(l)
}
