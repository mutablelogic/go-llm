package registry

import (
	"context"
	"errors"
	"strings"
	"time"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// EventStore has the outcomes of the events which the registry runs, such as
// heartbeats which fire, each with its identifier and serial, which the store
// sets, and changes each time it's stored, so a change is made to the event
// as it was last seen, such as when it's delivered
type EventStore interface {
	// Create creates an event with the outcome, and a new identifier, and
	// returns it, with its serial, and when it was created
	Create(context.Context, schema.EventMeta) (schema.Event, error)

	// Get returns the event with the identifier. Returns ErrNotFound when
	// there's none.
	Get(ctx context.Context, id uuid.UUID) (schema.Event, error)

	// List returns the events which the request selects, the most recently
	// created first, from its offset, and at most its limit of them, and how
	// many it selects
	List(context.Context, schema.EventListRequest) (schema.EventList, error)

	// Put replaces the event with its identifier, when its serial is that
	// event's, and returns it, with its new serial, and when it was
	// modified. Returns ErrNotFound when there's none, and ErrConflict when
	// its serial isn't the event's, as it was changed since it was seen.
	Put(context.Context, schema.Event) (schema.Event, error)

	// Delete deletes the event with the identifier, when the serial is its
	// serial, or does nothing when there's none. Returns ErrConflict when
	// the serial isn't its serial.
	Delete(ctx context.Context, id uuid.UUID, serial uint64) error
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// EventTable is the table of the registry's events of the outcomes of
// events, whose rows are their identifiers, and whose data, unless they
// were deleted, is an EventChange
const EventTable = "event"

// errNoEventStore is returned when the registry has no event store
var errNoEventStore = llm.ErrServiceUnavailable.With("the registry has no event store")

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListEvents returns the outcomes of the events which the request selects,
// those for its session, and of its status, when it has them, the most
// recent first, from its offset, and at most its limit of them, and how many
// it selects. Returns ErrBadParameter when the session isn't a UUID, and
// ErrServiceUnavailable when the registry has no event store.
func (r *Registry) ListEvents(ctx context.Context, req schema.EventListRequest) (_ *schema.EventList, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.ListEvents",
		attribute.String("arg.req", req.String()),
	)
	defer func() { endSpan(err) }()

	// The session, and status, as stores compare them, which is the
	// session's canonical form, and the status without the space around it
	if r.eventStore == nil {
		return nil, errNoEventStore
	} else if session := strings.TrimSpace(req.Session); session != "" {
		id, err := uuid.Parse(session)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("invalid session %q", req.Session)
		}
		req.Session = id.String()
	}
	if strings.TrimSpace(string(req.Status)) != "" {
		status, err := schema.ParseEventStatus(string(req.Status))
		if err != nil {
			return nil, err
		}
		req.Status = status
	}
	list, err := r.eventStore.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return &list, nil
}

// GetEvent returns the outcome of the event with the identifier. Returns
// ErrNotFound when there's none, and ErrServiceUnavailable when the registry
// has no event store.
func (r *Registry) GetEvent(ctx context.Context, id uuid.UUID) (_ *schema.Event, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.GetEvent",
		attribute.String("arg.id", id.String()),
	)
	defer func() { endSpan(err) }()

	if r.eventStore == nil {
		return nil, errNoEventStore
	}
	event, err := r.eventStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// DeleteEvent deletes the outcome of the event with the identifier, and
// returns it, as it was. Returns ErrNotFound when there's none, ErrConflict
// when it was changed as it was deleted, and ErrServiceUnavailable when the
// registry has no event store.
func (r *Registry) DeleteEvent(ctx context.Context, id uuid.UUID) (_ *schema.Event, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.DeleteEvent",
		attribute.String("arg.id", id.String()),
	)
	defer func() { endSpan(err) }()

	if r.eventStore == nil {
		return nil, errNoEventStore
	}
	event, err := r.eventStore.Get(ctx, id)
	if err != nil {
		return nil, err
	} else if err := r.eventStore.Delete(ctx, id, event.Serial); err != nil {
		return nil, err
	}
	r.eventChanged(eventschema.EventDelete, event)
	return &event, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// createEvent stores the outcome of an event, when the registry has an event
// store, and publishes that it was, or returns nil, when it has none
func (r *Registry) createEvent(ctx context.Context, meta schema.EventMeta) (*schema.Event, error) {
	if r.eventStore == nil {
		return nil, nil
	}
	event, err := r.eventStore.Create(ctx, meta)
	if err != nil {
		return nil, err
	}
	r.eventChanged(eventschema.EventCreate, event)
	return &event, nil
}

// eventChanged publishes that the event's outcome was created, updated or
// deleted, with the session it's for, and its serial, and status, unless it
// was deleted
func (r *Registry) eventChanged(action eventschema.EventAction, event schema.Event) {
	data := schema.EventChange{Session: event.Session}
	if action != eventschema.EventDelete {
		data.Serial, data.Status = event.Serial, event.Status
	}
	r.publish(action, EventTable, event.ID.String(), data)
}

///////////////////////////////////////////////////////////////////////////////
// TRIGGER

// EventBuilder returns how an event is run: its agent's input, the
// namespaces of the tools it's run without, and what it's for, and when it
// was due, and fired, for RunEvent
type EventBuilder func(context.Context, schema.EventRequest) (schema.RunEventRequest, error)

// TriggerEvent schedules the event, from a source other than a heartbeat,
// such as a file which changed, or a webhook, to run, in the background, in
// the session of its source, for the session it's for, or a session of its
// own, when it has no source, as RunEvent runs it, and returns its outcome,
// which is scheduled, until it's run, when it's updated, as it's pending,
// or ignored, or failed, when it couldn't be run, with the error as its
// message. An event of a source which is scheduled already, for the
// session, isn't scheduled, so a burst of them runs once, and the outcome
// of that event is returned. Returns ErrBadParameter when it has no session,
// or prompt, ErrNotFound when there's no session, and ErrServiceUnavailable
// when the registry has no event store, in which its outcome is kept, or
// session store.
func (r *Registry) TriggerEvent(ctx context.Context, req schema.EventRequest) (_ *schema.TriggerEventResponse, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.TriggerEvent",
		attribute.String("arg.req", req.String()),
	)
	defer func() { endSpan(err) }()

	req.Title = strings.TrimSpace(req.Title)
	req.Prompt = strings.TrimSpace(req.Prompt)
	req.Source = strings.TrimSpace(req.Source)
	switch {
	case r.eventStore == nil:
		return nil, errNoEventStore
	case req.Session == uuid.Nil():
		return nil, llm.ErrBadParameter.With("missing session")
	case req.Prompt == "":
		return nil, llm.ErrBadParameter.With("missing prompt")
	case req.Title == "":
		req.Title = "Event"
	}
	if r.sessionStore == nil {
		return nil, errNoSessionStore
	} else if _, err := r.sessionStore.Get(ctx, req.Session); err != nil {
		return nil, err
	}
	req.Fired = time.Now().UTC()
	if req.Scheduled.IsZero() || req.Scheduled.After(req.Fired) {
		req.Scheduled = req.Fired
	}

	// How it's run, which is the builder's, or, without one, with the time
	run, err := r.buildEvent(ctx, req)
	if err != nil {
		return nil, err
	}
	run.Session = req.Session
	id := req.RunID()

	// It isn't scheduled, when an event of its source is, for the session,
	// which this registry runs, and is otherwise scheduled, as its outcome,
	// under the lock, so two events at once aren't both scheduled
	r.triggerMu.Lock()
	if req.Source != "" {
		if existing, err := r.scheduledEvent(ctx, id); err != nil {
			r.triggerMu.Unlock()
			return nil, err
		} else if existing != nil {
			r.triggerMu.Unlock()
			return &schema.TriggerEventResponse{Scheduled: false, Event: *existing}, nil
		}
	}
	outcome, err := r.createEvent(ctx, schema.EventMeta{
		Run:       id,
		Session:   req.Session,
		Source:    req.Source,
		Title:     req.Title,
		Status:    schema.EventScheduled,
		Scheduled: req.Scheduled,
		Fired:     req.Fired,
	})
	if err == nil && req.Source != "" {
		r.triggered[id] = outcome.ID
	}
	r.triggerMu.Unlock()
	if err != nil {
		return nil, err
	}

	// It runs in the background, without the request, which ends once it's
	// returned, and its outcome is updated once it's run, or failed, when it
	// couldn't be, when it's no longer scheduled
	go func(ctx context.Context) {
		defer func() {
			r.triggerMu.Lock()
			if r.triggered[id] == outcome.ID {
				delete(r.triggered, id)
			}
			r.triggerMu.Unlock()
		}()
		if _, err := r.runEvent(ctx, id, run, nil, outcome.ID); err != nil {
			_, _ = r.updateEvent(ctx, outcome.ID, func(event *schema.Event) {
				event.Status, event.Message = schema.EventFailed, err.Error()
			})
		}
	}(context.WithoutCancel(ctx))
	return &schema.TriggerEventResponse{Scheduled: true, Event: *outcome}, nil
}

// scheduledEvent returns the outcome of the event which runs in the session
// with the identifier, which this registry has scheduled, and hasn't run,
// or nil, when there's none, such as when the outcome is scheduled, but its
// server stopped before it ran. The caller holds the lock.
func (r *Registry) scheduledEvent(ctx context.Context, run uuid.UUID) (*schema.Event, error) {
	id, ok := r.triggered[run]
	if !ok {
		return nil, nil
	}
	event, err := r.eventStore.Get(ctx, id)
	if errors.Is(err, llm.ErrNotFound) {
		delete(r.triggered, run)
		return nil, nil
	} else if err != nil {
		return nil, err
	} else if event.Status != schema.EventScheduled {
		return nil, nil
	}
	return &event, nil
}

// updateEvent changes the outcome of the event with the identifier, as it
// is in the store, with the function, and stores it, at its serial, and
// publishes that it was, and returns it
func (r *Registry) updateEvent(ctx context.Context, id uuid.UUID, fn func(*schema.Event)) (*schema.Event, error) {
	event, err := r.eventStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	fn(&event)
	if event, err = r.eventStore.Put(ctx, event); err != nil {
		return nil, err
	}
	r.eventChanged(eventschema.EventUpdate, event)
	return &event, nil
}

// buildEvent returns how the event is run, with the registry's builder, or,
// without one, with an input which says only the time, in UTC
func (r *Registry) buildEvent(ctx context.Context, req schema.EventRequest) (schema.RunEventRequest, error) {
	if r.eventBuilder != nil {
		return r.eventBuilder(ctx, req)
	}
	input, err := req.Input([]string{"Now: " + req.Fired.Format("Monday, 2 January 2006, 15:04") + ", UTC"})
	if err != nil {
		return schema.RunEventRequest{}, err
	}
	return schema.RunEventRequest{Input: input, Title: req.Title, Scheduled: req.Scheduled, Fired: req.Fired}, nil
}
