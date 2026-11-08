package registry

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
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

// DefaultMaxEvents is the most events which run at once, unless
// WithMaxEvents sets it
const DefaultMaxEvents = 2

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
// for, and the event's title, and takes them again, when they've changed,
// each time it's run,
// and an agent which has no model, such as the event agent, is run with it,
// and with the tools of the session the event is for, apart from those of
// the namespaces it's without, such as heartbeat, so an event doesn't
// schedule events.
// Returns ErrBadParameter when the identifier, or the session, is the nil
// UUID, or they're the same, or the session has no model, ErrNotFound when
// there's no session, or agent, ErrConflict when the event's session was
// changed as the agent ran, such as when its messages were forgotten, so
// what it did isn't stored, ErrServiceUnavailable when the registry has no
// session store, or agent store, and the errors of RunAgent.
func (r *Registry) RunEvent(ctx context.Context, id uuid.UUID, req schema.RunEventRequest, stream llm.GeneratorStreamFn) (_ *schema.RunEventResponse, err error) {
	return r.runEvent(ctx, id, req, stream, uuid.Nil())
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// runEvent runs the event, as RunEvent does, and stores its outcome, as a
// new outcome, or, when it's given, as the outcome which was scheduled,
// which is updated
func (r *Registry) runEvent(ctx context.Context, id uuid.UUID, req schema.RunEventRequest, stream llm.GeneratorStreamFn, scheduled uuid.UUID) (_ *schema.RunEventResponse, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.RunEvent",
		attribute.String("arg.id", id.String()),
		attribute.String("arg.req", types.Stringify(req)),
	)
	defer func() { endSpan(err) }()

	// It runs once there's a slot for it, as at most so many events run at
	// once, or not, when the context is cancelled as it waits
	select {
	case r.eventSlots <- struct{}{}:
		defer func() { <-r.eventSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

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
	// is stored, or not, unless it's deleted before it's replying, so its
	// status isn't set after it's deleted. It's as it is now, so what the
	// agent did is added to it as it is, and isn't, when it changes as it
	// runs, such as when its messages are forgotten, as a chat's reply isn't.
	r.lifecycle.RLock()
	stored, err := r.sessionStore.Get(ctx, session.ID)
	if err != nil {
		r.lifecycle.RUnlock()
		return nil, err
	}
	replied := r.replying(session.ID)
	r.lifecycle.RUnlock()
	defer replied()
	run, err := r.RunAgent(withoutNamespaces(ctx, req.Without), agent, schema.RunAgentRequest{Input: req.Input, Session: session.ID, Tools: tools}, stream)
	if err != nil {
		return nil, err
	}

	// What it did, which is its prompt, the tools it called, and its reply,
	// is added to the session, with its usage. Returns ErrConflict when the
	// session was changed as it ran.
	stored.Messages = append(stored.Messages, run.Messages...)
	stored.Usage = schema.SessionUsage{
		Usage:    addUsage(stored.Usage.Usage, run.Usage.Usage),
		Input:    run.Usage.Input,
		Output:   run.Usage.Output,
		Overhead: run.Usage.Overhead,
	}
	if stored, err = r.sessionStore.Put(ctx, stored); err != nil {
		return nil, err
	}
	r.sessionChanged(eventschema.EventUpdate, stored)
	session = r.withoutMessages(stored)

	// Its outcome, which is the agent's message, and how important it is, is
	// stored, as ignored, or pending, until it's delivered
	message, importance, err := eventReply(run.Reply)
	if err != nil {
		return nil, err
	}
	var event *schema.Event
	if scheduled != uuid.Nil() {
		event, err = r.updateEvent(ctx, scheduled, func(event *schema.Event) {
			event.Run, event.Message, event.Importance, event.Status = session.ID, message, importance, importance.Status()
		})
	} else {
		event, err = r.createEvent(ctx, schema.EventMeta{
			Run:        session.ID,
			Session:    req.Session,
			Title:      req.Title,
			Message:    message,
			Importance: importance,
			Status:     importance.Status(),
			Scheduled:  req.Scheduled,
			Fired:      req.Fired,
		})
	}
	if err != nil {
		return nil, err
	}

	// An outcome which is to be told to the person is delivered to the
	// session it's for, once it isn't replying, in the background, so the
	// run doesn't wait for it
	if event != nil && event.Status == schema.EventPending {
		go func(event schema.Event, run []providerschema.Message) {
			_ = r.deliver(context.WithoutCancel(ctx), event, run)
		}(*event, run.Messages)
	}
	return &schema.RunEventResponse{Session: *session, Run: *run, Event: event}, nil
}

// eventReply returns the message, and how important it is, of the reply of
// the agent which ran an event, which is its message, and importance, as the
// event agent replies, or, of an agent which replies with text, or with
// JSON without a message, such as another agent, its text, or its JSON, which
// is worth telling the person. Returns ErrBadParameter when its importance
// isn't one.
func eventReply(reply json.RawMessage) (string, schema.EventImportance, error) {
	var text string
	if err := json.Unmarshal(reply, &text); err == nil {
		return text, schema.ImportanceNotify, nil
	}
	var result struct {
		Message    *string `json:"message"`
		Importance string  `json:"importance"`
	}
	if err := json.Unmarshal(reply, &result); err != nil || result.Message == nil {
		return string(reply), schema.ImportanceNotify, nil
	}
	importance, err := schema.ParseEventImportance(result.Importance)
	if err != nil {
		return "", "", err
	}
	return *result.Message, importance, nil
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

	// The event's session, which is created, when there's none, with the
	// event's title, or takes the model, and the title, when they've changed,
	// such as when a heartbeat is renamed. One created at the same time, such
	// as by another server, is the one returned.
	title := schema.SessionMeta{}.Merge(schema.SessionMeta{Title: types.Ptr(req.Title)}).Title
	session, err := r.sessionStore.Get(ctx, id)
	if errors.Is(err, llm.ErrNotFound) {
		created, err := r.sessionStore.Create(ctx, id, schema.SessionMeta{Title: title, GeneratorMeta: schema.GeneratorMeta{Model: parent.Model}, Event: true})
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
	var change schema.SessionMeta
	if types.Value(session.Model) != types.Value(parent.Model) {
		change.Model = parent.Model
	}
	if title != nil && types.Value(session.Title) != *title {
		change.Title = title
	}
	if !change.IsZero() {
		updated, err := r.UpdateSession(ctx, id, change)
		return updated, parent, err
	}
	return r.withoutMessages(session), parent, nil
}
