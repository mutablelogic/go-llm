package ollama

import (
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Chat Message
type MessageMeta struct {
	Role         string     `json:"role"`
	Content      string     `json:"content,omitempty"`
	FunctionName string     `json:"name,omitempty"`       // Function name for a tool result
	Images       []Data     `json:"images,omitempty"`     // Image attachments
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"` // Tool calls from the assistant
}

type ToolCall struct {
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Index     int            `json:"index,omitempty"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// Data represents the raw binary data of an image file.
type Data []byte

// ToolFunction
type ToolFunction struct {
	Type     string   `json:"type"` // function
	Function llm.Tool `json:"function"`
}
