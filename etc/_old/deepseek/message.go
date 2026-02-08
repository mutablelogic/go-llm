package deepseek

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Message with text or object content
type Message struct {
	RoleContent
}

var _ llm.Completion = (*Message)(nil)

type RoleContent struct {
	Role    string `json:"role,omitempty"`    // assistant, user, tool, system
	Content string `json:"content,omitempty"` // string
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return the number of completions
func (Message) Num() int {
	return 1
}

// Return the current session role
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

// Return the text for the last completion
func (message *Message) Text(index int) string {
	if index != 0 {
		return ""
	}
	return message.Content
}

// No attachments
func (message *Message) Attachment(index int) *llm.Attachment {
	return nil
}

// No tool calls
func (message *Message) ToolCalls(index int) []llm.ToolCall {
	return nil
}
