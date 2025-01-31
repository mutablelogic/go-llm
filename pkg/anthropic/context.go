package anthropic

import (
	"encoding/json"

	llm "github.com/mutablelogic/go-llm"
)

//////////////////////////////////////////////////////////////////
// TYPES

type session struct {
	seq []*MessageMeta
}

var _ llm.Context = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (*model) Context(...llm.Opt) (llm.Context, error) {
	// TODO: Currently ignoring options
	return &session{}, nil
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

// Append user prompt (and attachments) to a context
func (session *session) AppendUserPrompt(text string, opts ...llm.Opt) error {
	// Apply attachments
	opt, err := apply(opts...)
	if err != nil {
		return err
	}

	meta := MessageMeta{
		Role:    "user",
		Content: make([]*Content, 1, len(opt.data)+1),
	}

	// Append the text
	meta.Content[0] = NewTextContent(text)

	// Append any additional data
	for _, data := range opt.data {
		meta.Content = append(meta.Content, data)
	}

	// Return success
	return nil
}

// Append the result of calling a tool to a context
func (session *session) AppendToolResult(string, ...llm.Opt) error {
	return llm.ErrNotImplemented
}
