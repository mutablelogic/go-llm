package ollama

import (
	"context"
	"encoding/json"
	"fmt"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Implementation of a message session, which is a sequence of messages
type session struct {
	opts  []llm.Opt
	model *model
	seq   []*MessageMeta
}

var _ llm.Context = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new empty context
func (model *model) Context(opts ...llm.Opt) llm.Context {
	return &session{
		model: model,
		opts:  opts,
	}
}

// Create a new context with a user prompt
func (model *model) UserPrompt(prompt string, opts ...llm.Opt) llm.Context {
	context := model.Context(opts...)
	context.(*session).seq = append(context.(*session).seq, &MessageMeta{
		Role:    "user",
		Content: prompt,
	})
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

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Generate a response from a user prompt (with attachments)
func (s *session) FromUser(ctx context.Context, prompt string, opts ...llm.Opt) error {
	// Append the user prompt
	if user, err := userPrompt(prompt, opts...); err != nil {
		return err
	} else {
		s.seq = append(s.seq, user)
	}

	// The options come from the session options and the user options
	chatopts := make([]llm.Opt, 0, len(s.opts)+len(opts))
	chatopts = append(chatopts, s.opts...)
	chatopts = append(chatopts, opts...)

	// Call the 'chat' method
	client := s.model.client
	r, err := client.Chat(ctx, s, chatopts...)
	if err != nil {
		return err
	} else {
		s.seq = append(s.seq, &r.Message)
	}

	// Return success
	return nil
}

// Generate a response from a tool calling result
func (s *session) FromTool(ctx context.Context, call string, result any, opts ...llm.Opt) error {
	// Append the tool result
	if message, err := toolResult(call, result); err != nil {
		return err
	} else {
		s.seq[len(s.seq)-1] = message
	}

	// The options come from the session options and the user options
	chatopts := make([]llm.Opt, 0, len(s.opts)+len(opts))
	chatopts = append(chatopts, s.opts...)
	chatopts = append(chatopts, opts...)

	// Call the 'chat' method
	r, err := s.model.client.Chat(ctx, s, chatopts...)
	if err != nil {
		return err
	} else {
		s.seq = append(s.seq, &r.Message)
	}

	// Return success
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

// Return the tool calls of the last message
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
	for _, call := range meta.ToolCalls {
		result = append(result, tool.NewCall(fmt.Sprint(call.Function.Index), call.Function.Name, call.Function.Arguments))
	}
	return result
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func userPrompt(prompt string, opts ...llm.Opt) (*MessageMeta, error) {
	// Apply options for attachments
	opt, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Create a new message
	var meta MessageMeta
	meta.Role = "user"
	meta.Content = prompt

	if attachments := opt.Attachments(); len(attachments) > 0 {
		meta.Images = make([]Data, len(attachments))
		for i, attachment := range attachments {
			meta.Images[i] = attachment.Data()
		}
	}

	// Return success
	return &meta, nil
}

func toolResult(name string, result any) (*MessageMeta, error) {
	// Turn result into JSON
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	// Create a new message
	var meta MessageMeta
	meta.Role = "tool"
	meta.FunctionName = name
	meta.Content = string(data)

	// Return success
	return &meta, nil
}
