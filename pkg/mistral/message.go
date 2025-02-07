package mistral

import (
	// Packages
	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Message with text or object content
type Message struct {
	RoleContent
	Calls ToolCalls `json:"tool_calls,omitempty"`
}

type RoleContent struct {
	Role    string `json:"role,omitempty"`         // assistant, user, tool, system
	Content any    `json:"content,omitempty"`      // string or array of text, reference, image_url
	Name    string `json:"name,omitempty"`         // function name - when role is tool
	Id      string `json:"tool_call_id,omitempty"` // tool call - when role is tool
}

var _ llm.Completion = (*Message)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - MESSAGE

func (Message) Num() int {
	return 1
}

func (message *Message) Role() string {
	return message.RoleContent.Role
}

// Return the completion
func (message *Message) Choice(index int) llm.Completion {
	if index != 0 {
		return nil
	}
	return message
}

func (message *Message) Text(index int) string {
	if index != 0 {
		return ""
	}
	// If content is text, return it
	if text, ok := message.Content.(string); ok {
		return text
	}
	// For other kinds, return empty string for the moment
	return ""
}

// Unsupported
func (message *Message) Audio(index int) *llm.Attachment {
	return nil
}

// Return all the tool calls
func (message *Message) ToolCalls(index int) []llm.ToolCall {
	if index != 0 {
		return nil
	}
	calls := make([]llm.ToolCall, 0, len(message.Calls))
	for _, call := range message.Calls {
		calls = append(calls, call)
	}
	return calls
}
