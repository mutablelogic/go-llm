package schema

import (
	"time"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ConnectorCapability represents a named capability advertised by an MCP server.
type ConnectorCapability string

// ConnectorState carries the server-reported runtime state of a connector.
// All fields are pointers so callers can update a subset without overwriting
// fields they did not observe.
type ConnectorState struct {
	// ConnectedAt is the time of the most recent successful connection.
	ConnectedAt *time.Time `json:"connected_at,omitempty"`

	// Name is the programmatic identifier reported by the server.
	Name *string `json:"name,omitempty"`

	// Title is a human-readable display name reported by the server.
	Title *string `json:"title,omitempty"`

	// Description is the server-reported description.
	Description *string `json:"description,omitempty"`

	// Version is the server-reported version string.
	Version *string `json:"version,omitempty"`

	// Capabilities is the set of capabilities declared at initialization.
	Capabilities []ConnectorCapability `json:"capabilities,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// CapabilityTools indicates the server advertises callable tools.
	CapabilityTools ConnectorCapability = "tools"

	// CapabilityResources indicates the server can serve resources.
	CapabilityResources ConnectorCapability = "resources"

	// CapabilityPrompts indicates the server provides reusable prompt templates.
	CapabilityPrompts ConnectorCapability = "prompts"

	// CapabilityLogging indicates the server emits log notifications.
	CapabilityLogging ConnectorCapability = "logging"

	// CapabilityCompletions indicates the server supports argument/prompt completion.
	CapabilityCompletions ConnectorCapability = "completions"
)

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (c ConnectorState) String() string {
	return types.Stringify(c)
}
