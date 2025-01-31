package ollama

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Chat Message
type MessageMeta struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	Images    []Data     `json:"images,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Implementation of a message session, which is a sequence of messages
type messages struct {
	seq []*MessageMeta
}

var _ llm.Context = (*messages)(nil)

// Data represents the raw binary data of an image file.
type Data []byte

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m messages) String() string {
	var data []byte
	var err error
	if len(m.seq) == 1 {
		data, err = json.MarshalIndent(m.seq[0], "", "  ")
	} else {
		data, err = json.MarshalIndent(m.seq, "", "  ")
	}
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create user message context, with optional images
func (*model) UserPrompt(v string, opts ...llm.Opt) llm.Context {
	// Apply options
	opt, err := apply(opts...)
	if err != nil {
		return nil
	}

	var meta MessageMeta
	meta.Role = "user"
	meta.Content = v
	if len(opt.data) > 0 {
		meta.Images = make([]Data, len(opt.data))
		copy(meta.Images, opt.data)
	}

	// Return prompt
	return &messages{
		seq: []*MessageMeta{&meta},
	}
}

// The result of a tool call
func (*model) ToolResult(id string, opts ...llm.Opt) llm.Context {
	// messages.append({'role': 'tool', 'content': str(output), 'name': tool.function.name})
	return nil
}

// Return the role of the last message
func (m messages) Role() string {
	if len(m.seq) == 0 {
		return ""
	}
	return m.seq[len(m.seq)-1].Role
}
