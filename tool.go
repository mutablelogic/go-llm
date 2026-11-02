package llm

import (
	"context"
	"encoding/json"
	"regexp"

	// Packages
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Tool is an interface for a callable tool with a name, description,
// input schema, optional output schema, and metadata hints.
type Tool interface {
	// Return the name of the tool
	Name() string

	// Return the description of the tool
	Description() string

	// Return the JSON schema for the tool input parameters.
	InputSchema() *jsonschema.Schema

	// Return the JSON schema for the tool output, or nil if unspecified.
	OutputSchema() *jsonschema.Schema

	// Return optional metadata / hints about the tool.
	Meta() ToolMeta

	// Run the tool with the given input as JSON (may be nil)
	Run(ctx context.Context, input json.RawMessage) (any, error)
}

// ToolMeta holds optional metadata about a tool, sourced from the MCP
// ToolAnnotations and protocol _meta fields. All fields are hints only.
type ToolMeta struct {
	// Title is a human-readable display name (takes precedence over Name).
	Title string

	// ReadOnlyHint indicates the tool does not modify its environment.
	ReadOnlyHint bool

	// DestructiveHint, when non-nil and true, indicates the tool may perform
	// destructive updates. Meaningful only when ReadOnlyHint is false.
	DestructiveHint *bool

	// IdempotentHint indicates repeated identical calls have no additional effect.
	// Meaningful only when ReadOnlyHint is false.
	IdempotentHint bool

	// OpenWorldHint, when non-nil and true, indicates the tool may interact
	// with external entities outside a closed domain (e.g. web search).
	OpenWorldHint *bool
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// reToolName matches a tool name, which models call the tool by
var reToolName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// IsToolName returns true if the name is a valid tool name, which is 1 to 64
// letters, digits, underscores or hyphens
func IsToolName(name string) bool {
	return reToolName.MatchString(name)
}
