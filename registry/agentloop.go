package registry

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// agentLoop is how the agent loop asks a model, from its options
type agentLoop struct {
	maxIterations uint             // The most times the model is asked
	toolTimeout   time.Duration    // The most time a tool has to return
	contextWindow uint             // The context window option, in tokens, or zero when it isn't set
	respond       *toolkit.Respond // The tool the model replies with, when the reply has a schema
	tools         []llm.Tool       // The tools, and the respond tool
	opts          []llm.Opt        // The options passed to the model
	stopAtTools   bool             // Whether the loop stops when the model calls tools, other than the respond tool, for the caller to run
	session       uuid.UUID        // The session the loop runs in, which its tools run in, or the nil UUID
	maxDepth      uint             // The most agents which run in one another, from the loop, or zero when it isn't set
	zone          *time.Location   // The time zone the times of the person's messages are told in, or nil, when they aren't
}

// timeFormat is how the time a message was sent is told, with the day of
// the week, as models can't always work it out, and its offset
const timeFormat = "Monday 2006-01-02T15:04-07:00"

// respondAlone is the respond tool when the model called other tools too,
// whose results it hasn't seen, so it isn't run, and its result is an error
type respondAlone struct{ toolkit.Base }

var _ llm.Tool = respondAlone{}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// DefaultMaxIterations is how many times the agent loop asks a model for
	// its reply, when it isn't told how many
	DefaultMaxIterations = 10

	// DefaultToolTimeout is the most time a tool the agent loop runs has to
	// return, when it isn't told how long
	DefaultToolTimeout = 30 * time.Second
)

var (
	// errRespondAlone is the result of the respond tool, when the model
	// called other tools too, whose results it hasn't seen
	errRespondAlone = errors.New("not run, as other tools were called too: call the " + toolkit.RespondName + " tool on its own, once you have their results")
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// AgentLoop asks the model for its reply to the messages, runs the tools it
// calls, and asks it again with their results, until it replies, its reply
// is truncated as it reached the most tokens it can generate, or it's been
// asked as many times as the max_iterations option, or DefaultMaxIterations
// times when it isn't set.
//
// When the json_schema option is set, the model replies by calling the
// respond tool, which is added to the tools, with a result which conforms
// to the schema, and the system prompt says so. The reply is then the
// result, as JSON text, rather than the call, so each tool call in the
// conversation has its result. A model which replies with text is asked to
// call the tool, and a result which doesn't conform, or a call with other
// tools, is returned to the model as an error, each of which counts as an
// iteration, until it's been asked as many times as it can be.
//
// Each tool has the tool_timeout option to return, or DefaultToolTimeout
// when it isn't set, after which its result is that it timed out. The
// results are truncated, as toolkit.TruncateResult does, as the model's
// provider measures them, or as they're estimated, so the model's context,
// which is the smaller of its input token limit and the context_window
// option, is at most two thirds full with them, or to DefaultToolResultLimit
// tokens each, when the model's context isn't known.
//
// The max_iterations, tool_timeout and json_schema options are the loop's,
// so aren't passed to the model, unlike the other options. Returns the messages added
// to the conversation: each of the model's replies, followed by the results
// of the tools it called, or the request to call the respond tool, except
// the last, which is the model's last reply, whose result is why the loop
// stopped: stop, or another reason the model stopped; or max_tokens,
// blocked, error or max_iterations, when the tools it called aren't run, as
// its input may be incomplete, or it didn't finish, or it's been asked as
// many times as it can be, so are left out of it. So each tool call has its
// result, and the messages can be sent to a model again,
// as the rest of a conversation. When the reply has a schema, its result is
// stop only when it's the result. A tool which the model calls, and which
// isn't one of the tools, or fails, has its error returned to the model as
// its result. The replies are streamed to the function as they're
// generated, when it isn't nil, each separated by a blank line, until the
// loop returns, when nothing more is streamed, though a tool which didn't
// return in time may still log. Returns ErrBadParameter when a message is
// nil, an option isn't valid, or a tool is named as the
// respond tool when the reply has a schema, the errors of WithoutSession,
// and the context's error when it's done while a tool runs.
func (r *Registry) AgentLoop(ctx context.Context, model llm.Model, messages []llm.Message, tools []llm.Tool, stream llm.GeneratorStreamFn, opts ...llm.Opt) (_ []providerschema.Message, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.AgentLoop",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	// The system prompts of the tools, from the registry's toolkit
	prompts, err := r.toolPrompts(ctx, tools)
	if err != nil {
		return nil, llm.Usage{}, err
	}

	// The most iterations, the respond tool, and the options passed to the
	// model, with the system prompt
	loop, err := newAgentLoop(tools, prompts, opts...)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	loop.zone = r.zoneFor(tools)
	h, usage, err := r.agentLoop(ctx, model, messages, nil, loop, stream)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	return h.added, usage, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// zoneFor returns the time zone in which the model is told when each of
// the person's messages was sent, when it can call a tool of one of the
// namespaces which WithTimes sets, such as memory, or nil, when it can't
func (r *Registry) zoneFor(tools []llm.Tool) *time.Location {
	for _, tool := range tools {
		named, ok := tool.(interface{ CanonicalName() string })
		if !ok {
			continue
		}
		namespace, _, _ := strings.Cut(named.CanonicalName(), toolschema.ToolSeparator)
		if !slices.Contains(r.timesFor, namespace) {
			continue
		}
		if r.timesZone != nil {
			if zone := r.timesZone(); zone != nil {
				return zone
			}
		}
		return time.Local
	}
	return nil
}

// toolPrompts returns the system prompts of the namespaces of the tools
// which are the registry's toolkit's, by their canonical names, each read
// once, in the order of the tools. A tool which isn't the toolkit's has none.
func (r *Registry) toolPrompts(ctx context.Context, tools []llm.Tool) ([]string, error) {
	if r.toolkit == nil {
		return nil, nil
	}

	// The namespaces of the toolkit's tools, once each
	var namespaces []string
	for _, tool := range tools {
		named, ok := tool.(interface{ CanonicalName() string })
		if !ok {
			continue
		}
		canonical := named.CanonicalName()
		if _, err := r.toolkit.ToolNames(ctx, canonical); errors.Is(err, llm.ErrNotFound) {
			continue
		} else if err != nil {
			return nil, err
		}
		namespace, _, _ := strings.Cut(canonical, toolschema.ToolSeparator)
		if all := namespace + toolschema.ToolSeparator + "*"; !slices.Contains(namespaces, all) {
			namespaces = append(namespaces, all)
		}
	}

	// Their system prompts, read once
	if len(namespaces) == 0 {
		return nil, nil
	}
	return r.toolkit.SystemPrompts(ctx, namespaces...)
}

// agentLoop asks the model as AgentLoop does, as the loop says, which, when
// it stops at tool calls, returns the model's reply when it calls tools,
// other than the respond tool, for the caller to run, without the call of
// the respond tool, if it called it too, as it's asked to call it on its
// own, once it has their results. The fresh messages follow the messages,
// and are the first the loop adds, so they're returned as they're sent.
// Returns the conversation, with the messages the loop added, and the tokens
// of each, and the usage.
func (r *Registry) agentLoop(ctx context.Context, model llm.Model, messages []llm.Message, fresh []providerschema.Message, loop *agentLoop, stream llm.GeneratorStreamFn) (*history, llm.Usage, error) {
	ctx = loop.depthContext(ctx)
	start := time.Now()

	// The conversation, which the replies and tool results are added to,
	// without changing the caller's messages, measured as the model's
	// provider measures it
	measure := func(ctx context.Context, text string) (uint, error) {
		return r.Measure(ctx, model, text)
	}
	overhead, err := loop.overhead(ctx, measure)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	h, err := newHistory(ctx, messages, fresh, overhead, measure, r.attachmentText(loop.session))
	if err != nil {
		return nil, llm.Usage{}, err
	}
	var usage llm.Usage
	replies := newReplyStream(stream)
	defer replies.close()
	for iteration := uint(1); ; iteration++ {
		step, stepUsage, done, err := r.iterate(ctx, model, loop, h, iteration, replies)
		if err != nil {
			return nil, llm.Usage{}, err
		}
		// The tokens of the model's reply, and of the tools it called, such
		// as agents, which ask models, though only the model's reply is of
		// the conversation, which the history measures
		usage = addUsage(addUsage(usage, stepUsage), toolUsage(step))
		if err := h.add(ctx, step, stepUsage); err != nil {
			return nil, llm.Usage{}, err
		} else if done {
			// How long it took, rather than the sum of what ran at once, as
			// tools do
			usage.Duration = time.Since(start)
			return h, usage, nil
		}
	}
}

// iterate compacts the conversation, to fit the model's context, and asks
// the model once, in a span of its own, and returns the messages the
// iteration adds to the conversation, its usage, and whether the loop is
// done: the model's reply, which is the last when the loop is done,
// followed by the results of the tools it called, or the request to call
// the respond tool, when it isn't. The reply is empty, with max_context as
// its result, when the conversation doesn't fit, so the model isn't asked.
// The span has how the conversation was compacted, the reply's result, the
// tools it called, and the tokens it used.
func (r *Registry) iterate(ctx context.Context, model llm.Model, loop *agentLoop, h *history, iteration uint, replies *replyStream) (_ []providerschema.Message, _ llm.Usage, _ bool, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.AgentLoop.Iteration",
		attribute.Int("arg.iteration", int(iteration)),
	)
	defer func() { endSpan(err) }()

	// The session of each tool, whose logger streams what it logs, and adds
	// it to the iteration's span, when the registry traces
	var span trace.Span
	if r.tracer != nil {
		span = trace.SpanFromContext(ctx)
	}
	session := func(name string) toolkit.Session {
		return toolkit.NewSession(name, loop.session, replies.Tool(), span)
	}

	// Compact the conversation, to fit the model's context, unless it can't
	fits, err := r.compact(ctx, model, loop, h, session, span)
	if err != nil {
		return nil, llm.Usage{}, false, err
	} else if !fits {
		message := newReply(nil)
		message.Result = providerschema.ResultMaxContext
		r.traceIteration(ctx, message.Result, nil, llm.Usage{})
		return []providerschema.Message{message}, llm.Usage{}, true, nil
	}

	// The conversation, with the content of the attachments which refer to
	// resources, which is read as it's sent, rather than kept with it
	conversation, err := r.withContent(ctx, loop.session, h.messages(), true, model)
	if err != nil {
		return nil, llm.Usage{}, false, err
	}
	if loop.zone != nil {
		conversation = withTimes(conversation, loop.zone)
	}
	if span != nil {
		span.SetAttributes(attribute.Int("arg.messages", len(conversation)))
	}
	reply, usage, err := r.WithoutSession(ctx, model, conversation, loop.tools, replies.Next(), loop.opts...)
	if err != nil {
		return nil, llm.Usage{}, false, err
	}
	h.sent(usage)
	message := newReply(reply)
	if message, err = r.keepReplyResources(ctx, loop.session, message); err != nil {
		return nil, llm.Usage{}, false, err
	}
	calls := message.ToolCalls()
	defer func() { r.traceIteration(ctx, message.Result, calls, usage) }()

	// The model's reply was truncated, so the tools it called, whose input
	// may be incomplete, are left out of it, as they can't be run, or be
	// sent to a model again. A reply which was blocked, or ended in an
	// error, didn't finish either, so its tools aren't run, and the loop
	// stops, even when the reply has a schema.
	switch message.Result {
	case providerschema.ResultMaxTokens, providerschema.ResultBlocked, providerschema.ResultError:
		return []providerschema.Message{withoutToolCalls(message)}, usage, true, nil
	}

	// The model called the respond tool on its own, so its result is
	// validated once: the reply is the result, when it conforms to the
	// schema, or the model is told why it doesn't, as the call's result,
	// and asked again, unless it's been asked as many times as it can be
	if call, ok := loop.respondCall(calls); ok {
		result, err := loop.respond.Run(ctx, call.Input)
		if err == nil {
			message = withResult(message, result.(json.RawMessage))
			return []providerschema.Message{message}, usage, true, nil
		}
		if iteration >= loop.maxIterations {
			return []providerschema.Message{maxIterations(message)}, usage, true, nil
		}
		results := newToolResults(1)
		results.Content = append(results.Content, providerschema.NewToolError(call.ID, call.Name, err))
		return []providerschema.Message{message, results}, usage, false, nil
	}

	// The model has replied, when the reply has no schema
	if len(calls) == 0 && loop.respond == nil {
		if message.Result == "" {
			message.Result = providerschema.ResultStop
		}
		return []providerschema.Message{message}, usage, true, nil
	}

	// The model replied with text, when the reply has a schema, which is the
	// reply, when it's what the respond tool would have been called with, or
	// its result, which conforms to the schema, as some models reply with
	// it, rather than calling the tool, so it isn't lost by asking again,
	// even when it's been asked as many times as it can be
	if len(calls) == 0 {
		if result, ok := loop.textResult(ctx, message.Text()); ok {
			return []providerschema.Message{withResult(message, result)}, usage, true, nil
		}
	}

	// The model called tools, which the caller runs
	if loop.stopAtTools && slices.ContainsFunc(calls, func(call llm.ToolCall) bool { return call.Name != toolkit.RespondName || loop.respond == nil }) {
		if loop.respond != nil {
			message = withoutRespond(message)
		}
		if message.Result == "" {
			message.Result = providerschema.ResultToolCall
		}
		return []providerschema.Message{message}, usage, true, nil
	}

	// The model has been asked as many times as it can be, so the tools it
	// called aren't run
	if iteration >= loop.maxIterations {
		return []providerschema.Message{maxIterations(message)}, usage, true, nil
	}

	// Or, otherwise, it's asked to call the respond tool
	if len(calls) == 0 {
		nudge := providerschema.Message{MessageMeta: providerschema.MessageMeta{
			Sender:  "user",
			Content: []providerschema.ContentBlock{{Text: types.Ptr(toolkit.RespondNudge)}},
		}}
		return []providerschema.Message{message, nudge}, usage, false, nil
	}

	// Run the tools, each in its session, whose resources, which they
	// return, are kept, as resources of the loop's session, and ask the model
	// again with their results, which are compacted, if they don't fit
	toolCtx := ctx
	if store := r.toolStore(loop.session); store != nil {
		toolCtx = toolkit.WithStore(toolkit.WithResolver(ctx, r.toolResolver(loop.session)), store)
	}
	results, err := loop.runTools(toolCtx, calls, func(call llm.ToolCall) toolkit.Session {
		return session(call.Name)
	})
	if err != nil {
		return nil, llm.Usage{}, false, err
	}
	if used := toolUsage([]providerschema.Message{results}); !used.IsZero() && r.tracer != nil {
		trace.SpanFromContext(ctx).SetAttributes(
			attribute.Int("tools.input_tokens", int(used.InputTokens)),
			attribute.Int("tools.output_tokens", int(used.OutputTokens)),
		)
	}
	return []providerschema.Message{message, results}, usage, false, nil
}

// traceIteration sets the attributes of the iteration's span, which is the
// context's: the reply's result, when it has one, the names of the tools it
// called, and the tokens it used: its input and output tokens, and of those,
// the cached and reasoning tokens, when there are any, when the registry
// traces, so another span isn't changed
func (r *Registry) traceIteration(ctx context.Context, result providerschema.ResultType, calls []llm.ToolCall, usage llm.Usage) {
	if r.tracer == nil {
		return
	}
	names := make([]string, 0, len(calls))
	for _, call := range calls {
		names = append(names, call.Name)
	}
	attrs := []attribute.KeyValue{
		attribute.StringSlice("tool_calls", names),
		attribute.Int("input_tokens", int(usage.InputTokens)),
		attribute.Int("output_tokens", int(usage.OutputTokens)),
	}
	if usage.CachedTokens > 0 {
		attrs = append(attrs, attribute.Int("cached_tokens", int(usage.CachedTokens)))
	}
	if usage.ReasoningTokens > 0 {
		attrs = append(attrs, attribute.Int("reasoning_tokens", int(usage.ReasoningTokens)))
	}
	if result != "" {
		attrs = append(attrs, attribute.String("result", result.String()))
	}
	trace.SpanFromContext(ctx).SetAttributes(attrs...)
}

// toolUsage returns the tokens which the tools whose results are in the
// messages used, such as agents, which ask models
func toolUsage(messages []providerschema.Message) llm.Usage {
	var usage llm.Usage
	for _, message := range messages {
		for _, block := range message.Content {
			if block.ToolResult != nil && block.ToolResult.Usage != nil {
				usage = usage.Add(*block.ToolResult.Usage)
			}
		}
	}
	return usage
}

// textResult returns the result of a reply with text, when the reply has a
// schema, and the text, apart from a fence around it, such as ```json, is
// what the respond tool would have been called with, or its result, which
// conforms to the schema
func (l *agentLoop) textResult(ctx context.Context, text string) (json.RawMessage, bool) {
	text = withoutFence(strings.TrimSpace(text))
	if l.respond == nil || text == "" || !json.Valid([]byte(text)) {
		return nil, false
	}

	// As the respond tool's input, with the result, and then as the result
	// itself
	for _, input := range []string{text, `{"result":` + text + `}`} {
		if result, err := l.respond.Run(ctx, json.RawMessage(input)); err == nil {
			return result.(json.RawMessage), true
		}
	}
	return nil, false
}

// respondCall returns the call of the respond tool, when the reply has a
// schema, and it's the only tool the model called
func (l *agentLoop) respondCall(calls []llm.ToolCall) (llm.ToolCall, bool) {
	if l.respond == nil || len(calls) != 1 || calls[0].Name != toolkit.RespondName {
		return llm.ToolCall{}, false
	}
	return calls[0], true
}

// runTools runs the tools the model called, in parallel, each in the
// session the function returns for its call, and returns the message of
// their results, in the order of the calls, as toolkit.RunTools does, but
// the respond tool, when other tools were called too, has an error as its
// result, which asks the model to call it on its own
func (l *agentLoop) runTools(ctx context.Context, calls []llm.ToolCall, session func(llm.ToolCall) toolkit.Session) (providerschema.Message, error) {
	tools := l.tools
	if l.respond != nil && len(calls) > 1 {
		tools = slices.Clone(tools)
		tools[slices.Index(tools, llm.Tool(l.respond))] = respondAlone{}
	}
	content, err := toolkit.RunTools(ctx, tools, calls, l.toolTimeout, session)
	if err != nil {
		return providerschema.Message{}, err
	}
	results := newToolResults(0)
	results.Content = content
	return results, nil
}

// depthContext returns the context, in which the agents its tools run are
// at most as deep as the loop says, or those which run it say, or otherwise
// DefaultMaxDepth, which is recorded, so the agents its tools run can't
// raise it
func (l *agentLoop) depthContext(ctx context.Context) context.Context {
	if l.maxDepth > 0 {
		return withMaxDepth(ctx, l.maxDepth)
	} else if agentChainOf(ctx).maxDepth == 0 {
		return withMaxDepth(ctx, DefaultMaxDepth)
	}
	return ctx
}

// window returns the model's context, in tokens, which is the smaller of
// its input token limit, and the context window option, and whether it's
// known
func (l *agentLoop) window(model llm.Model) (uint, bool) {
	window := l.contextWindow
	if limit := model.InputTokenLimit(); limit != nil && *limit > 0 && (window == 0 || *limit < window) {
		window = *limit
	}
	return window, window > 0
}

// overhead returns an estimate of the tokens of what's sent to the model
// with the messages, as the function measures them: the system prompt, and
// the tools, as their names, descriptions and input schemas
func (l *agentLoop) overhead(ctx context.Context, measure toolkit.MeasureFn) (uint, error) {
	o, err := llm.ApplyOpts(l.opts...)
	if err != nil {
		return 0, err
	}
	system, _ := llm.GetOpt[string](o, llm.SystemPromptKey)
	type tool struct {
		Name        string             `json:"name"`
		Description string             `json:"description"`
		InputSchema *jsonschema.Schema `json:"input_schema,omitempty"`
	}
	tools := make([]tool, 0, len(l.tools))
	for _, t := range l.tools {
		tools = append(tools, tool{t.Name(), t.Description(), t.InputSchema()})
	}
	data, err := json.Marshal(tools)
	if err != nil {
		return 0, err
	}
	return measure(ctx, system+string(data))
}

// compact compacts the conversation, so it's at most contextFill of the
// model's context, when it's more, in turn, until it is: by removing the
// results of the tools of earlier turns, oldest first; then by truncating
// the results of the tools of the latest turn, each to its share of what's
// left, but no fewer than minToolResultLimit tokens; and then by dropping
// earlier turns, oldest first, until only the system messages, the latest
// user message, and the latest turn are left. When the model's context
// isn't known, the results of the tools of the latest turn are truncated to
// DefaultToolResultLimit tokens each, instead. Returns false when the
// conversation doesn't fit in the model's whole context, so can't be sent.
// The span, unless it's nil, has how the conversation was compacted.
func (r *Registry) compact(ctx context.Context, model llm.Model, loop *agentLoop, h *history, session func(string) toolkit.Session, span trace.Span) (bool, error) {
	var done compacted
	defer func() {
		if span == nil {
			return
		}
		for key, value := range map[string]int{"removed_results": done.removed, "truncated_results": done.truncated, "dropped_messages": done.dropped} {
			if value > 0 {
				span.SetAttributes(attribute.Int(key, value))
			}
		}
	}()

	// The model's context isn't known
	window, known := loop.window(model)
	if !known {
		var err error
		done.truncated, err = h.truncateResults(ctx, DefaultToolResultLimit, session)
		return true, err
	}

	// Remove the results of earlier turns, then truncate those of the latest,
	// and then drop earlier turns, until it fits
	budget := uint(float64(window) * contextFill)
	if h.size() <= budget {
		return true, nil
	}
	var err error
	if done.removed, err = h.removeResults(ctx, budget); err != nil {
		return false, err
	}
	if h.size() > budget {
		if count, tokens := h.latestResults(); count > 0 {
			available := int(budget) - int(h.size()-tokens)
			if done.truncated, err = h.truncateResults(ctx, uint(max(available/count, minToolResultLimit)), session); err != nil {
				return false, err
			}
		}
	}
	if h.size() > budget {
		done.dropped = h.dropTurns(budget)
	}
	return h.size() <= window, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// newAgentLoop returns how the agent loop asks a model with the tools, from
// the options: the most iterations, which is the max_iterations option, or
// DefaultMaxIterations when it isn't set; the respond tool, when the
// json_schema option is set, which is added to the tools; the system prompt,
// which is the system_prompt option, followed by the system prompts of the
// tools, unless the tool choice is none, and the instruction to call the
// respond tool, when there is one;
// and the other options, which are passed to the model
func newAgentLoop(tools []llm.Tool, prompts []string, opts ...llm.Opt) (*agentLoop, error) {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}
	loop := &agentLoop{maxIterations: DefaultMaxIterations, toolTimeout: DefaultToolTimeout, tools: tools}

	// The context window, which is passed to the model, and limits its
	// context, when it's set
	if o.Has(llm.ContextWindowKey) {
		if loop.contextWindow, err = llm.GetOpt[uint](o, llm.ContextWindowKey); err != nil {
			return nil, llm.ErrBadParameter.Withf("invalid option %q: %v", llm.ContextWindowKey, err)
		}
	}

	// The most iterations
	if o.Has(llm.MaxIterationsKey) {
		if loop.maxIterations, err = llm.GetOpt[uint](o, llm.MaxIterationsKey); err != nil {
			return nil, llm.ErrBadParameter.Withf("invalid option %q: %v", llm.MaxIterationsKey, err)
		} else if loop.maxIterations == 0 {
			return nil, llm.ErrBadParameter.Withf("%s must be one or more", llm.MaxIterationsKey)
		}
		o.Delete(llm.MaxIterationsKey)
	}

	// The most agents which run in one another, which the agents the loop's
	// tools run have, rather than the model
	if o.Has(llm.MaxDepthKey) {
		if loop.maxDepth, err = llm.GetOpt[uint](o, llm.MaxDepthKey); err != nil {
			return nil, llm.ErrBadParameter.Withf("invalid option %q: %v", llm.MaxDepthKey, err)
		} else if loop.maxDepth == 0 {
			return nil, llm.ErrBadParameter.Withf("%s must be one or more", llm.MaxDepthKey)
		}
		o.Delete(llm.MaxDepthKey)
	}

	// The most time a tool has to return, which is a duration, or a string
	// such as "30s", rather than a number, which would be nanoseconds
	if o.Has(llm.ToolTimeoutKey) {
		if loop.toolTimeout, err = toolTimeout(o); err != nil {
			return nil, err
		}
		o.Delete(llm.ToolTimeoutKey)
	}

	// The reply's schema, which needs the respond tool, whose name no other
	// tool can have, which the model can call, unless the tool choice is none
	// The tools' system prompts, unless the tool choice is none, so the model
	// has no tools, and isn't told how to use them
	if choice, _ := llm.GetOpt[string](o, llm.ToolChoiceKey); choice == "none" {
		prompts = nil
	}
	system, _ := llm.GetOpt[string](o, llm.SystemPromptKey)
	system = systemPrompt(system, prompts)
	if !o.Has(llm.JSONSchemaKey) {
		loop.opts = []llm.Opt{o.Opt()}
		if system != "" {
			loop.opts = append(loop.opts, llm.WithSystemPrompt(system))
		}
		return loop, nil
	}
	schema, err := llm.GetOpt[json.RawMessage](o, llm.JSONSchemaKey)
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid option %q: %v", llm.JSONSchemaKey, err)
	} else if loop.respond, err = toolkit.NewRespond(schema); err != nil {
		return nil, err
	}
	o.Delete(llm.JSONSchemaKey)
	if choice, _ := llm.GetOpt[string](o, llm.ToolChoiceKey); choice == "none" {
		return nil, llm.ErrBadParameter.With("a schema needs tools, so the tool choice can't be none")
	}
	for _, tool := range tools {
		if tool.Name() == toolkit.RespondName {
			return nil, llm.ErrBadParameter.Withf("a schema can't be used with a tool named %q", toolkit.RespondName)
		}
	}
	loop.tools = append(slices.Clip(tools), loop.respond)

	// The system prompt, with the instruction to call the respond tool
	loop.opts = []llm.Opt{o.Opt(), llm.WithSystemPrompt(respondSystemPrompt(system))}
	return loop, nil
}

// toolTimeout returns the tool_timeout option, which is a duration, or a
// string such as "30s", which is more than zero
func toolTimeout(o *llm.Opts) (time.Duration, error) {
	timeout, ok := o.Get(llm.ToolTimeoutKey).(time.Duration)
	if !ok {
		text, err := llm.GetOpt[string](o, llm.ToolTimeoutKey)
		if err != nil {
			return 0, llm.ErrBadParameter.Withf("invalid option %q: a duration, such as \"30s\"", llm.ToolTimeoutKey)
		} else if timeout, err = time.ParseDuration(text); err != nil {
			return 0, llm.ErrBadParameter.Withf("invalid option %q: %v", llm.ToolTimeoutKey, err)
		}
	}
	if timeout <= 0 {
		return 0, llm.ErrBadParameter.Withf("%s must be more than zero", llm.ToolTimeoutKey)
	}
	return timeout, nil
}

// withResult returns the model's reply, which called the respond tool with
// the result, as the result, as JSON text, with its thinking, rather than the
// call, or its text, so the call needn't have a result
func withResult(message providerschema.Message, result json.RawMessage) providerschema.Message {
	content := make([]providerschema.ContentBlock, 0, len(message.Content))
	for _, block := range message.Content {
		if block.Thinking != nil {
			content = append(content, block)
		}
	}
	message.Content = append(content, providerschema.ContentBlock{Text: types.Ptr(string(result))})
	message.Result = providerschema.ResultStop
	return message
}

// withoutFence returns the text without a fence of Markdown around it, such
// as ```json and ```, as some models put around JSON, or the text, when it
// has none
func withoutFence(text string) string {
	rest, ok := strings.CutPrefix(text, "```")
	if !ok {
		return text
	}
	rest, ok = strings.CutSuffix(strings.TrimRight(rest, " \t\r\n"), "```")
	if !ok {
		return text
	}

	// The fence's language, such as json, is on its first line
	if tag, body, ok := strings.Cut(rest, "\n"); ok && !strings.ContainsAny(tag, "{[\"") {
		rest = body
	}
	return strings.TrimSpace(rest)
}

// withoutRespond returns the message without its calls of the respond tool
func withoutRespond(message providerschema.Message) providerschema.Message {
	message.Content = slices.DeleteFunc(slices.Clone(message.Content), func(block providerschema.ContentBlock) bool {
		return block.ToolCall != nil && block.ToolCall.Name == toolkit.RespondName
	})
	return message
}

// withoutToolCalls returns the message without its tool calls, which, when
// that leaves it without content, has empty text
func withoutToolCalls(message providerschema.Message) providerschema.Message {
	message.Content = slices.DeleteFunc(slices.Clone(message.Content), func(block providerschema.ContentBlock) bool {
		return block.ToolCall != nil
	})
	if len(message.Content) == 0 {
		message.Content = []providerschema.ContentBlock{{Text: types.Ptr("")}}
	}
	return message
}

// maxIterations returns the model's last reply, when the agent loop has
// asked it as many times as it can, without the tools it called, as they
// aren't run, so needn't have results, whose result is max_iterations
func maxIterations(message providerschema.Message) providerschema.Message {
	message = withoutToolCalls(message)
	message.Result = providerschema.ResultMaxIterations
	return message
}

func (respondAlone) Name() string                                      { return toolkit.RespondName }
func (respondAlone) Description() string                               { return "" }
func (respondAlone) InputSchema() *jsonschema.Schema                   { return nil }
func (respondAlone) Run(context.Context, json.RawMessage) (any, error) { return nil, errRespondAlone }

// newToolResults returns the message of the results of tools, with room for
// as many as there are calls
func newToolResults(calls int) providerschema.Message {
	return providerschema.Message{MessageMeta: providerschema.MessageMeta{
		Sender:  "tool",
		Content: make([]providerschema.ContentBlock, 0, calls),
	}}
}
