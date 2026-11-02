package schema

import (
	"encoding/json"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// ToolCall represents a tool invocation requested by the model
type ToolCall struct {
	ID    string          `json:"id,omitempty" help:"Provider-assigned tool call identifier" example:"call_123"`
	Name  string          `json:"name" help:"Tool name to invoke" example:"get_weather"`
	Input json.RawMessage `json:"input,omitempty" help:"JSON-encoded arguments passed to the tool" example:"{\"city\":\"London\"}"`
	Meta  map[string]any  `json:"meta,omitempty" help:"Provider-specific metadata associated with the tool call" example:"{\"provider\":\"demo\"}"`
}

// ToolResult represents the result of running a tool
type ToolResult struct {
	ID      string          `json:"id,omitempty" help:"Tool call identifier this result belongs to" example:"call_123"`
	Name    string          `json:"name,omitempty" help:"Tool name that produced this result" example:"get_weather"`
	Content json.RawMessage `json:"content,omitempty" help:"JSON-encoded tool output content" example:"{\"temperature_c\":18}"`
	IsError bool            `json:"is_error,omitempty" help:"Whether the tool result represents an error" example:"false"`
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewToolResult creates a content block containing the result of the tool
// call with the id and name, or a tool error if the result is an error, or
// can't be encoded as JSON
func NewToolResult(id, name string, v any) ContentBlock {
	if err, ok := v.(error); ok {
		return NewToolError(id, name, err)
	}
	data, err := json.Marshal(v)
	if err != nil {
		return NewToolError(id, name, err)
	}
	return ContentBlock{
		ToolResult: &ToolResult{
			ID:      id,
			Name:    name,
			Content: json.RawMessage(data),
		},
	}
}

// NewToolError creates a content block containing the error from the tool
// call with the id and name, with the error message as a JSON string
func NewToolError(id, name string, err error) ContentBlock {
	data, _ := json.Marshal(err.Error())
	return ContentBlock{
		ToolResult: &ToolResult{
			ID:      id,
			Name:    name,
			Content: json.RawMessage(data),
			IsError: true,
		},
	}
}
