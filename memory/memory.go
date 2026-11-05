// Package memory is what the assistant remembers, which is shared by every
// session and agent: the details of the person it's talking to, and of
// itself, such as their names, where the person is, and their time zone and
// language, which are in the system prompt, each of which is known, not
// known, or declined, so it isn't. The memory is a namespace of a toolkit,
// whose tools set them, and whose system prompt has them, with the time.
package memory

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Store keeps the details, such as in a file, or a database, so they're
// kept when the server restarts
type Store interface {
	// Details returns the details, which are all not known, when none are
	// stored
	Details(context.Context) (schema.Details, error)

	// PutDetails replaces the details
	PutDetails(context.Context, schema.Details) error
}

// Memory is what the assistant remembers, which is kept in a store. It's a
// namespace of a toolkit, whose system prompt has the details, and the time,
// and whose tools change them. It's safe to use concurrently.
type Memory struct {
	mu          sync.RWMutex
	store       Store
	broadcaster *broadcaster.Broadcaster
	tracer      trace.Tracer
	details     schema.Details
	now         func() time.Time
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// DetailTable is the table of the events of the details, whose schema is
// Name, and whose rows are their fields, such as location
const DetailTable = "detail"

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the memory, with the details in the store, and the options,
// such as the broadcaster it publishes its changes to, and the tracer.
// Returns ErrBadParameter when there's no store, or an option isn't valid,
// and the store's errors.
func New(ctx context.Context, store Store, opts ...Opt) (_ *Memory, err error) {
	var o opt
	if store == nil {
		return nil, llm.ErrBadParameter.With("missing memory store")
	} else if err := o.apply(opts); err != nil {
		return nil, err
	}
	ctx, endSpan := otel.StartSpan(o.tracer, ctx, "memory.New")
	defer func() { endSpan(err) }()
	details, err := store.Details(ctx)
	if err != nil {
		return nil, err
	}
	return &Memory{store: store, broadcaster: o.broadcaster, tracer: o.tracer, details: details, now: time.Now}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Details returns the details
func (m *Memory) Details() schema.Details {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return clone(m.details)
}

// SetDetail sets the detail of the field, which is its value, or that it's
// declined, or forgets it, when it's nil, so it isn't known, in the store,
// publishes the change, when it changed, and returns the details. Returns
// ErrBadParameter when the field, or the detail, isn't valid, and the
// store's errors, when the details aren't changed.
func (m *Memory) SetDetail(ctx context.Context, field schema.Field, detail *schema.Detail) (_ schema.Details, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "memory.SetDetail",
		attribute.String("arg.field", string(field)),
		attribute.String("arg.detail", types.Stringify(detail)),
	)
	defer func() { endSpan(err) }()

	m.mu.Lock()
	defer m.mu.Unlock()
	details := clone(m.details)
	if err := details.Set(field, detail); err != nil {
		return schema.Details{}, err
	}

	// Nothing is written, or published, when it's as it was
	before, after := m.details.Get(field), details.Get(field)
	if equal(before, after) {
		return details, nil
	} else if err := m.store.PutDetails(ctx, details); err != nil {
		return schema.Details{}, err
	}
	m.details = details

	// Publish the change, under the lock, so the events are in the order of
	// the changes, which doesn't wait
	switch {
	case before == nil:
		m.publish(eventschema.EventCreate, field, after)
	case after == nil:
		m.publish(eventschema.EventDelete, field, nil)
	default:
		m.publish(eventschema.EventUpdate, field, after)
	}
	return clone(details), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// publish publishes the change to the detail of the field, when the memory
// has a broadcaster, with the detail, when there is one
func (m *Memory) publish(action eventschema.EventAction, field schema.Field, detail *schema.Detail) {
	if m.broadcaster == nil {
		return
	}
	event := eventschema.Event{Action: action, Schema: Name, Table: DetailTable, Row: string(field)}
	if detail != nil {
		if data, err := json.Marshal(detail); err == nil {
			event.Data = data
		}
	}
	m.broadcaster.Publish(event)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// equal returns whether the details are the same, or both nil
func equal(a, b *schema.Detail) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// clone returns a copy of the details, whose details are copies, so
// changing it doesn't change them
func clone(details schema.Details) schema.Details {
	var result schema.Details
	for _, field := range schema.Fields {
		// Set doesn't fail, as the details are valid
		_ = result.Set(field, details.Get(field))
	}
	return result
}
