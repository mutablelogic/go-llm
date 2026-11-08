package schema

import (
	"net/url"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	tui "github.com/mutablelogic/go-server/pkg/tui"
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
// definition does, which are what's needed to create a client of its
// server, and what a store of connectors holds, and its status: whether
// its server is connected, and which server it is, when it is, or why it
// couldn't be connected, or its session ended, which isn't stored, as it's
// the process's
type Connector struct {
	Name string `json:"name" help:"Connector name, which is an identifier, starting with a letter, followed by letters, numbers, underscores or hyphens" example:"github"`
	ConnectorMeta
	Serial uint64 `json:"serial,omitempty" readonly:"" help:"Revision of the connector's definition, which changes each time it's changed" example:"1"`

	// The status, which isn't stored
	Connected bool             `json:"connected,omitempty" readonly:"" help:"Whether the connector's server is connected"`
	Server    *ConnectorServer `json:"server,omitempty" readonly:"" help:"The connector's server, as it said, once it's connected"`
	Error     string           `json:"error,omitempty" readonly:"" help:"Why the connector's server couldn't be connected, or its session ended"`
}

// ConnectorServer is the name, and version, of a connector's server, as it
// said, once it's connected
type ConnectorServer struct {
	Name    string `json:"name" help:"The server's name" example:"github-mcp-server"`
	Version string `json:"version,omitempty" help:"The server's version" example:"1.0.0"`
}

// ConnectorEvent is the data of an event of a connector which was created,
// or updated: its new serial
type ConnectorEvent struct {
	Serial uint64 `json:"serial" help:"The connector's serial, once it changed"`
}

// ConnectorList is a list of connectors, in order of their names
type ConnectorList struct {
	Count uint         `json:"count" help:"The number of connectors"`
	Body  []*Connector `json:"body,omitempty" help:"The connectors"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Definition returns the connector without its status, which is what's
// stored
func (c Connector) Definition() Connector {
	c.Connected, c.Server, c.Error = false, nil, ""
	return c
}

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

///////////////////////////////////////////////////////////////////////////////
// TABLE

func (c *Connector) Header() []string {
	return []string{"Name", "Status", "Server", "URL"}
}

func (c *Connector) Width(col int) int {
	return 0
}

func (c *Connector) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return c.Name
	case 1:
		// A connector whose server isn't connected shows why, when it knows
		switch {
		case c.Connected:
			return "connected"
		case c.Error != "":
			// Its first line, as an error can have one for each transport
			line, _, _ := strings.Cut(c.Error, "\n")
			return line
		default:
			return "not connected"
		}
	case 2:
		if c.Server == nil {
			return ""
		} else if c.Server.Version != "" {
			return c.Server.Name + " " + c.Server.Version
		}
		return c.Server.Name
	case 3:
		return c.URL
	default:
		return ""
	}
}
