package registry

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// respondAttempts is how many times a model is asked for its reply, when the
// reply has a schema
const respondAttempts = 3

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Ask returns the reply of the request's model to its messages, which
// include earlier turns of the conversation. The model replies after the
// system prompt, with the options, and can call the tools, which are got
// from the registry's toolkit, and whose calls are in its reply, for the
// caller to run, rather than run. When the request has a schema, the model
// replies by calling the respond tool, whose result is the reply's result:
// a model which replies with text is asked once to call it, and a result
// which doesn't conform to the schema is returned to the model to correct,
// until it's been asked respondAttempts times. The reply is streamed to the
// function as it's generated, when it isn't nil, with each of the model's
// replies separated by a blank line. Returns ErrBadParameter when the
// request isn't valid, ErrNotFound when the model, or a tool, isn't found,
// and the errors of WithoutSession.
func (r *Registry) Ask(ctx context.Context, req schema.AskRequest, stream llm.GeneratorStreamFn) (_ *schema.AskResponse, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Ask",
		attribute.String("arg.req", req.String()),
	)
	defer func() { endSpan(err) }()

	// The model, and its messages
	if strings.TrimSpace(req.Model) == "" {
		return nil, llm.ErrBadParameter.With("missing model")
	} else if len(req.Messages) == 0 {
		return nil, llm.ErrBadParameter.With("missing messages")
	}
	messages := make([]llm.Message, len(req.Messages), len(req.Messages)+2*respondAttempts)
	for i := range req.Messages {
		messages[i] = &providerschema.Message{MessageMeta: req.Messages[i]}
	}

	// The tool the model replies with, when the reply has a schema, which is
	// in the system prompt
	respond, err := newRespond(req)
	if err != nil {
		return nil, err
	}
	system := req.System
	if respond != nil {
		system = respondSystemPrompt(system)
	}
	opts, err := req.Opts(system)
	if err != nil {
		return nil, err
	}

	// The tools, and the respond tool, which the model can call
	tools, err := r.askTools(ctx, req, respond)
	if err != nil {
		return nil, err
	}

	// The model
	model, err := r.GetModel(ctx, strings.TrimSpace(req.Model))
	if err != nil {
		return nil, err
	}
	canonical := r.Name(model)

	// Ask the model, until it replies with a result which conforms to the
	// schema, when the request has one
	var usage llm.Usage
	var nudged bool
	replies := newReplyStream(stream)
	for attempt := 1; ; attempt++ {
		reply, replyUsage, err := r.WithoutSession(ctx, model, messages, tools, replies.Next(), opts...)
		if err != nil {
			return nil, err
		}
		usage = addUsage(usage, replyUsage)
		message := newReply(reply)
		response := &schema.AskResponse{
			Model:   canonical,
			Message: message,
			Schema:  respond != nil,
			Usage:   usage,
		}
		if respond == nil {
			return response, nil
		}

		// The model replied with text, so it's asked once to call the tool,
		// unless it's been asked as many times as it can be
		calls := message.ToolCalls()
		if len(calls) == 0 {
			if nudged || attempt >= respondAttempts {
				return response, nil
			}
			nudged = true
			messages = append(messages, &message, &providerschema.Message{MessageMeta: providerschema.MessageMeta{
				Sender:  "user",
				Content: []providerschema.ContentBlock{{Text: types.Ptr(toolkit.RespondNudge)}},
			}})
			continue
		}

		// The model called other tools, but not the respond tool, which the
		// caller runs
		index := slices.IndexFunc(calls, func(call llm.ToolCall) bool { return call.Name == toolkit.RespondName })
		if index < 0 {
			return response, nil
		}

		// The model replied with a result which conforms
		call := calls[index]
		result, err := respond.Run(ctx, call.Input)
		if err == nil {
			response.Result = result.(json.RawMessage)
			return response, nil
		}

		// The result doesn't conform, so the model is told why, unless it
		// called other tools too, which the caller runs, or it's been asked
		// as many times as it can be
		if len(calls) > 1 || attempt >= respondAttempts {
			return response, nil
		}
		messages = append(messages, &message, &providerschema.Message{MessageMeta: providerschema.MessageMeta{
			Sender:  "tool",
			Content: []providerschema.ContentBlock{providerschema.NewToolError(call.ID, call.Name, err)},
		}})
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// askTools returns the tools of the request, from the registry's toolkit,
// and the respond tool, when there is one. When the tool choice is "none",
// there are no tools, and no tool choice, so the provider needn't support
// it, unless the reply has a schema, which needs the respond tool. Any other
// tool choice needs tools to choose from.
func (r *Registry) askTools(ctx context.Context, req schema.AskRequest, respond *toolkit.Respond) ([]llm.Tool, error) {
	var tools []llm.Tool
	if len(req.Tools) > 0 {
		if r.toolkit == nil {
			return nil, llm.ErrNotFound.With("no tools, as the registry has no toolkit")
		}
		var err error
		if tools, err = r.toolkit.Tools(ctx, req.Tools...); err != nil {
			return nil, err
		}
	}

	// The tool choice
	var choice string
	if value, exists := req.Opt[llm.ToolChoiceKey]; exists {
		if err := json.Unmarshal(value, &choice); err != nil {
			return nil, llm.ErrBadParameter.Withf("invalid option %q: %v", llm.ToolChoiceKey, err)
		}
	}
	switch {
	case choice == "none" && respond != nil:
		return nil, llm.ErrBadParameter.With("a schema needs tools, so the tool choice can't be none")
	case choice == "none":
		return nil, nil
	case choice != "" && len(tools) == 0 && respond == nil:
		return nil, llm.ErrBadParameter.With("a tool choice needs tools")
	}

	// The respond tool, whose name no other tool can have
	if respond != nil {
		for _, tool := range tools {
			if tool.Name() == toolkit.RespondName {
				return nil, llm.ErrBadParameter.Withf("a schema can't be used with a tool named %q", toolkit.RespondName)
			}
		}
		tools = append(tools, respond)
	}
	return tools, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// newRespond returns the tool a model calls to reply with JSON which conforms
// to the request's schema, or nil when the reply is text
func newRespond(req schema.AskRequest) (*toolkit.Respond, error) {
	if result, err := req.JSONSchema(); err != nil || result == nil {
		return nil, err
	}
	return toolkit.NewRespond(req.Schema)
}

// respondSystemPrompt returns the system prompt, with the instruction to
// reply by calling the respond tool
func respondSystemPrompt(system string) string {
	if system = strings.TrimSpace(system); system == "" {
		return toolkit.RespondPrompt
	}
	return system + "\n\n" + toolkit.RespondPrompt
}

// newReply returns the model's reply as a message, which is kept whole when
// it's a message of the provider schema, by value or pointer, or otherwise
// is its text and tool calls. When it has no content, such as a reasoning
// model's which used its tokens reasoning, it has empty text.
func newReply(reply llm.Message) providerschema.Message {
	var message providerschema.Message
	switch m := reply.(type) {
	case *providerschema.Message:
		if m != nil {
			message = *m
		}
	case providerschema.Message:
		message = m
	case nil:
		// No reply, which has empty text
	default:
		message.Sender = reply.Role()
		if text := reply.Text(); text != "" {
			message.Content = append(message.Content, providerschema.ContentBlock{Text: types.Ptr(text)})
		}
		for _, call := range reply.ToolCalls() {
			message.Content = append(message.Content, providerschema.ContentBlock{ToolCall: &providerschema.ToolCall{ID: call.ID, Name: call.Name, Input: call.Input}})
		}
	}
	if message.Sender == "" {
		message.Sender = "assistant"
	}
	if len(message.Content) == 0 {
		message.Content = []providerschema.ContentBlock{{Text: types.Ptr("")}}
	}
	return message
}

// addUsage returns the tokens, and pages, of both usages
func addUsage(a, b llm.Usage) llm.Usage {
	return llm.Usage{
		InputTokens:     a.InputTokens + b.InputTokens,
		OutputTokens:    a.OutputTokens + b.OutputTokens,
		CachedTokens:    a.CachedTokens + b.CachedTokens,
		ReasoningTokens: a.ReasoningTokens + b.ReasoningTokens,
		Pages:           a.Pages + b.Pages,
	}
}

///////////////////////////////////////////////////////////////////////////////
// REPLY STREAM

// replyStream streams the model's replies, which are separated by a blank
// line, so the thinking, or text, of each reply is a paragraph of its own,
// rather than running into the last
type replyStream struct {
	fn       llm.GeneratorStreamFn
	streamed bool // Something has been streamed
	started  bool // The current reply has streamed something
}

// newReplyStream returns the stream of the replies, which streams to the
// function, or nil when the function is nil
func newReplyStream(fn llm.GeneratorStreamFn) *replyStream {
	if fn == nil {
		return nil
	}
	return &replyStream{fn: fn}
}

// Next returns the function which streams the next reply, or nil when the
// replies aren't streamed
func (s *replyStream) Next() llm.GeneratorStreamFn {
	if s == nil {
		return nil
	}
	s.started = false
	return s.stream
}

// stream streams a piece of the reply, which, when it's the first of a reply
// after one which streamed something, starts with a blank line
func (s *replyStream) stream(what llm.StreamType, text string) {
	if text == "" {
		return
	}
	if !s.started {
		s.started = true
		if s.streamed {
			text = "\n\n" + text
		}
	}
	s.streamed = true
	s.fn(what, text)
}
