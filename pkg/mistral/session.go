package mistral

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

//////////////////////////////////////////////////////////////////
// TYPES

type session struct {
	model *model        // The model used for the session
	opts  []llm.Opt     // Options to apply to the session
	seq   []Completions // Sequence of messages
}

var _ llm.Context = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Return an empty session context object for the model, setting session options
func (model *model) Context(opts ...llm.Opt) llm.Context {
	return &session{
		model: model,
		opts:  opts,
		seq:   make([]Completions, 0, 10),
	}
}

// Convenience method to create a session context object with a user prompt, which
// panics on error
func (model *model) UserPrompt(prompt string, opts ...llm.Opt) llm.Context {
	context := model.Context(opts...)

	// Create a user prompt
	message, err := userPrompt(prompt, opts...)
	if err != nil {
		panic(err)
	}

	// Add to the sequence
	context.(*session).seq = append(context.(*session).seq, []Completion{
		{Message: message},
	})

	// Return success
	return context
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (session session) String() string {
	var data []byte
	var err error
	if len(session.seq) == 1 {
		data, err = json.MarshalIndent(session.seq[0], "", "  ")
	} else {
		data, err = json.MarshalIndent(session.seq, "", "  ")
	}
	if err != nil {
		return err.Error()
	}
	return string(data)
}

//////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return the number of completions
func (session *session) Num() int {
	if len(session.seq) == 0 {
		return 0
	}
	return session.seq[len(session.seq)-1].Num()
}

// Return the role of the last message
func (session *session) Role() string {
	if len(session.seq) == 0 {
		return ""
	}
	return session.seq[len(session.seq)-1].Role()
}

// Return the text of the last message
func (session *session) Text(index int) string {
	if len(session.seq) == 0 {
		return ""
	}
	return session.seq[len(session.seq)-1].Text(index)
}

// Return tool calls for the last message
func (session *session) ToolCalls(index int) []llm.ToolCall {
	return nil
}

// Generate a response from a user prompt (with attachments and
// other options)
func (session *session) FromUser(context.Context, string, ...llm.Opt) error {
	return llm.ErrNotImplemented
}

// Generate a response from a tool, passing the results
// from the tool call
func (session *session) FromTool(context.Context, ...llm.ToolResult) error {
	return llm.ErrNotImplemented
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func systemPrompt(prompt string) *Message {
	return &Message{
		Role:    "system",
		Content: prompt,
	}
}

func userPrompt(prompt string, opts ...llm.Opt) (*Message, error) {
	// Get attachments
	opt, err := llm.ApplyPromptOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Get attachments, allocate content
	attachments := opt.Attachments()
	content := make([]*Content, 1, len(attachments)+1)

	// Append the text and the attachments
	content[0] = NewTextContent(prompt)
	for _, attachment := range attachments {
		content = append(content, NewImageAttachment(attachment))
	}

	// Return success
	return &Message{
		Role:    "user",
		Content: content,
	}, nil
}
