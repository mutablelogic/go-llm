package registry

import (
	"context"
	"errors"
	"fmt"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithSession asks the session's model, after its system prompt, with its
// options, tools and the schema of the reply, for its reply to the
// conversation, which is the session's messages, each with its tokens, when
// it has them, and the messages, as the agent loop does, which runs the
// tools the model calls. The messages can be any messages, which are kept as
// messages of the provider schema, with the data of their attachments read.
// Returns the messages to add to the session: the messages, and those the
// loop added, the last of which is the model's reply, each with its tokens;
// the tokens of the conversation, with them, of the messages sent to the
// model, of its replies, and of what's sent with them; and the usage. With a
// schema, the model's reply, when its result is stop, is JSON which
// conforms to it. The session isn't changed. The reply is streamed to the
// function as it's generated, when it isn't nil. Returns ErrBadParameter
// when the session, or the messages, aren't valid, ErrNotFound when the
// model, or a tool, isn't found, and the errors of AgentLoop.
func (r *Registry) WithSession(ctx context.Context, session schema.Session, messages []llm.Message, stream llm.GeneratorStreamFn) (_ *schema.SessionResponse, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.WithSession",
		attribute.String("arg.session", session.SessionMeta.String()),
		attribute.Int("arg.session.messages", len(session.Messages)),
		attribute.Int("arg.messages", len(messages)),
	)
	defer func() { endSpan(err) }()
	meta, earlier := session.GeneratorMeta, session.Messages

	// The model, and the conversation, of the session's messages and the
	// messages
	if strings.TrimSpace(types.Value(meta.Model)) == "" {
		return nil, llm.ErrBadParameter.With("missing model")
	} else if len(messages) == 0 {
		return nil, llm.ErrBadParameter.With("missing messages")
	}
	added := make([]providerschema.Message, 0, len(messages))
	for i, message := range messages {
		message, err := sessionMessage(ctx, message)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("message %d: %v", i, err)
		}

		// Attachments which refer to resources are the session's, or those
		// on their own, which are linked to it
		if message, err = r.attach(ctx, session.ID, message); err != nil {
			return nil, err
		}
		added = append(added, message)
	}
	conversation := make([]llm.Message, 0, len(earlier))
	for i := range earlier {
		conversation = append(conversation, &earlier[i])
	}

	// Ask the model, until it replies, running the tools it calls
	model, loop, _, err := r.generatorLoop(ctx, meta, DefaultMaxIterations)
	if err != nil {
		return nil, err
	}
	loop.session = session.ID
	h, usage, err := r.agentLoop(ctx, model, conversation, added, loop, stream)
	if err != nil {
		return nil, err
	}

	// The messages to add, which are the messages, as they were sent, and
	// those the loop added, each with its tokens
	response := &schema.SessionResponse{
		Messages: h.added,
		Usage:    schema.SessionUsage{Usage: usage, Overhead: h.overhead},
	}

	// The tokens of the conversation, with the messages to add, of its
	// replies, and the rest
	for i, message := range earlier {
		countTokens(response, message.Role(), h.measured[i])
	}
	for _, message := range response.Messages {
		countTokens(response, message.Role(), uint(message.Tokens))
	}

	return response, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// generatorLoop returns the model of how it's asked, and how the agent loop
// asks it, with the system prompt, followed by those of its tools, the
// options, the tools, from the registry's toolkit, and the schema of the
// reply, as the respond tool, when there is one, whose instruction follows
// them, and whether there is one. The model is asked at most the
// iterations, unless the max_iterations option says otherwise.
func (r *Registry) generatorLoop(ctx context.Context, meta schema.GeneratorMeta, iterations uint) (llm.Model, *agentLoop, bool, error) {
	result, err := meta.JSONSchema()
	if err != nil {
		return nil, nil, false, err
	}
	hasSchema := result != nil
	tools, prompts, err := r.askTools(ctx, meta, hasSchema)
	if err != nil {
		return nil, nil, false, err
	}
	opts, err := meta.Opts(types.Value(meta.System))
	if err != nil {
		return nil, nil, false, err
	}
	opts = append([]llm.Opt{llm.WithMaxIterations(iterations)}, opts...)
	if hasSchema {
		opts = append(opts, llm.WithJSONSchema(meta.Schema))
	}
	model, err := r.GetModel(ctx, strings.TrimSpace(types.Value(meta.Model)))
	if err != nil {
		return nil, nil, false, err
	}
	loop, err := newAgentLoop(tools, prompts, opts...)
	if err != nil {
		return nil, nil, false, err
	}
	return model, loop, hasSchema, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// sessionMessage returns the message as a message of the provider schema,
// which is a copy of it, when it's one, or its meta, or otherwise its role,
// text, attachments, whose data is read, tool calls and tool results, so it
// can be kept, as it's sent. Returns an error when it's nil, or an
// attachment can't be read.
func sessionMessage(ctx context.Context, message llm.Message) (providerschema.Message, error) {
	if isNil(message) {
		return providerschema.Message{}, errors.New("missing message")
	} else if message, ok := schemaMessage(message); ok {
		return *message, nil
	}
	var content []providerschema.ContentBlock
	if text := message.Text(); text != "" {
		content = append(content, providerschema.ContentBlock{Text: types.Ptr(text)})
	}
	for _, resource := range message.Resources() {
		data, err := llm.ReadAll(ctx, resource)
		if err != nil {
			return providerschema.Message{}, fmt.Errorf("%s: %w", resource.Name(), err)
		}
		content = append(content, providerschema.ContentBlock{Resource: &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{
			ContentType: resource.Type(),
			Data:        data,
		}}})
	}
	for _, call := range message.ToolCalls() {
		content = append(content, providerschema.ContentBlock{ToolCall: &providerschema.ToolCall{ID: call.ID, Name: call.Name, Input: call.Input}})
	}
	for _, result := range message.ToolResults() {
		content = append(content, providerschema.ContentBlock{ToolResult: &providerschema.ToolResult{ID: result.ID, Name: result.Name, Content: result.Content, IsError: result.IsError}})
	}
	if len(content) == 0 {
		content = []providerschema.ContentBlock{{Text: types.Ptr("")}}
	}
	return providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: message.Role(), Content: content}}, nil
}

// systemPrompt returns the system prompt, followed by the system prompts of
// the tools, each separated by a blank line, without those which are empty
func systemPrompt(system string, prompts []string) string {
	parts := make([]string, 0, len(prompts)+1)
	for _, part := range append([]string{system}, prompts...) {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "\n\n")
}

// countTokens adds the tokens of a message, by its role, to those of the
// model's replies, or of the messages sent to it
func countTokens(response *schema.SessionResponse, role string, tokens uint) {
	if role == "assistant" {
		response.Usage.Output += tokens
	} else {
		response.Usage.Input += tokens
	}
}
