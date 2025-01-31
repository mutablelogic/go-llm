package ollama

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Implementation of a message session, which is a sequence of messages
type session struct {
	seq []*MessageMeta
}

var _ llm.Context = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (*model) Context(...llm.Opt) llm.Context {
	// TODO: Currently ignoring options
	return &session{}
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

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Append user message context, with optional images
func (session *session) AppendUserPrompt(v string, opts ...llm.Opt) error {
	// Apply options
	opt, err := apply(opts...)
	if err != nil {
		return nil
	}

	var meta MessageMeta
	meta.Role = "user"
	meta.Content = v
	if len(opt.data) > 0 {
		meta.Images = make([]Data, len(opt.data))
		copy(meta.Images, opt.data)
	}

	// Append the message
	session.seq = append(session.seq, &meta)

	// Return success
	return nil
}

// Append the result of a tool call
func (session *session) AppendToolResult(id string, opts ...llm.Opt) error {
	// messages.append({'role': 'tool', 'content': str(output), 'name': tool.function.name})
	return nil
}

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
	return session.seq[len(session.seq)-1].Content
}
