package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime"
	"slices"
	"strings"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// AgentStore has agents, each by its name, with its serial, which the store
// sets, and changes each time the agent is stored, so a change is made to
// the agent as it was last seen, rather than a newer one, such as one which
// another server made. An agent's serial is never one which an agent with
// its name had before, including one which was deleted, and created again,
// so a change to the agent as it was isn't made to the new one.
type AgentStore interface {
	// Create creates an agent with the definition, and returns it, with its
	// serial, and when it was created. Returns ErrConflict when there's an
	// agent with its name.
	Create(context.Context, schema.AgentMeta) (schema.Agent, error)

	// Get returns the agent with the name. Returns ErrNotFound when there's
	// none.
	Get(ctx context.Context, name string) (schema.Agent, error)

	// List returns the agents, in the store's order, from the request's
	// offset, and at most its limit of them, and how many there are
	List(context.Context, schema.AgentListRequest) (schema.AgentList, error)

	// Put replaces the agent with its name, when its serial is that agent's,
	// and returns it, with its new serial, and when it was modified. Returns
	// ErrNotFound when there's none, and ErrConflict when its serial isn't
	// the agent's, as it was changed since it was seen.
	Put(context.Context, schema.Agent) (schema.Agent, error)

	// Delete deletes the agent with the name, when the serial is its serial,
	// or does nothing when there's none. Returns ErrConflict when the serial
	// isn't its serial, as it was changed since it was seen.
	Delete(ctx context.Context, name string, serial uint64) error
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// DefaultMaxDepth is the most agents which run in one another, as an agent
// calls another, or itself, as a tool, when the max_depth option of the
// session, or agent, which runs them, doesn't say
const DefaultMaxDepth = 3

// errNoAgentStore is the error of an agent's method, when the registry has
// no agent store
var errNoAgentStore = llm.ErrServiceUnavailable.With("the registry has no agent store")

// errBuiltinAgent returns the error of changing, or deleting, a built-in
// agent, which the server creates as it is each time it starts
func errBuiltinAgent(name string) error {
	return llm.ErrForbidden.Withf("agent %q is built in, so it can't be changed, or deleted, but it can be copied under another name", name)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateAgent creates an agent with the definition, in the registry's agent
// store, or replaces the agent with its name, with the definition, as it is,
// rather than merging them, when it's changed, and returns it, as it's
// stored, or the agent as it is, when it hasn't changed, so creating agents
// each time a server starts, such as its examples, changes only those which
// have, and whether it was created, rather than replaced, or kept. Its
// name must be an identifier, such as segment,
// which refers to it. Its input, when it has one, must be a JSON schema of an
// object, and its prompt a Go template, which is executed with the input. Its
// model, when it has one, is the registry's, by its canonical name, and its
// tools are the toolkit's, by their canonical names, as a session's are. It
// needn't have a model, as one can be given when it's run. Returns
// ErrBadParameter when the definition isn't valid, ErrNotFound when the
// model isn't the registry's, or a tool isn't the toolkit's, ErrConflict
// when the agent was changed as it was replaced, or a name is ambiguous, and
// ErrServiceUnavailable when the registry has no agent store.
func (r *Registry) CreateAgent(ctx context.Context, meta schema.AgentMeta) (_ *schema.Agent, created bool, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.CreateAgent",
		attribute.String("arg.meta", meta.String()),
	)
	defer func() { endSpan(err) }()

	if r.agentStore == nil {
		return nil, false, errNoAgentStore
	} else if !validAgentName(meta.Name) {
		return nil, false, llm.ErrBadParameter.Withf("invalid agent name %q, which must be an identifier, without two underscores, or starting or ending with one", meta.Name)
	} else if meta, err = r.validAgentMeta(ctx, meta, meta); err != nil {
		return nil, false, err
	}
	r.agentsMu.Lock()
	defer r.agentsMu.Unlock()

	// The agent with the name, which is replaced, when it's changed, or
	// it's created
	existing, err := r.agentStore.Get(ctx, meta.Name)
	switch {
	case errors.Is(err, llm.ErrNotFound):
		existing, err = r.agentStore.Create(ctx, meta)
		created = true
	case err != nil:
		return nil, false, err
	case types.Value(existing.Builtin) && !types.Value(meta.Builtin):
		return nil, false, errBuiltinAgent(meta.Name)
	case sameAgentMeta(existing.AgentMeta, meta):
		return &existing, false, nil
	default:
		existing.AgentMeta = meta
		existing, err = r.agentStore.Put(ctx, existing)
	}
	if err != nil {
		return nil, false, err
	}
	r.agentChanged(existing)
	return &existing, created, nil
}

// sameAgentMeta returns true when the definitions are the same, as JSON
func sameAgentMeta(a, b schema.AgentMeta) bool {
	x, err := json.Marshal(a)
	if err != nil {
		return false
	}
	y, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return bytes.Equal(x, y)
}

// GetAgent returns the agent with the name. Returns ErrNotFound when there's
// none, and ErrServiceUnavailable when the registry has no agent store.
func (r *Registry) GetAgent(ctx context.Context, name string) (_ *schema.Agent, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.GetAgent",
		attribute.String("arg.name", name),
	)
	defer func() { endSpan(err) }()

	if r.agentStore == nil {
		return nil, errNoAgentStore
	}
	agent, err := r.agentStore.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// ListAgents returns the agents, in the store's order, as the request
// selects. Returns ErrServiceUnavailable when the registry has no agent
// store.
func (r *Registry) ListAgents(ctx context.Context, req schema.AgentListRequest) (_ *schema.AgentList, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.ListAgents",
		attribute.String("arg.req", req.String()),
	)
	defer func() { endSpan(err) }()

	if r.agentStore == nil {
		return nil, errNoAgentStore
	}
	list, err := r.agentStore.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return &list, nil
}

// UpdateAgent changes the definition of the agent with the name, as
// AgentMeta.Merge does: what the update sets is changed, its options are set,
// or removed, when they're null, and the rest is kept. Its name isn't
// changed: the update has no name, or the agent's, such as when it's the
// agent's whole definition. The definition, as changed, must be valid, as CreateAgent says,
// and a model, or tools, which are changed are the registry's, by their
// canonical names. Returns the agent. Returns ErrBadParameter when the
// update has another name, or changes nothing, or the definition, as changed, isn't valid,
// ErrNotFound when there's no agent, or the model, or a tool, isn't the
// registry's, ErrConflict when it was changed as it was updated, or a name is
// ambiguous, and ErrServiceUnavailable when the registry has no agent store.
func (r *Registry) UpdateAgent(ctx context.Context, name string, meta schema.AgentMeta) (_ *schema.Agent, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.UpdateAgent",
		attribute.String("arg.name", name),
		attribute.String("arg.meta", meta.String()),
	)
	defer func() { endSpan(err) }()

	if r.agentStore == nil {
		return nil, errNoAgentStore
	} else if meta.Name != "" && meta.Name != name {
		return nil, llm.ErrBadParameter.Withf("agent %q can't be renamed %q", name, meta.Name)
	} else if meta.IsZero() {
		return nil, llm.ErrBadParameter.With("nothing to update")
	}
	r.agentsMu.Lock()
	defer r.agentsMu.Unlock()
	agent, err := r.agentStore.Get(ctx, name)
	if err != nil {
		return nil, err
	} else if types.Value(agent.Builtin) {
		return nil, errBuiltinAgent(name)
	}
	if agent.AgentMeta, err = r.validAgentMeta(ctx, agent.AgentMeta.Merge(meta), meta); err != nil {
		return nil, err
	}
	if agent, err = r.agentStore.Put(ctx, agent); err != nil {
		return nil, err
	}
	r.agentChanged(agent)
	return &agent, nil
}

// DeleteAgent deletes the agent with the name, and returns it, as it was.
// Returns ErrNotFound when there's none, ErrConflict when it was changed as
// it was deleted, and ErrServiceUnavailable when the registry has no agent
// store.
func (r *Registry) DeleteAgent(ctx context.Context, name string) (_ *schema.Agent, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.DeleteAgent",
		attribute.String("arg.name", name),
	)
	defer func() { endSpan(err) }()

	if r.agentStore == nil {
		return nil, errNoAgentStore
	}
	r.agentsMu.Lock()
	defer r.agentsMu.Unlock()
	agent, err := r.agentStore.Get(ctx, name)
	if err != nil {
		return nil, err
	} else if types.Value(agent.Builtin) {
		return nil, errBuiltinAgent(name)
	} else if err := r.agentStore.Delete(ctx, name, agent.Serial); err != nil {
		return nil, err
	}
	if r.agents != nil {
		r.agents.remove(name)
	}
	return &agent, nil
}

// RunAgent runs the agent with the name: the request's input is validated
// against its input schema, and its prompt is executed with it, as the
// message to its model, which is its own, or, when it has none, the model of
// the session it's run from, which is asked as its settings say, without
// its history, and runs the tools it calls, until it replies. Its reply is
// streamed to the function as it's generated, when it isn't nil. Returns
// what the agent did: the messages it added, the tokens used, and its reply,
// as JSON, which is its text, or what it replied, when it has a schema.
//
// The agents which are running, as an agent calls another, or itself, as a
// tool, are in the context, with the most which can run in one another,
// which is the max_depth option of the session, or agent, which runs them,
// or the least of them, or DefaultMaxDepth, so an agent isn't run deeper,
// as agents could call one another without end.
//
// Returns ErrNotFound when there's no agent
// with the name, or session, ErrForbidden when it would run deeper than the
// most agents which can run in one another, ErrBadParameter when its input
// isn't valid, or its prompt can't be executed with it, or it has no model,
// nor has the session it's run from, and ErrServiceUnavailable when the
// registry has no agent store, and the errors of WithSession.
func (r *Registry) RunAgent(ctx context.Context, name string, req schema.RunAgentRequest, stream llm.GeneratorStreamFn) (_ *schema.RunAgentResponse, err error) {
	return r.runAgent(ctx, name, req, stream, false)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// runAgent runs the agent, as RunAgent does, or, when a model calls it as a
// tool, returns ErrNotFound when it's internal, as only the server runs it,
// such as when it was made internal while the model had it as a tool
func (r *Registry) runAgent(ctx context.Context, name string, req schema.RunAgentRequest, stream llm.GeneratorStreamFn, asTool bool) (_ *schema.RunAgentResponse, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.RunAgent",
		attribute.String("arg.name", name),
		attribute.String("arg.req", req.String()),
	)
	defer func() { endSpan(err) }()

	// The agent, which isn't run deeper than the most agents which can run
	// in one another
	chain := agentChainOf(ctx)
	if r.agentStore == nil {
		return nil, errNoAgentStore
	} else if depth := uint(len(chain.running)) + 1; depth > chain.depth() {
		return nil, llm.ErrForbidden.Withf("agent %q can't run in %s, as at most %d agents run in one another, which max_depth sets", name, strings.Join(chain.running, " → "), chain.depth())
	}
	agent, err := r.agentStore.Get(ctx, name)
	if err != nil {
		return nil, err
	} else if asTool && types.Value(agent.Internal) {
		return nil, llm.ErrNotFound.Withf("agent %q isn't a tool", name)
	}

	// The message to its model, which is its prompt, executed with the input,
	// which is validated, with the text of the files its input refers to, in
	// the session it's run from, or on their own
	message, err := agent.MessageWith(req.Input, r.agentReader(ctx, req.Session))
	if err != nil {
		return nil, err
	}

	// Its model, or the model of the session it's run from
	meta := agent.GeneratorMeta
	if meta.Model == nil && req.Session != uuid.Nil() {
		if r.sessionStore == nil {
			return nil, errNoSessionStore
		}
		session, err := r.sessionStore.Get(ctx, req.Session)
		if err != nil {
			return nil, err
		}
		meta.Model = session.Model
	}
	if meta.Model == nil {
		return nil, llm.ErrBadParameter.Withf("agent %q has no model, nor has the session it's run from", name)
	}

	// The tools it's run with, instead of its own, when they're given, apart
	// from those of the namespaces it's run without, such as an event's,
	// which an agent it calls is run without too
	if req.Tools != nil {
		if meta.Tools, err = r.toolNames(ctx, req.Tools); err != nil {
			return nil, err
		}
	}
	if without := namespacesWithout(ctx); len(without) > 0 && len(meta.Tools) > 0 {
		if meta.Tools, err = r.eventTools(ctx, meta.Tools, without); err != nil {
			return nil, err
		}
	}

	// The tools it calls run as it runs, so it isn't run again by them
	ctx = withRunningAgent(ctx, name)

	// Ask its model, with its settings, in the session it's run from,
	// without its history, so the agents its tools run are run from it too,
	// and which runs the tools it calls, until it replies
	session := schema.Session{ID: req.Session, SessionMeta: schema.SessionMeta{GeneratorMeta: meta}}
	prompt := &providerschema.Message{MessageMeta: providerschema.MessageMeta{
		Sender:  "user",
		Content: []providerschema.ContentBlock{{Text: types.Ptr(message)}},
	}}
	response, err := r.WithSession(ctx, session, []llm.Message{prompt}, stream)
	if err != nil {
		return nil, err
	}

	// What it did, and its reply
	replySchema, err := meta.JSONSchema()
	if err != nil {
		return nil, err
	}
	reply, err := agentReply(response.Messages, replySchema != nil)
	if err != nil {
		return nil, err
	}
	return &schema.RunAgentResponse{Name: agent.Name, Serial: agent.Serial, SessionResponse: *response, Reply: reply}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// agentReader returns the function which reads the text of a file, by its
// URI, resource:///<hash>, which an agent's input refers to, in the session
// it's run from, or on its own, as it is, up to MaxAttachmentsSize, or
// returns ErrBadParameter when it isn't text, such as an image, or audio,
// which a tool reads first
func (r *Registry) agentReader(ctx context.Context, session uuid.UUID) schema.ReadFunc {
	reader := r.newContentReader(session)
	reader.fallback = true
	return func(uri string) (string, error) {
		data, contentType, _, err := reader.read(ctx, uri)
		if err != nil {
			return "", err
		}
		if !providerschema.IsText(data, contentType) {
			tool := "a tool which reads it"
			switch media, _, _ := mime.ParseMediaType(contentType); {
			case strings.HasPrefix(media, "image/"), media == "application/pdf":
				tool = "llm:ocr"
			case strings.HasPrefix(media, "audio/"), strings.HasPrefix(media, "video/"):
				tool = "llm:transcribe"
			}
			return "", llm.ErrBadParameter.Withf("%s is %s, which isn't text, so give it to %s, and its text, or the URI of its text, to the agent", uri, contentType, tool)
		}
		return string(data), nil
	}
}

// agentFinished returns nil when the agent's reply, which is the last of the
// messages, is complete, or an error, which says how it stopped, when it
// isn't, such as when it was cut short, or the agent loop stopped it
func agentFinished(name string, messages []providerschema.Message) error {
	if len(messages) == 0 {
		return llm.ErrInternalServerError.Withf("agent %q didn't reply", name)
	}
	switch result := messages[len(messages)-1].Result; result {
	case "", providerschema.ResultStop:
		return nil
	case providerschema.ResultMaxTokens:
		return llm.ErrMaxTokens.Withf("agent %q didn't finish its reply", name)
	case providerschema.ResultBlocked:
		return llm.ErrRefusal.Withf("agent %q's reply was blocked", name)
	case providerschema.ResultMaxContext:
		return llm.ErrBadParameter.Withf("agent %q wasn't asked, as its input is too long for its model's context", name)
	default:
		return llm.ErrInternalServerError.Withf("agent %q didn't finish its reply, as it stopped with %s", name, result)
	}
}

// agentReply returns the agent's reply, which is the last of the messages,
// as JSON: what it replied, when it has a schema, and the reply is complete
// JSON, or otherwise its text, as a JSON string
func agentReply(messages []providerschema.Message, hasSchema bool) (json.RawMessage, error) {
	if len(messages) == 0 {
		return json.RawMessage(`""`), nil
	}
	reply := messages[len(messages)-1]
	text := reply.Text()
	if hasSchema && reply.Result == providerschema.ResultStop && json.Valid([]byte(text)) {
		return json.RawMessage(text), nil
	}
	return json.Marshal(text)
}

// agentChain is the agents which are running, in a context, in the order
// they were run, as an agent calls another, or itself, as a tool, and the
// most which can run in one another, or zero, when it isn't set
type agentChain struct {
	running  []string
	maxDepth uint
}

// agentChainKey is the key of the agents which are running, in a context
type agentChainKey struct{}

// agentChainOf returns the agents which are running, in the context, and
// the most which can run in one another
func agentChainOf(ctx context.Context) agentChain {
	chain, _ := ctx.Value(agentChainKey{}).(agentChain)
	return chain
}

// depth returns the most agents which can run in one another
func (c agentChain) depth() uint {
	if c.maxDepth == 0 {
		return DefaultMaxDepth
	}
	return c.maxDepth
}

// withRunningAgent returns the context, with the agent with the name
// running, after those which are
func withRunningAgent(ctx context.Context, name string) context.Context {
	chain := agentChainOf(ctx)
	chain.running = append(slices.Clip(chain.running), name)
	return context.WithValue(ctx, agentChainKey{}, chain)
}

// withMaxDepth returns the context, with at most the agents running in one
// another, or fewer, when it says so already, so an agent can't run deeper
// than what runs it says
func withMaxDepth(ctx context.Context, maxDepth uint) context.Context {
	chain := agentChainOf(ctx)
	if chain.maxDepth == 0 || maxDepth < chain.maxDepth {
		chain.maxDepth = maxDepth
	}
	return context.WithValue(ctx, agentChainKey{}, chain)
}

// validAgentName returns true when the name is an identifier, which is also
// a tool's name in the agent namespace, so the name a model calls it by,
// such as agent__segment, is separated where it is: it doesn't have two
// underscores, and doesn't start or end with one
func validAgentName(name string) bool {
	return types.IsIdentifier(name) && toolschema.IsModelToolPart(name)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// agentChanged makes the agent, as it's stored, a tool, or replaces it, when
// the registry has a toolkit
func (r *Registry) agentChanged(agent schema.Agent) {
	if r.agents != nil {
		r.agents.set(r, agent)
	}
}

// loadAgents makes the stored agents tools, when the registry has a
// toolkit, as they're kept when the server restarts, and returns the error
// listing them
func (r *Registry) loadAgents(ctx context.Context) error {
	if r.agents == nil {
		return nil
	}
	r.agentsMu.Lock()
	defer r.agentsMu.Unlock()
	for offset := uint(0); ; {
		list, err := r.agentStore.List(ctx, schema.AgentListRequest{Offset: offset})
		if err != nil {
			return err
		}
		for _, agent := range list.Body {
			r.agentChanged(agent)
		}
		offset += uint(len(list.Body))
		if len(list.Body) == 0 || offset >= list.Count {
			return nil
		}
	}
}

// validAgentMeta returns the definition, with its model and tools, when the
// change sets them, by their canonical names, or ErrBadParameter when it
// isn't valid: its input isn't a JSON schema of an object, its prompt isn't
// a Go template, its timeout isn't a duration, or its settings aren't valid, as a session's mustn't be
func (r *Registry) validAgentMeta(ctx context.Context, meta, change schema.AgentMeta) (schema.AgentMeta, error) {
	var err error
	if _, err = meta.InputSchema(); err != nil {
		return meta, err
	} else if _, err = meta.Template(); err != nil {
		return meta, err
	} else if _, err = meta.ToolTimeout(); err != nil {
		return meta, err
	} else if err = validGeneratorMeta(meta.GeneratorMeta); err != nil {
		return meta, err
	}
	if change.Model != nil {
		if meta.Model, err = r.canonicalModel(ctx, meta.Model); err != nil {
			return meta, err
		}
	}
	if change.Tools != nil {
		if meta.Tools, err = r.toolNames(ctx, meta.Tools); err != nil {
			return meta, err
		}
	}
	return meta, nil
}
