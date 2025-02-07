package openai

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Message with text or object content
type Message struct {
	RoleContent
	Media *llm.Attachment `json:"audio,omitempty"`
	Calls ToolCalls       `json:"tool_calls,omitempty"`
	*ToolResults
}

var _ llm.Completion = (*Message)(nil)

type RoleContent struct {
	Role    string `json:"role,omitempty"`    // assistant, user, tool, system
	Content any    `json:"content,omitempty"` // string or array of text, reference, image_url
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
	// If content is text, return it
	if text, ok := message.Content.(string); ok && text != "" {
		return text
	}
	// If content is audio, and there is a caption, return it
	if audio := message.Audio(0); audio != nil && audio.Caption() != "" {
		return audio.Caption()
	}

	// For other kinds, return empty string for the moment
	return ""
}

// Return the audio
func (message *Message) Audio(index int) *llm.Attachment {
	if index != 0 {
		return nil
	}
	return message.Media
}

// Return the current session tool calls given the completion index.
// Will return nil if no tool calls were returned.
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
