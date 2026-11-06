package registry

import (
	"context"
	"errors"
	"slices"
	"strings"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// DefaultEventAgent is the agent which runs an event, unless another is
// given
const DefaultEventAgent = "event"

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// RunEvent runs the event with the identifier, such as a heartbeat which
// fired, with the request's agent, or the event agent, and its input, from
// the event's session, whose identifier is the event's, so an event which
// happens again, such as a heartbeat on a schedule, has the one session, and
// returns the session, and what the agent did, and replied, which is added to
// the session, with its usage, so it has each run, which is shown as it's
// replying, as the agent runs. The session is
// created, when there's none, with the model of the session the event is
// for, and takes that model again, when it's changed, each time it's run,
// and an agent which has no model, such as the event agent, is run with it,
// and with the tools of the session the event is for, apart from those of
// the namespaces it's without, such as heartbeat, so an event doesn't
// schedule events.
// Returns ErrBadParameter when the identifier, or the session, is the nil
// UUID, or they're the same, or the session has no model, ErrNotFound when
// there's no session, or agent, ErrServiceUnavailable when the registry has
// no session store, or agent store, and the errors of RunAgent.
func (r *Registry) RunEvent(ctx context.Context, id uuid.UUID, req schema.RunEventRequest, stream llm.GeneratorStreamFn) (_ *schema.RunEventResponse, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.RunEvent",
		attribute.String("arg.id", id.String()),
		attribute.String("arg.req", types.Stringify(req)),
	)
	defer func() { endSpan(err) }()

	session, parent, err := r.eventSession(ctx, id, req)
	if err != nil {
		return nil, err
	}
	tools, err := r.eventTools(ctx, parent.Tools, req.Without)
	if err != nil {
		return nil, err
	}
	agent := req.Agent
	if agent == "" {
		agent = DefaultEventAgent
	}

	// The session is replying, until the agent has replied, and what it did
	// is stored, or not
	r.lifecycle.RLock()
	replied := r.replying(session.ID)
	r.lifecycle.RUnlock()
	defer replied()
	run, err := r.RunAgent(withoutNamespaces(ctx, req.Without), agent, schema.RunAgentRequest{Input: req.Input, Session: session.ID, Tools: tools}, stream)
	if err != nil {
		return nil, err
	}

	// What it did, which is its prompt, the tools it called, and its reply,
	// is added to the session, with its usage
	if session, err = r.addToSession(ctx, session.ID, run.SessionResponse); err != nil {
		return nil, err
	}
	return &schema.RunEventResponse{Session: *session, Run: *run}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// addToSession adds the messages of the response to the session with the
// identifier, with its usage, and returns the session, without its
// messages. A session which is changed as they're added, such as by another
// reply, is read again, and they're added to it, at most a few times.
func (r *Registry) addToSession(ctx context.Context, id uuid.UUID, response schema.SessionResponse) (*schema.Session, error) {
	const tries = 3
	for try := 1; ; try++ {
		session, err := r.sessionStore.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		session.Messages = append(session.Messages, response.Messages...)
		session.Usage = schema.SessionUsage{
			Usage:    addUsage(session.Usage.Usage, response.Usage.Usage),
			Input:    response.Usage.Input,
			Output:   response.Usage.Output,
			Overhead: response.Usage.Overhead,
		}
		stored, err := r.sessionStore.Put(ctx, session)
		if errors.Is(err, llm.ErrConflict) && try < tries {
			continue
		} else if err != nil {
			return nil, err
		}
		r.sessionChanged(eventschema.EventUpdate, stored)
		return r.withoutMessages(stored), nil
	}
}

// withoutNamespacesKey is the key of the namespaces whose tools the agents
// run in a context aren't given, in it
type withoutNamespacesKey struct{}

// withoutNamespaces returns the context, in which the agents which run, such
// as those an event's agent calls as tools, aren't given the tools of the
// namespaces, as well as those which were already excluded
func withoutNamespaces(ctx context.Context, namespaces []string) context.Context {
	if len(namespaces) == 0 {
		return ctx
	}
	without := append(slices.Clone(namespacesWithout(ctx)), namespaces...)
	return context.WithValue(ctx, withoutNamespacesKey{}, without)
}

// namespacesWithout returns the namespaces whose tools the agents run in the
// context aren't given
func namespacesWithout(ctx context.Context) []string {
	without, _ := ctx.Value(withoutNamespacesKey{}).([]string)
	return without
}

// eventTools returns the tools, apart from those of the namespaces, by their
// names, or patterns, where all the tools, "*", are the patterns of every
// other namespace, such as "weatherapi:*", or no tools, when there are none
func (r *Registry) eventTools(ctx context.Context, tools, without []string) ([]string, error) {
	result := []string{}
	if len(tools) == 0 {
		return result, nil
	}
	excluded := func(namespace string) bool {
		return slices.Contains(without, namespace)
	}
	for _, name := range tools {
		if name == "*" {
			if r.toolkit == nil {
				continue
			}
			namespaces, err := r.toolkit.ListNamespaces(ctx)
			if err != nil {
				return nil, err
			}
			for _, namespace := range namespaces.Body {
				if !excluded(namespace.Name) && namespace.Tools > 0 {
					result = append(result, namespace.Name+toolschema.ToolSeparator+"*")
				}
			}
		} else if namespace, _, ok := strings.Cut(name, toolschema.ToolSeparator); !ok || !excluded(namespace) {
			result = append(result, name)
		}
	}
	slices.Sort(result)
	return slices.Compact(result), nil
}

// eventSession returns the session of the event, which is created, when
// there's none, with the model of the session the event is for, or takes it,
// when it's changed, and the session it's for
func (r *Registry) eventSession(ctx context.Context, id uuid.UUID, req schema.RunEventRequest) (_ *schema.Session, _ *schema.Session, err error) {
	switch {
	case r.sessionStore == nil:
		return nil, nil, errNoSessionStore
	case id == uuid.Nil():
		return nil, nil, llm.ErrBadParameter.With("missing event")
	case req.Session == uuid.Nil():
		return nil, nil, llm.ErrBadParameter.With("missing session")
	case id == req.Session:
		return nil, nil, llm.ErrBadParameter.With("an event's session isn't the session it's for")
	}

	// The model of the session the event is for
	stored, err := r.sessionStore.Get(ctx, req.Session)
	if err != nil {
		return nil, nil, err
	} else if types.Value(stored.Model) == "" {
		return nil, nil, llm.ErrBadParameter.Withf("session %q has no model", req.Session)
	}
	parent := r.withoutMessages(stored)

	// The event's session, which is created, when there's none, or takes the
	// model, when it's changed. One created at the same time, such as by
	// another server, is the one returned.
	session, err := r.sessionStore.Get(ctx, id)
	if errors.Is(err, llm.ErrNotFound) {
		created, err := r.sessionStore.Create(ctx, id, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: parent.Model}})
		if errors.Is(err, llm.ErrConflict) {
			session, err = r.sessionStore.Get(ctx, id)
		} else if err == nil {
			r.sessionChanged(eventschema.EventCreate, created)
			return r.withoutMessages(created), parent, nil
		}
		if err != nil {
			return nil, nil, err
		}
	} else if err != nil {
		return nil, nil, err
	}
	if types.Value(session.Model) != types.Value(parent.Model) {
		updated, err := r.UpdateSession(ctx, id, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: parent.Model}})
		return updated, parent, err
	}
	return r.withoutMessages(session), parent, nil
}
