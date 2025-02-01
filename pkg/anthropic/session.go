package anthropic

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
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

// Return the current session tool calls, or empty if no tool calls were made
func (session *session) ToolCalls() []llm.ToolCall {
	// Sanity check for tool call
	if len(session.seq) == 0 {
		return nil
	}
	meta := session.seq[len(session.seq)-1]
	if meta.Role != "assistant" {
		return nil
	}

	// Gather tool calls
	var result []llm.ToolCall
	for _, content := range meta.Content {
		if content.Type == "tool_use" {
			result = append(result, tool.NewCall(content.ContentTool.Id, content.ContentTool.Name, content.ContentTool.Input))
		}
	}
	return result
}

// Generate a response from a user prompt (with attachments) and
// other empheral options
func (session *session) FromUser(ctx context.Context, prompt string, opts ...llm.Opt) error {
	// Append the user prompt to the sequence
	meta, err := userPrompt(prompt, opts...)
	if err != nil {
		return err
	} else {
		session.seq = append(session.seq, meta)
	}

	// The options come from the session options and the user options
	chatopts := make([]llm.Opt, 0, len(session.opts)+len(opts))
	chatopts = append(chatopts, session.opts...)
	chatopts = append(chatopts, opts...)

	// Call the 'chat' method
	client := session.model.client
	r, err := client.Messages(ctx, session, chatopts...)
	if err != nil {
		return err
	} else {
		session.seq = append(session.seq, &r.MessageMeta)
	}

	// Return success
	return nil
}

// Generate a response from a tool, passing the call identifier or
// function name, and the result
func (session *session) FromTool(ctx context.Context, results ...llm.ToolResult) error {
	meta, err := toolResults(results...)
	if err != nil {
		return err
	} else {
		session.seq = append(session.seq, meta)
	}

	// Call the 'chat' method
	client := session.model.client
	r, err := client.Messages(ctx, session, session.opts...)
	if err != nil {
		return err
	} else {
		session.seq = append(session.seq, &r.MessageMeta)
	}

	// Return success
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

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
	meta.Content[0] = NewTextContent(prompt)

	// Append any additional data
	for _, attachment := range attachments {
		content, err := attachmentContent(attachment, optEphemeral(opt), optCitations(opt))
		if err != nil {
			return nil, err
		}
		meta.Content = append(meta.Content, content)
	}

	// Return success
	return &meta, nil
}

func toolResults(results ...llm.ToolResult) (*MessageMeta, error) {
	// Check for no results
	if len(results) == 0 {
		return nil, llm.ErrBadParameter.Withf("No tool results")
	}

	// Create user message
	meta := MessageMeta{
		Role:    "user",
		Content: make([]*Content, 0, len(results)),
	}
	for _, result := range results {
		meta.Content = append(meta.Content, NewToolResultContent(result))
	}

	// Return success
	return &meta, nil
}
