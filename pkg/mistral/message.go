package mistral

import (
	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Possible completions
type Completions []Completion

var _ llm.Completion = Completions{}

// Completion Variation
type Completion struct {
	Index   uint64   `json:"index"`
	Message *Message `json:"message"`
	Delta   *Message `json:"delta,omitempty"` // For streaming
	Reason  string   `json:"finish_reason,omitempty"`
}

// Message with text or object content
type Message struct {
	Role      string `json:"role,omitempty"` // assistant, user, tool, system
	Prefix    bool   `json:"prefix,omitempty"`
	Content   any    `json:"content,omitempty"`
	ToolCalls `json:"tool_calls,omitempty"`
}

type Content struct {
	Type        string                       `json:"type"` // text, reference, image_url
	*Text       `json:"text,omitempty"`      // text content
	*Prediction `json:"content,omitempty"`   // prediction
	*Image      `json:"image_url,omitempty"` // image_url
}

// A set of tool calls
type ToolCalls []ToolCall

// text content
type Text string

// text content
type Prediction string

// either a URL or "data:image/png;base64," followed by the base64 encoded image
type Image string

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Return a Content object with text content (either in "text" or "prediction" field)
func NewContent(t, v, p string) *Content {
	content := new(Content)
	content.Type = t
	if v != "" {
		content.Text = (*Text)(&v)
	}
	if p != "" {
		content.Prediction = (*Prediction)(&p)
	}
	return content
}

// Return a Content object with text content
func NewTextContent(v string) *Content {
	return NewContent("text", v, "")
}

// Return an image attachment
func NewImageAttachment(a *llm.Attachment) *Content {
	content := new(Content)
	image := a.Url()
	content.Type = "image_url"
	content.Image = (*Image)(&image)
	return content
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return the number of completions
func (c Completions) Num() int {
	return len(c)
}

// Return the role of the completion
func (c Completions) Role() string {
	// The role should be the same for all completions, let's use the first one
	if len(c) == 0 {
		return ""
	}
	return c[0].Message.Role
}

// Return the text content for a specific completion
func (c Completions) Text(index int) string {
	if index < 0 || index >= len(c) {
		return ""
	}
	completion := c[index].Message
	if text, ok := completion.Content.(string); ok {
		return text
	}
	// TODO: Will the text be in other forms?
	return ""
}

// Return the current session tool calls given the completion index.
// Will return nil if no tool calls were returned.
func (c Completions) ToolCalls(index int) []llm.ToolCall {
	if index < 0 || index >= len(c) {
		return nil
	}

	// Get the completion
	completion := c[index].Message
	if completion == nil {
		return nil
	}

	// Make the tool calls
	calls := make([]llm.ToolCall, 0, len(completion.ToolCalls))
	for _, call := range completion.ToolCalls {
		calls = append(calls, &toolcall{call})
	}

	// Return success
	return calls
}

// Return message for a specific completion
func (c Completions) Message(index int) *Message {
	if index < 0 || index >= len(c) {
		return nil
	}
	return c[index].Message
}
