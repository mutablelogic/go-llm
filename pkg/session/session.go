package session

import (
	"context"

	// Packages
	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACE

// Abstract interface for a message factory
type MessageFactory interface {
	// Generate a system prompt
	SystemPrompt(prompt string) Message

	// Generate a user prompt, with attachments and other options
	UserPrompt(string, ...llm.Opt) (Message, error)

	// Generate an array of results from calling tools
	ToolResults(...llm.ToolResult) ([]Message, error)
}

// Abstract interface for a message
type Message interface {
	llm.Completion
}

///////////////////////////////////////////////////////////////////////////////
// TYPES

// A chat session with history
type session struct {
	model llm.Model // The model used for the session
	opts  []llm.Opt // Options to apply to the session
	seq   []Message // Sequence of messages
}

var _ llm.Context = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new empty session with a capacity for 10 messages in the history
func NewSession(model llm.Model, factory MessageFactory, opts ...llm.Opt) *session {
	return &session{
		model: model,
		opts:  opts,
		seq:   make([]Message, 0, 10),
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return an array of messages in the session with system prompt. If the
// prompt is empty, no system prompt is prepended
func (session *session) WithSystemPrompt(prompt string) []Message {
	// TODO
	return nil
}

// Append a message to the session
func (session *session) Append(messages ...Message) {
	session.seq = append(session.seq, messages...)
}

// Generate a response from a user prompt (with attachments and other options)
func (session *session) FromUser(context.Context, string, ...llm.Opt) error {
	return llm.ErrNotImplemented
}

// Generate a response from a tool, passing the results from the tool call
func (session *session) FromTool(context.Context, ...llm.ToolResult) error {
	return llm.ErrNotImplemented
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - COMPLETION

// Return the number of completions
func (session *session) Num() int {
	if len(session.seq) == 0 {
		return 0
	}
	return session.seq[len(session.seq)-1].Num()
}

// Return the current session role
func (session *session) Role() string {
	if len(session.seq) == 0 {
		return ""
	}
	return session.seq[len(session.seq)-1].Role()
}

// Return the text for the last completion
func (session *session) Text(index int) string {
	if len(session.seq) == 0 {
		return ""
	}
	return session.seq[len(session.seq)-1].Text(index)
}

// Return audio for the last completion
func (session *session) Audio(index int) *llm.Attachment {
	if len(session.seq) == 0 {
		return nil
	}
	return session.seq[len(session.seq)-1].Audio(index)
}

// Return the current session tool calls given the completion index.
// Will return nil if no tool calls were returned.
func (session *session) ToolCalls(index int) []llm.ToolCall {
	if len(session.seq) == 0 {
		return nil
	}
	return session.seq[len(session.seq)-1].ToolCalls(index)
}
