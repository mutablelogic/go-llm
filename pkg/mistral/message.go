package mistral

import (
	"encoding/json"

	"github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Possible completions
type Completions []Completion

var _ llm.Completion = Completions{}

// Message with text or object content
type Message struct {
	RoleContent
	ToolCallArray `json:"tool_calls,omitempty"`
}

type RoleContent struct {
	Role    string `json:"role,omitempty"`         // assistant, user, tool, system
	Id      string `json:"tool_call_id,omitempty"` // tool call - when role is tool
	Name    string `json:"name,omitempty"`         // function name - when role is tool
	Content any    `json:"content,omitempty"`      // string or array of text, reference, image_url
}

var _ llm.Completion = (*Message)(nil)

// Completion Variation
type Completion struct {
	Index   uint64   `json:"index"`
	Message *Message `json:"message"`
	Delta   *Message `json:"delta,omitempty"` // For streaming
	Reason  string   `json:"finish_reason,omitempty"`
}

type Content struct {
	Type        string                       `json:"type,omitempty"` // text, reference, image_url
	*Text       `json:"text,omitempty"`      // text content
	*Prediction `json:"content,omitempty"`   // prediction
	*Image      `json:"image_url,omitempty"` // image_url
}

// A set of tool calls
type ToolCallArray []ToolCall

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
// PUBLIC METHODS - MESSAGE

func (m Message) Num() int {
	return 1
}

func (m Message) Role() string {
	return m.RoleContent.Role
}

func (m Message) Text(index int) string {
	if index != 0 {
		return ""
	}
	// If content is text, return it
	if text, ok := m.Content.(string); ok {
		return text
	}
	// For other kinds, return empty string for the moment
	return ""
}

func (m Message) ToolCalls(index int) []llm.ToolCall {
	if index != 0 {
		return nil
	}

	// Make the tool calls
	calls := make([]llm.ToolCall, 0, len(m.ToolCallArray))
	for _, call := range m.ToolCallArray {
		var args map[string]any
		if call.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				return nil
			}
		}
		calls = append(calls, tool.NewCall(call.Id, call.Function.Name, args))
	}

	// Return success
	return calls
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - COMPLETIONS

// Return the number of completions
func (c Completions) Num() int {
	return len(c)
}

// Return message for a specific completion
func (c Completions) Message(index int) *Message {
	if index < 0 || index >= len(c) {
		return nil
	}
	return c[index].Message
}

// Return the role of the completion
func (c Completions) Role() string {
	// The role should be the same for all completions, let's use the first one
	if len(c) == 0 {
		return ""
	}
	return c[0].Message.Role()
}

// Return the text content for a specific completion
func (c Completions) Text(index int) string {
	if index < 0 || index >= len(c) {
		return ""
	}
	return c[index].Message.Text(0)
}

// Return the current session tool calls given the completion index.
// Will return nil if no tool calls were returned.
func (c Completions) ToolCalls(index int) []llm.ToolCall {
	if index < 0 || index >= len(c) {
		return nil
	}
	return c[index].Message.ToolCalls(0)
}
