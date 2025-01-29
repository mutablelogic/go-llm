package ollama

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Implementation of a message
type message struct {
	MessageMeta
}

var _ llm.Context = (*message)(nil)

// Chat Message
type MessageMeta struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	Images    []Data     `json:"images,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Data represents the raw binary data of an image file.
type Data []byte

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m message) String() string {
	data, err := json.MarshalIndent(m.MessageMeta, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create user message context, with optional images
func (ollama *Client) UserPrompt(v string, opts ...llm.Opt) llm.Context {
	// Apply options
	opt, err := apply(opts...)
	if err != nil {
		return nil
	}

	m := new(message)
	m.MessageMeta.Role = "user"
	m.MessageMeta.Content = v
	if len(opt.data) > 0 {
		m.MessageMeta.Images = make([]Data, len(opt.data))
		copy(m.MessageMeta.Images, opt.data)
	}

	// Return success
	return m
}

// The result of a tool call
func (ollama *Client) ToolResult(v any) llm.Context {
	m := new(message)
	m.MessageMeta.Role = "tool"

	switch v := v.(type) {
	case string:
		m.MessageMeta.Content = v
	default:
		// Encode the result into json
		data, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		m.MessageMeta.Content = string(data)
	}

	// Return success
	return m
}

// Return the role of a message
func (m message) Role() string {
	return m.MessageMeta.Role
}
