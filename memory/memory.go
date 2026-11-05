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
	"errors"
	"strings"
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

// Store keeps the details, and the memories, such as in files, or a
// database, so they're kept when the server restarts
type Store interface {
	// Details returns the details, which are all not known, when none are
	// stored
	Details(context.Context) (schema.Details, error)

	// PutDetails replaces the details
	PutDetails(context.Context, schema.Details) error

	// ListMemories returns the memories which match the query, as Matches
	// says, the most recently changed first
	ListMemories(ctx context.Context, q string) ([]schema.Memory, error)

	// GetMemory returns the memory with the key, or ErrNotFound when there's
	// none
	GetMemory(ctx context.Context, key string) (schema.Memory, error)

	// PutMemory creates the memory, or replaces the memory with its key
	PutMemory(context.Context, schema.Memory) error

	// DeleteMemory deletes the memory with the key, or returns ErrNotFound
	// when there's none
	DeleteMemory(ctx context.Context, key string) error
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
const (
	DetailTable = "detail"

	// MemoryTable is the table of the events of the memories, whose schema
	// is Name, and whose rows are their keys
	MemoryTable = "memory"
)

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

// ListMemories returns the memories which match the request's query, the
// most recently changed first, of its page, and how many there are
func (m *Memory) ListMemories(ctx context.Context, req schema.MemoryListRequest) (_ schema.MemoryList, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "memory.ListMemories",
		attribute.String("arg.req", types.Stringify(req)),
	)
	defer func() { endSpan(err) }()

	memories, err := m.store.ListMemories(ctx, req.Q)
	if err != nil {
		return schema.MemoryList{}, err
	}
	return req.Page(memories), nil
}

// GetMemory returns the memory with the key, or ErrNotFound when there's
// none
func (m *Memory) GetMemory(ctx context.Context, key string) (_ schema.Memory, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "memory.GetMemory",
		attribute.String("arg.key", key),
	)
	defer func() { endSpan(err) }()
	return m.store.GetMemory(ctx, key)
}

// Remember creates the memory, when the version is zero, or replaces the
// memory with its key, when the version is its version, and it's changed,
// publishes the change, and returns the memory, and whether it was created.
// Returns ErrBadParameter when it isn't valid, ErrConflict when there's a
// memory with its key, and the version isn't its version, as it was changed
// since it was seen, or there's none, and the version isn't zero, and the
// store's errors.
func (m *Memory) Remember(ctx context.Context, meta schema.MemoryMeta, version uint64) (_ schema.Memory, created bool, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "memory.Remember",
		attribute.String("arg.key", meta.Key),
		attribute.Int64("arg.version", int64(version)),
	)
	defer func() { endSpan(err) }()

	meta, err = meta.Validate()
	if err != nil {
		return schema.Memory{}, false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	// It's created, or replaced, at its version, when it's changed
	memory, err := m.store.GetMemory(ctx, meta.Key)
	switch {
	case errors.Is(err, llm.ErrNotFound) && version != 0:
		return schema.Memory{}, false, llm.ErrConflict.Withf("there's no memory %q at version %d, so remember it without a version, to create it", meta.Key, version)
	case errors.Is(err, llm.ErrNotFound):
		memory, created = schema.Memory{MemoryMeta: meta, Version: 1, Created: m.now()}, true
	case err != nil:
		return schema.Memory{}, false, err
	case version != memory.Version:
		return schema.Memory{}, false, llm.ErrConflict.Withf("memory %q is at version %d, not %d, so recall it, and remember all of it again, with its version", meta.Key, memory.Version, version)
	case memory.MemoryMeta == meta:
		return memory, false, nil
	default:
		memory.MemoryMeta, memory.Version, memory.Modified = meta, memory.Version+1, m.now()
	}
	if err := m.store.PutMemory(ctx, memory); err != nil {
		return schema.Memory{}, false, err
	}
	if created {
		m.publishMemory(eventschema.EventCreate, memory)
	} else {
		m.publishMemory(eventschema.EventUpdate, memory)
	}
	return memory, created, nil
}

// Forget deletes the memory with the key, publishes that it was, and
// returns it, as it was. Returns ErrNotFound when there's none.
func (m *Memory) Forget(ctx context.Context, key string) (_ schema.Memory, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "memory.Forget",
		attribute.String("arg.key", key),
	)
	defer func() { endSpan(err) }()

	m.mu.Lock()
	defer m.mu.Unlock()
	memory, err := m.store.GetMemory(ctx, key)
	if err != nil {
		return schema.Memory{}, err
	} else if err := m.store.DeleteMemory(ctx, key); err != nil {
		return schema.Memory{}, err
	}
	m.publishMemory(eventschema.EventDelete, memory)
	return memory, nil
}

// ForgetLine takes the line out of the body of the memory with the key,
// apart from the space around it, and a bullet, such as "- ", or forgets the
// memory, when there's nothing left of it, publishes the change, and returns
// the memory, as it is, or was, when it's forgotten. Returns ErrNotFound when
// there's no memory with the key, or it hasn't the line.
func (m *Memory) ForgetLine(ctx context.Context, key, line string) (_ schema.Memory, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "memory.ForgetLine",
		attribute.String("arg.key", key),
	)
	defer func() { endSpan(err) }()

	m.mu.Lock()
	defer m.mu.Unlock()
	memory, err := m.store.GetMemory(ctx, key)
	if err != nil {
		return schema.Memory{}, err
	}
	body, removed := memory.WithoutLine(line)
	switch {
	case !removed:
		return schema.Memory{}, llm.ErrNotFound.Withf("memory %q has no line %q", key, strings.TrimSpace(line))
	case body == "":
		if err := m.store.DeleteMemory(ctx, key); err != nil {
			return schema.Memory{}, err
		}
		m.publishMemory(eventschema.EventDelete, memory)
		return memory, nil
	}
	memory.Body, memory.Version, memory.Modified = body, memory.Version+1, m.now()
	if err := m.store.PutMemory(ctx, memory); err != nil {
		return schema.Memory{}, err
	}
	m.publishMemory(eventschema.EventUpdate, memory)
	return memory, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// publishMemory publishes the change to the memory, when the memory has a
// broadcaster, whose row is its key, and whose data is its version, unless
// it was deleted
func (m *Memory) publishMemory(action eventschema.EventAction, memory schema.Memory) {
	if m.broadcaster == nil {
		return
	}
	event := eventschema.Event{Action: action, Schema: Name, Table: MemoryTable, Row: memory.Key}
	if action != eventschema.EventDelete {
		if data, err := json.Marshal(schema.MemoryEvent{Version: memory.Version}); err == nil {
			event.Data = data
		}
	}
	m.broadcaster.Publish(event)
}

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
