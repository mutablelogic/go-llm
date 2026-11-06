package test

import (
	"context"
	"sync"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// EventStore is a store of the outcomes of events in memory, for tests,
// which lists them the most recently created first
type EventStore struct {
	mu     sync.Mutex
	events map[uuid.UUID]schema.Event
}

var _ registry.EventStore = (*EventStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewEventStore returns a store of events in memory, which has none
func NewEventStore() *EventStore {
	return &EventStore{events: make(map[uuid.UUID]schema.Event)}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (m *EventStore) Create(_ context.Context, meta schema.EventMeta) (schema.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	event := schema.Event{ID: uuid.NewV7(), Serial: 1, Created: now, Modified: now, EventMeta: meta}
	m.events[event.ID] = event
	return event, nil
}

func (m *EventStore) Get(_ context.Context, id uuid.UUID) (schema.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	event, exists := m.events[id]
	if !exists {
		return schema.Event{}, llm.ErrNotFound.Withf("event %q", id)
	}
	return event, nil
}

func (m *EventStore) List(_ context.Context, req schema.EventListRequest) (schema.EventList, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []schema.Event
	for _, event := range m.events {
		if req.Matches(event) {
			list = append(list, event)
		}
	}
	schema.SortEvents(list)
	count := uint(len(list))
	list = list[min(req.Offset, count):]
	if req.Limit != nil {
		list = list[:min(*req.Limit, uint(len(list)))]
	}
	return schema.EventList{EventListRequest: req, Count: count, Body: list}, nil
}

func (m *EventStore) Put(_ context.Context, event schema.Event) (schema.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, exists := m.events[event.ID]
	if !exists {
		return schema.Event{}, llm.ErrNotFound.Withf("event %q", event.ID)
	} else if existing.Serial != event.Serial {
		return schema.Event{}, llm.ErrConflict.Withf("event %q is at serial %d, not %d", event.ID, existing.Serial, event.Serial)
	}
	event.Serial++
	event.Created = existing.Created
	event.Modified = time.Now()
	m.events[event.ID] = event
	return event, nil
}

func (m *EventStore) Delete(_ context.Context, id uuid.UUID, serial uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, exists := m.events[id]
	if !exists {
		return nil
	} else if existing.Serial != serial {
		return llm.ErrConflict.Withf("event %q is at serial %d, not %d", id, existing.Serial, serial)
	}
	delete(m.events, id)
	return nil
}
