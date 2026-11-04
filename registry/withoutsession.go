package registry

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

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

// respondAttempts is how many times a model is asked for its reply, unless
// the max_iterations option says otherwise, when the reply has a schema
const respondAttempts = 3

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Ask returns the reply of the request's model to its messages, which
// include earlier turns of the conversation, as the agent loop does, but
// which stops when the model calls tools, whose calls are in its reply, for
// the caller to run, rather than run. The model replies after the system
// prompt, with the options, and can call the tools, which are got from the
// registry's toolkit. When the request has a schema, the model replies by
// calling the respond tool, whose result is the reply's result: a model
// which replies with text is asked to call it, and a result which doesn't
// conform to the schema is returned to the model to correct, until it's
// been asked respondAttempts times, or as many times as the max_iterations
// option. The messages are compacted, as the agent loop does, to fit the
// model's context, without changing them, and when they don't fit, however
// compacted, the model isn't asked, and the reply is empty, with
// max_context as its result. The reply is streamed to the function as it's
// generated, when it isn't nil, with each of the model's replies separated
// by a blank line. Returns ErrBadParameter when the request isn't valid,
// ErrNotFound when the model, or a tool, isn't found, and the errors of
// WithoutSession.
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
	messages := make([]llm.Message, len(req.Messages))
	for i := range req.Messages {
		messages[i] = &providerschema.Message{MessageMeta: req.Messages[i]}
	}

	// Ask the model, until it replies, with a result which conforms to the
	// schema, when the request has one, or calls tools
	model, loop, hasSchema, err := r.generatorLoop(ctx, req.GeneratorMeta, respondAttempts)
	if err != nil {
		return nil, err
	}
	loop.stopAtTools = true
	h, usage, err := r.agentLoop(ctx, model, messages, nil, loop, stream)
	if err != nil {
		return nil, err
	}
	message := h.added[len(h.added)-1]
	response := &schema.AskResponse{
		Model:   r.Name(model),
		Message: message,
		Schema:  hasSchema,
		Usage:   usage,
	}
	if hasSchema && message.Result == providerschema.ResultStop {
		response.Result = json.RawMessage(message.Text())
	}
	return response, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// askTools returns the tools of the session, from the registry's toolkit.
// When the tool choice is "none", there are no tools, and no tool choice, so
// the provider needn't support it, unless the reply has a schema, which
// needs the respond tool, which the agent loop adds. Any other tool choice
// needs tools to choose from.
func (r *Registry) askTools(ctx context.Context, req schema.GeneratorMeta, hasSchema bool) ([]llm.Tool, error) {
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
	case choice == "none" && hasSchema:
		return nil, llm.ErrBadParameter.With("a schema needs tools, so the tool choice can't be none")
	case choice == "none":
		return nil, nil
	case choice != "" && len(tools) == 0 && !hasSchema:
		return nil, llm.ErrBadParameter.With("a tool choice needs tools")
	}
	return tools, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

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
	streamed bool       // Something has been streamed
	started  bool       // The current reply has streamed something
	closed   bool       // Nothing more is streamed
	mu       sync.Mutex // Guards the function, and what's streamed, as tools log to it in parallel, even after they time out
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = false
	return s.stream
}

// Log returns the function which streams a line a tool logs, which isn't
// part of a reply, so doesn't separate the replies, or nil when the replies
// aren't streamed. Tools which run in parallel can log at once, and a tool
// which doesn't return when it times out can log as the next reply is
// streamed, so the lines, and the replies, are streamed in turn.
func (s *replyStream) Log() func(string) {
	if s == nil {
		return nil
	}
	return func(text string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if !s.closed {
			s.fn(llm.StreamLog, text)
		}
	}
}

// close stops streaming, once what's being streamed has been, so a tool which
// doesn't return when it times out, and logs, doesn't stream once the
// replies are done
func (s *replyStream) close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
}

// stream streams a piece of the reply, which, when it's the first of a reply
// after one which streamed something, starts with a blank line
func (s *replyStream) stream(what llm.StreamType, text string) {
	if text == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
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
