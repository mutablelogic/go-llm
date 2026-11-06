package registry

import (
	"context"
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

	if r.eventStore == nil {
		return nil, errNoEventStore
	} else if req.Session != "" {
		if _, err := uuid.Parse(req.Session); err != nil {
			return nil, llm.ErrBadParameter.Withf("invalid session %q", req.Session)
		}
	}
	if req.Status != "" {
		if _, err := schema.ParseEventStatus(string(req.Status)); err != nil {
			return nil, err
		}
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
