package schema

import "time"

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Capability represents a named capability advertised by an MCP server.
type Capability string

// ConnectorMeta holds the user-configurable settings for a registered MCP server.
type ConnectorMeta struct {
	// Enabled controls whether the server is active. Disabled servers are
	// retained in the registry but not connected on startup.
	Enabled bool `json:"enabled"`

	// Namespace is an optional prefix used to disambiguate tools from this
	// connector when multiple connectors expose tools with the same name.
	Namespace string `json:"namespace,omitempty"`
}

// Connector combines persisted metadata with runtime state for an MCP server.
type Connector struct {
	ConnectorMeta

	// URL is the MCP server endpoint (used as the primary key).
	URL string `json:"url"`

	// CreatedAt is the time the connector was first registered.
	CreatedAt time.Time `json:"created_at"`

	// ConnectedAt is the time of the most recent successful connection.
	// Zero value indicates the server has not yet connected successfully.
	ConnectedAt time.Time `json:"connected_at,omitempty"`

	// Name is the programmatic identifier reported by the server (e.g. "github-mcp-server").
	Name string `json:"name,omitempty"`

	// Title is a human-readable display name for the server.
	Title string `json:"title,omitempty"`

	// Description is an optional free-text description of the server.
	Description string `json:"description,omitempty"`

	// Version is the server-reported version string.
	Version string `json:"version,omitempty"`

	// Capabilities is the set of capabilities declared by the server at
	// initialization time. See the Capability* constants.
	Capabilities []Capability `json:"capabilities,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

const (
	// CapabilityTools indicates the server advertises callable tools.
	CapabilityTools Capability = "tools"

	// CapabilityResources indicates the server can serve resources.
	CapabilityResources Capability = "resources"

	// CapabilityPrompts indicates the server provides reusable prompt templates.
	CapabilityPrompts Capability = "prompts"

	// CapabilityLogging indicates the server emits log notifications.
	CapabilityLogging Capability = "logging"
)
