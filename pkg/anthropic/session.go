package anthropic

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

//////////////////////////////////////////////////////////////////
// TYPES

type session struct {
	model *model
	opts  []llm.Opt
	seq   []*MessageMeta
}

var _ llm.Context = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Return am empty session context object for the model,
// setting session options
func (model *model) Context(opts ...llm.Opt) llm.Context {
	return &session{
		model: model,
		opts:  opts,
	}
}

// Convenience method to create a session context object
// with a user prompt, which panics on error
func (model *model) UserPrompt(prompt string, opts ...llm.Opt) llm.Context {
	// Apply attachments
	opt, err := apply(opts...)
	if err != nil {
		panic(err)
	}

	meta := MessageMeta{
		Role:    "user",
		Content: make([]*Content, 1, len(opt.data)+1),
	}

	// Append the text
	meta.Content[0] = NewTextContent(prompt)

	// Append any additional data
	for _, data := range opt.data {
		meta.Content = append(meta.Content, data)
	}

	// Return success
	return nil
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
	data, err := json.MarshalIndent(meta.Content, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

// Generate a response from a user prompt (with attachments and
// other empheral options
func (session *session) FromUser(context.Context, string, ...llm.Opt) (llm.Context, error) {
	return nil, llm.ErrNotImplemented
}

// Generate a response from a tool, passing the call identifier or
// function name, and the result
func (session *session) FromTool(context.Context, string, any, ...llm.Opt) (llm.Context, error) {
	return nil, llm.ErrNotImplemented
}
