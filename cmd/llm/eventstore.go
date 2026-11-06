//go:build !client

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// eventStore is a store of the outcomes of events in a directory, as a file
// of each, as JSON, named by its identifier, such as
// event-3b241101-e2bb-4255-8caf-4136c566a962.json, beside the sessions. A
// file is written to a temporary file, which is renamed to it, so it's never
// written partly. It's safe to use concurrently.
type eventStore struct {
	mu  sync.Mutex
	dir string
}

var _ registry.EventStore = (*eventStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// eventPrefix and eventSuffix are of the name of an event's file
	eventPrefix = "event-"
	eventSuffix = ".json"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newEventStore returns a store of the outcomes of events in the directory,
// which is created, when it doesn't exist
func newEventStore(dir string) (*eventStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, llm.ErrBadParameter.With("missing directory")
	} else if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &eventStore{dir: dir}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create writes the event's outcome, with a new identifier, which is a
// random UUID, and a serial of one, and returns it
func (s *eventStore) Create(_ context.Context, meta schema.EventMeta) (schema.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	event := schema.Event{ID: uuid.New(), Serial: 1, Created: now, Modified: now, EventMeta: meta}
	if err := s.write(event); err != nil {
		return schema.Event{}, err
	}
	return event, nil
}

// Get returns the event with the identifier, or ErrNotFound when there's
// none
func (s *eventStore) Get(_ context.Context, id uuid.UUID) (schema.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(id)
}

// List returns the events the request selects, the most recently created
// first, from its offset, and at most its limit of them, and how many it
// selects. A file which can't be read as an event is skipped.
func (s *eventStore) List(_ context.Context, req schema.EventListRequest) (schema.EventList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return schema.EventList{}, err
	}
	var list []schema.Event
	for _, entry := range entries {
		id, ok := eventID(entry.Name())
		if !ok || entry.IsDir() {
			continue
		}
		if event, err := s.read(id); err == nil && req.Matches(event) {
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

// Put replaces the event with its identifier, when its serial is that
// event's, and returns it, with its serial one more, and when it was
// modified. Returns ErrNotFound when there's none, and ErrConflict when its
// serial isn't the event's.
func (s *eventStore) Put(_ context.Context, event schema.Event) (schema.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, err := s.read(event.ID)
	if err != nil {
		return schema.Event{}, err
	} else if existing.Serial != event.Serial {
		return schema.Event{}, llm.ErrConflict.Withf("event %q is at serial %d, not %d", event.ID, existing.Serial, event.Serial)
	}
	event.Serial++
	event.Created = existing.Created
	event.Modified = time.Now().UTC()
	if err := s.write(event); err != nil {
		return schema.Event{}, err
	}
	return event, nil
}

// Delete deletes the event with the identifier, when the serial is its
// serial, or does nothing when there's none. Returns ErrConflict when the
// serial isn't its serial.
func (s *eventStore) Delete(_ context.Context, id uuid.UUID, serial uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, err := s.read(id)
	if errors.Is(err, llm.ErrNotFound) {
		return nil
	} else if err != nil {
		return err
	} else if existing.Serial != serial {
		return llm.ErrConflict.Withf("event %q is at serial %d, not %d", id, existing.Serial, serial)
	}
	return os.Remove(s.path(id))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// path returns the path of the file of the event with the identifier
func (s *eventStore) path(id uuid.UUID) string {
	return filepath.Join(s.dir, eventPrefix+id.String()+eventSuffix)
}

// read returns the event with the identifier, from its file, or ErrNotFound
// when there's none. The caller holds the lock.
func (s *eventStore) read(id uuid.UUID) (schema.Event, error) {
	data, err := os.ReadFile(s.path(id))
	if errors.Is(err, fs.ErrNotExist) {
		return schema.Event{}, llm.ErrNotFound.Withf("event %q", id)
	} else if err != nil {
		return schema.Event{}, err
	}
	var event schema.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return schema.Event{}, llm.ErrInternalServerError.Withf("event %q: %v", id, err)
	}
	return event, nil
}

// write writes the event to its file, by writing a temporary file, which
// only the user can read, and renaming it to the event's. The caller holds
// the lock.
func (s *eventStore) write(event schema.Event) error {
	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.dir, "."+eventPrefix+"*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	} else if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	} else if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.path(event.ID))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// eventID returns the identifier of the event whose file has the name, and
// whether it is an event's file
func eventID(name string) (uuid.UUID, bool) {
	value, ok := strings.CutPrefix(name, eventPrefix)
	if !ok {
		return uuid.Nil(), false
	} else if value, ok = strings.CutSuffix(value, eventSuffix); !ok {
		return uuid.Nil(), false
	}
	id, err := uuid.Parse(value)
	return id, err == nil
}
