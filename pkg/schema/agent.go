package schema

import (
	"context"
	"time"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// AgentMeta describes the definition of an agent, including which model
// and provider to use and the schemas that govern its input and output.
type AgentMeta struct {
	GeneratorMeta `yaml:",inline"`
	Name          string     `json:"name" yaml:"name" help:"Unique agent name"`
	Title         string     `json:"title,omitempty" yaml:"title" help:"Human-readable title" optional:""`
	Description   string     `json:"description,omitempty" yaml:"description" help:"Agent description" optional:""`
	Template      string     `json:"template,omitempty" yaml:"-" help:"Go template for the user message" optional:""`
	Input         JSONSchema `json:"input,omitempty" yaml:"input" help:"JSON schema for agent input" optional:""`
	Tools         []string   `json:"tools,omitzero" yaml:"tools" help:"Tool names the agent is allowed to use" optional:""`
}

// Agent is a versioned, stored agent definition.
type Agent struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
	Version uint      `json:"version"`
	AgentMeta
}

// AgentStore is the interface for agent storage backends.
type AgentStore interface {
	// CreateAgent creates a new agent from the given metadata,
	// returning the agent with a unique ID and version 1.
	CreateAgent(ctx context.Context, meta AgentMeta) (*Agent, error)

	// GetAgent retrieves an existing agent by ID or name.
	// Returns an error if the agent does not exist.
	GetAgent(ctx context.Context, id string) (*Agent, error)

	// ListAgents returns agents matching the request, with pagination support.
	// Returns offset, limit and total count in the response.
	ListAgents(ctx context.Context, req ListAgentRequest) (*ListAgentResponse, error)

	// DeleteAgent removes an agent by ID or name. When a name is provided,
	// all versions of the agent are deleted. Returns an error if no matching
	// agent exists.
	DeleteAgent(ctx context.Context, id string) error

	// UpdateAgent applies non-zero fields from the given metadata to an existing
	// agent and increments the version. Returns the updated agent.
	UpdateAgent(ctx context.Context, id string, meta AgentMeta) (*Agent, error)
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (a AgentMeta) String() string {
	return types.Stringify(a)
}

func (a Agent) String() string {
	return types.Stringify(a)
}
