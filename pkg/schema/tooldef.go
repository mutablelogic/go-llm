package schema

// ToolDefinition represents a provider-agnostic tool definition.
// Providers can reshape this into their required payloads.
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"input_schema,omitempty"`
}
