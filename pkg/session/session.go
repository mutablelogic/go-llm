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
	SystemPrompt(prompt string) llm.Completion

	// Generate a user prompt, with attachments and other options
	UserPrompt(string, ...llm.Opt) (llm.Completion, error)

	// Generate an array of results from calling tools
	ToolResults(...llm.ToolResult) ([]llm.Completion, error)
}

///////////////////////////////////////////////////////////////////////////////
// TYPES

// A chat session with history
type session struct {
	model   llm.Model        // The model used for the session
	opts    []llm.Opt        // Options to apply to the session
	seq     []llm.Completion // Sequence of messages
	factory MessageFactory   // Factory for generating messages
}

var _ llm.Context = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new empty session to store a context window
func NewSession(model llm.Model, factory MessageFactory, opts ...llm.Opt) *session {
	return &session{
		model:   model,
		opts:    opts,
		seq:     make([]llm.Completion, 0, 10),
		factory: factory,
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return an array of messages in the session with system prompt. If the
// prompt is empty, no system prompt is prepended
func (session *session) WithSystemPrompt(prompt string) []llm.Completion {
	messages := make([]llm.Completion, 0, len(session.seq)+1)
	if prompt != "" {
		messages = append(messages, session.factory.SystemPrompt(prompt))
	}
	return append(messages, session.seq...)
}

// Append a message to the session
// TODO: Trim the context window to a maximum size
func (session *session) Append(messages ...llm.Completion) {
	session.seq = append(session.seq, messages...)
}

// Generate a response from a user prompt (with attachments and other options)
func (session *session) FromUser(ctx context.Context, prompt string, opts ...llm.Opt) error {
	// Append the user prompt
	message, err := session.factory.UserPrompt(prompt, opts...)
	if err != nil {
		return err
	} else {
		session.Append(message)
	}

	// Generate the completion
	completion, err := session.model.Chat(ctx, session.seq, session.opts...)
	if err != nil {
		return err
	}

	// Append the completion
	session.Append(completion)

	// Success
	return nil
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
