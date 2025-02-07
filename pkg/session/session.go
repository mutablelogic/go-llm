package session

import (
	"context"
	"encoding/json"

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

type Model interface {
	// Additional method for a context object
	Chat(ctx context.Context, completions []llm.Completion, opts ...llm.Opt) (llm.Completion, error)
}

///////////////////////////////////////////////////////////////////////////////
// TYPES

// A chat session with history
type session struct {
	model   Model            // The model used for the session
	opts    []llm.Opt        // Options to apply to the session
	seq     []llm.Completion // Sequence of messages
	factory MessageFactory   // Factory for generating messages
}

var _ llm.Context = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new empty session to store a context window
func NewSession(model llm.Model, factory MessageFactory, opts ...llm.Opt) *session {
	chatmodel, ok := model.(Model)
	if !ok || model == nil {
		panic("Model does not implement the session.Model interface")
	}
	return &session{
		model:   chatmodel,
		opts:    opts,
		seq:     make([]llm.Completion, 0, 10),
		factory: factory,
	}
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (session session) MarshalJSON() ([]byte, error) {
	return json.Marshal(session.seq)
}

func (session session) String() string {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
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
		return session.chat(ctx, message)
	}
}

// Generate a response from a tool, passing the results from the tool call
func (session *session) FromTool(ctx context.Context, results ...llm.ToolResult) error {
	// Append the tool results
	if results, err := session.factory.ToolResults(results...); err != nil {
		return err
	} else {
		return session.chat(ctx, results...)
	}
}

func (session *session) chat(ctx context.Context, messages ...llm.Completion) error {
	// Append the messages to the chat
	session.Append(messages...)

	// Generate the completion, and append the first choice of the completion
	// TODO: Use Opts to select which completion choice to use
	completion, err := session.model.Chat(ctx, session.seq, session.opts...)
	if err != nil {
		return err
	} else if completion.Num() == 0 {
		return llm.ErrBadParameter.With("No completion choices returned")
	}

	// Append the first choice
	session.Append(completion.Choice(0))

	// Success
	return nil
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

// Return the current session choice
func (session *session) Choice(index int) llm.Completion {
	if len(session.seq) == 0 {
		return nil
	}
	return session.seq[len(session.seq)-1].Choice(index)
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
