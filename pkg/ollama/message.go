package ollama

// Packages

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

// Data represents the raw binary data of an image file.
type Data []byte
