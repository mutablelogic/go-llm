package ollama

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
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
func (model *model) Context(opts ...llm.Opt) (llm.Context, error) {
	return &session{
		model: model,
		opts:  opts,
	}, nil
}

// Create a new context with a user prompt
func (model *model) MustUserPrompt(prompt string, opts ...llm.Opt) llm.Context {
	context, err := model.Context(opts...)
	if err != nil {
		panic(err)
	}
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
func (s *session) FromUser(ctx context.Context, prompt string, opts ...llm.Opt) (llm.Context, error) {
	// Make a new session
	response := new(session)
	response.model = s.model
	response.opts = s.opts
	response.seq = make([]*MessageMeta, len(s.seq)+1, len(s.seq)+2)

	// Append the user prompt
	if user, err := userPrompt(prompt, opts...); err != nil {
		return nil, err
	} else {
		response.seq[len(response.seq)-1] = user
	}

	// Call the 'chat' method
	client := s.model.client
	r, err := client.Chat(ctx, response, response.opts...)
	if err != nil {
		return nil, err
	} else {
		response.seq = append(response.seq, &r.Message)
	}

	// Return success
	return response, nil
}

// Generate a response from a tool calling result
func (s *session) FromTool(ctx context.Context, call string, result any) (llm.Context, error) {
	// Make a new session
	response := new(session)
	response.model = s.model
	response.opts = s.opts
	response.seq = make([]*MessageMeta, len(s.seq)+1, len(s.seq)+2)

	// Append the tool result
	if message, err := toolResult(call, result); err != nil {
		return nil, err
	} else {
		response.seq[len(response.seq)-1] = message
	}

	// Call the 'chat' method
	r, err := s.model.client.Chat(ctx, response, response.opts...)
	if err != nil {
		return nil, err
	} else {
		response.seq = append(response.seq, &r.Message)
	}

	// Return success
	return response, nil
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

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func userPrompt(prompt string, opts ...llm.Opt) (*MessageMeta, error) {
	// Apply options
	opt, err := apply(opts...)
	if err != nil {
		return nil, err
	}

	// Create a new message
	var meta MessageMeta
	meta.Role = "user"
	meta.Content = prompt
	if len(opt.data) > 0 {
		meta.Images = make([]Data, len(opt.data))
		copy(meta.Images, opt.data)
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
