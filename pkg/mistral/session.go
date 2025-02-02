package mistral

import (
	// Packages
	"context"
	"encoding/json"

	llm "github.com/mutablelogic/go-llm"
)

//////////////////////////////////////////////////////////////////
// TYPES

type session struct {
	model *model         // The model used for the session
	opts  []llm.Opt      // Options to apply to the session
	seq   []*MessageMeta // Sequence of messages
}

var _ llm.Context = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Return an empty session context object for the model, setting session options
func (model *model) Context(opts ...llm.Opt) llm.Context {
	return &session{
		model: model,
		opts:  opts,
	}
}

// Convenience method to create a session context object with a user prompt, which
// panics on error
func (model *model) UserPrompt(prompt string, opts ...llm.Opt) llm.Context {
	context := model.Context(opts...)

	meta, err := userPrompt(prompt, opts...)
	if err != nil {
		panic(err)
	}

	// Add to the sequence
	context.(*session).seq = append(context.(*session).seq, meta)

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

// Return the role of the last message
func (session *session) Role() string {
	if len(session.seq) == 0 {
		return ""
	}
	return session.seq[len(session.seq)-1].Role
}

// Return the text of the last message
func (session *session) Text() string {
	if len(session.seq) == 0 {
		return ""
	}
	meta := session.seq[len(session.seq)-1]
	return meta.Text()
}

// Return the text of the last message
func (session *session) ToolCalls() []llm.ToolCall {
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

func systemPrompt(prompt string) *MessageMeta {
	return &MessageMeta{
		Role:    "system",
		Content: prompt,
	}
}

func userPrompt(prompt string, opts ...llm.Opt) (*MessageMeta, error) {
	// Apply attachments
	opt, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Get attachments
	attachments := opt.Attachments()

	// Create user message
	meta := MessageMeta{
		Role:    "user",
		Content: make([]*Content, 1, len(attachments)+1),
	}

	// Append the text
	meta.Content = []*Content{
		NewTextContent(prompt),
	}

	// Append any additional data
	// TODO
	/*
		for _, attachment := range attachments {
			content, err := attachmentContent(attachment)
			if err != nil {
				return nil, err
			}
			meta.Content = append(meta.Content, content)
		}
	*/

	// Return success
	return &meta, nil
}
