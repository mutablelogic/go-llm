package llm

import (
	"context"
	"encoding/json"
	"regexp"

	// Packages
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

////////////////////////////////////////////////////////////////////////////////
// INTERFACES

// Tool is an interface for a callable tool with a name, description,
// input schema, and optional output schema.
type Tool interface {
	// Return the name of the tool
	Name() string

	// Return the description of the tool
	Description() string

	// Return the JSON schema for the tool input parameters.
	InputSchema() *jsonschema.Schema

	// Return the JSON schema for the tool output, or nil if unspecified.
	OutputSchema() *jsonschema.Schema

	// Run the tool with the given input as JSON (may be nil). The result is
	// JSON as a json.RawMessage, a Resource such as an image or audio, any
	// other value which can be encoded as JSON, or nil when the tool has no
	// result.
	Run(ctx context.Context, input json.RawMessage) (any, error)
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
