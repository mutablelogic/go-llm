package schema

import (
	"context"
	"fmt"
	"net/url"
	"time"

	// Packages
	pg "github.com/mutablelogic/go-pg"
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

// AgentListRequest represents a request to list externally exposed agents,
// which are backed by toolkit prompts.
type AgentListRequest struct {
	pg.OffsetLimit

	// Namespace restricts results to a single namespace.
	// Use BuiltinNamespace for locally-implemented agents, a connector namespace
	// for remote agents, or leave empty to include all namespaces.
	Namespace string `json:"namespace,omitempty" help:"Restrict results to a single namespace" example:"builtin"`

	// Name restricts results to agents whose names appear in this list.
	// An empty slice means no name filter.
	Name []string `json:"name,omitempty" help:"Restrict results to the listed agent names" example:"[\"builtin.summarize\",\"research.translate\"]"`
}

// AgentList represents a response containing a list of externally exposed agents.
type AgentList struct {
	AgentListRequest
	Count uint         `json:"count" help:"Total number of matching agents" example:"2"`
	Body  []*AgentMeta `json:"body,omitzero" help:"Agent metadata returned for the current page" example:"[{\"name\":\"builtin.summarize\",\"title\":\"Summarize\"}]"`
}

// CallAgentRequest represents a request to call an agent directly.
type CallAgentRequest struct {
	CallToolRequest
	Attachments []interface {
		URI() string
		Name() string
		Description() string
		Type() string
		Read(context.Context) ([]byte, error)
	} `json:"attachments,omitempty" help:"Additional resources attached to the agent call" optional:""`
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

func (r AgentListRequest) String() string {
	return types.Stringify(r)
}

func (r AgentListRequest) Query() url.Values {
	values := url.Values{}
	if r.Offset > 0 {
		values.Set("offset", fmt.Sprintf("%d", r.Offset))
	}
	if r.Limit != nil {
		values.Set("limit", fmt.Sprintf("%d", types.Value(r.Limit)))
	}
	if r.Namespace != "" {
		values.Set("namespace", r.Namespace)
	}
	for _, name := range r.Name {
		if name != "" {
			values.Add("name", name)
		}
	}
	return values
}

func (r AgentList) String() string {
	return types.Stringify(r)
}

func (r CallAgentRequest) String() string {
	return types.Stringify(r)
}

func (a Agent) String() string {
	return types.Stringify(a)
}
