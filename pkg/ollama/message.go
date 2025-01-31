package ollama

// Packages

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Chat Message
type MessageMeta struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	Images    []Data     `json:"images,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Data represents the raw binary data of an image file.
type Data []byte
