package registry

import (
	"context"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
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
// another server made
type AgentStore interface {
	// Create creates an agent with the definition, and returns it, with its
	// serial, and when it was created. Returns ErrConflict when there's an
	// agent with its name.
	Create(context.Context, schema.AgentMeta) (schema.Agent, error)

	// Get returns the agent with the name. Returns ErrNotFound when there's
	// none.
	Get(ctx context.Context, name string) (schema.Agent, error)

	// List returns the agents, in order of their names, from the request's
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

// errNoAgentStore is the error of an agent's method, when the registry has
// no agent store
var errNoAgentStore = llm.ErrServiceUnavailable.With("the registry has no agent store")

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateAgent creates an agent with the definition, in the registry's agent
// store, and returns it. Its name must be an identifier, such as segment,
// which refers to it. Its input, when it has one, must be a JSON schema of an
// object, and its prompt a Go template, which is executed with the input. Its
// model, when it has one, is the registry's, by its canonical name, and its
// tools are the toolkit's, by their canonical names, as a session's are. It
// needn't have a model, as one can be given when it's run. Returns
// ErrBadParameter when the definition isn't valid, ErrNotFound when the
// model isn't the registry's, or a tool isn't the toolkit's, ErrConflict
// when there's an agent with the name, or a name is ambiguous, and
// ErrServiceUnavailable when the registry has no agent store.
func (r *Registry) CreateAgent(ctx context.Context, meta schema.AgentMeta) (_ *schema.Agent, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.CreateAgent",
		attribute.String("arg.meta", meta.String()),
	)
	defer func() { endSpan(err) }()

	if r.agentStore == nil {
		return nil, errNoAgentStore
	} else if !validAgentName(meta.Name) {
		return nil, llm.ErrBadParameter.Withf("invalid agent name %q, which must be an identifier, without two underscores, or starting or ending with one", meta.Name)
	} else if meta, err = r.validAgentMeta(ctx, meta, meta); err != nil {
		return nil, err
	}
	agent, err := r.agentStore.Create(ctx, meta)
	if err != nil {
		return nil, err
	}
	return &agent, nil
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

// ListAgents returns the agents, in order of their names, as the request
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
// changed. The definition, as changed, must be valid, as CreateAgent says,
// and a model, or tools, which are changed are the registry's, by their
// canonical names. Returns the agent. Returns ErrBadParameter when the
// update changes nothing, or the definition, as changed, isn't valid,
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
	} else if meta.IsZero() {
		return nil, llm.ErrBadParameter.With("nothing to update")
	}
	agent, err := r.agentStore.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	if agent.AgentMeta, err = r.validAgentMeta(ctx, agent.AgentMeta.Merge(meta), meta); err != nil {
		return nil, err
	}
	if agent, err = r.agentStore.Put(ctx, agent); err != nil {
		return nil, err
	}
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
	agent, err := r.agentStore.Get(ctx, name)
	if err != nil {
		return nil, err
	} else if err := r.agentStore.Delete(ctx, name, agent.Serial); err != nil {
		return nil, err
	}
	return &agent, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// validAgentName returns true when the name is an identifier, which is also
// a tool's name in the agent namespace, so the name a model calls it by,
// such as agent__segment, is separated where it is: it doesn't have two
// underscores, and doesn't start or end with one
func validAgentName(name string) bool {
	return types.IsIdentifier(name) && toolschema.IsModelToolPart(name)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// validAgentMeta returns the definition, with its model and tools, when the
// change sets them, by their canonical names, or ErrBadParameter when it
// isn't valid: its input isn't a JSON schema of an object, its prompt isn't
// a Go template, or its settings aren't valid, as a session's mustn't be
func (r *Registry) validAgentMeta(ctx context.Context, meta, change schema.AgentMeta) (schema.AgentMeta, error) {
	var err error
	if _, err = meta.InputSchema(); err != nil {
		return meta, err
	} else if _, err = meta.Template(); err != nil {
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
