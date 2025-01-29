package anthropic

import (
	"encoding/json"

	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Implementation of a message
type message struct {
	MessageMeta
}

var _ llm.Context = (*message)(nil)

// Message with text or object content
type MessageMeta struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type Attachment struct {
	Type   string `json:"type"` // image, document
	Source struct {
		Type         string `json:"type"`                    // base64
		MediaType    string `json:"media_type"`              // image/jpeg, image/png, image/gif, image/webp, application/pdf
		Data         string `json:"data"`                    // ...base64 encoded data
		CacheControl string `json:"cache_control,omitempty"` // ephemeral
	} `json:"source"`
}

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

func (m message) Role() string {
	return m.MessageMeta.Role
}

// Create user message context
func (*Client) UserPrompt(text string, opts ...llm.Opt) llm.Context {
	context := &message{}
	context.MessageMeta.Role = "user"
	context.MessageMeta.Content = text
	return context
}

// Create the result of calling a tool
func (*Client) ToolResult(any) llm.Context {
	return nil
}
