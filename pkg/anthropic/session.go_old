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
	seq   []*Message
}

var _ llm.Context = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Return an empty session context object for the model, setting session options
func (model *model) Context(opts ...llm.Opt) llm.Context {
	return &session{
		model: model,
		opts:  opts,
		seq:   make([]*Message, 0, 10),
	}
}

// Convenience method to create a session context object with a user prompt, which
// panics on error
func (model *model) UserPrompt(prompt string, opts ...llm.Opt) llm.Context {
	context := model.Context(opts...)

	message, err := userPrompt(prompt, opts...)
	if err != nil {
		panic(err)
	}

	// Add to the sequence
	context.(*session).seq = append(context.(*session).seq, message)

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
	return 1
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

// Return the current session tool calls, or empty if no tool calls were made
func (session *session) ToolCalls(index int) []llm.ToolCall {
	// Sanity check for tool call
	if len(session.seq) == 0 {
		return nil
	}
	return session.seq[len(session.seq)-1].ToolCalls(index)
}

// Generate a response from a user prompt (with attachments) and
// other empheral options
func (session *session) FromUser(ctx context.Context, prompt string, opts ...llm.Opt) error {
	message, err := userPrompt(prompt, opts...)
	if err != nil {
		return err
	}

	// Append the user prompt to the sequence
	session.seq = append(session.seq, message)

	// The options come from the session options and the user options
	chatopts := make([]llm.Opt, 0, len(session.opts)+len(opts))
	chatopts = append(chatopts, session.opts...)
	chatopts = append(chatopts, opts...)

	// Call the 'chat' method
	r, err := session.model.Messages(ctx, session, chatopts...)
	if err != nil {
		return err
	}

	// Append the first message from the set of completions
	session.seq = append(session.seq, &r.Message)

	// Return success
	return nil
}

// Generate a response from a tool, passing the call identifier or
// function name, and the result
func (session *session) FromTool(ctx context.Context, results ...llm.ToolResult) error {
	message, err := toolResults(results...)
	if err != nil {
		return err
	}

	// Append the tool results to the sequence
	session.seq = append(session.seq, message)

	// Call the 'chat' method
	r, err := session.model.Messages(ctx, session, session.opts...)
	if err != nil {
		return err
	}

	// Append the first message from the set of completions
	session.seq = append(session.seq, &r.Message)

	// Return success
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

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
		contentData, err := NewAttachment(attachment, optEphemeral(opt), optCitations(opt))
		if err != nil {
			return nil, err
		}
		content = append(content, contentData)
	}

	// Return success
	return &Message{
		RoleContent: RoleContent{
			Role:    "user",
			Content: content,
		},
	}, nil
}

func toolResults(results ...llm.ToolResult) (*Message, error) {
	// Check for no results
	if len(results) == 0 {
		return nil, llm.ErrBadParameter.Withf("No tool results")
	}

	// Create user message
	message := Message{
		RoleContent{
			Role:    "user",
			Content: make([]*Content, 0, len(results)),
		},
	}
	for _, result := range results {
		message.RoleContent.Content = append(message.RoleContent.Content, NewToolResultContent(result))
	}

	// Return success
	return &message, nil
}
