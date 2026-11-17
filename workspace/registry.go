package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"maps"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Registry is the workspaces, by their names. It's safe to use
// concurrently.
type Registry struct {
	opt
	mu         sync.RWMutex
	workspaces map[string]Workspace

	// The lifetime of the watches of the workspaces which are Watchers,
	// with a broadcaster, which ends when Close is called, and the cancels
	// of each, by name
	ctx     context.Context
	cancel  context.CancelFunc
	watches map[string]context.CancelFunc

	// Whether it runs
	running atomic.Bool
}

// ResourceEvent is the data of an event of a workspace's folders, and
// resources, which changed: the URI of the folder, or resource, or none,
// when it isn't known which
type ResourceEvent struct {
	URI string `json:"uri,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// EventSchema is the schema of the registry's events
	EventSchema = "workspace"

	// WorkspaceTable is the table of the events of the workspaces which
	// are added, and removed, whose rows are their names
	WorkspaceTable = "workspace"

	// WatchTimeout is how long Add waits for a workspace which is a Watcher
	// to say it's watching, before it's said to be added
	WatchTimeout = 5 * time.Second

	// ResourceTable is the table of the events of the workspaces' folders,
	// and resources, which change, whose rows are the workspaces' names,
	// and whose data is a ResourceEvent
	ResourceTable = "resource"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a registry, with no workspaces, with the options, such as
// the broadcaster its changes are published to, with WithBroadcaster
func New(opts ...Opt) (*Registry, error) {
	r := &Registry{workspaces: make(map[string]Workspace), watches: make(map[string]context.CancelFunc)}
	if err := r.opt.apply(opts); err != nil {
		return nil, err
	}
	r.ctx, r.cancel = context.WithCancel(context.Background())
	return r, nil
}

// Close stops watching the workspaces, whose changes aren't published
func (r *Registry) Close() error {
	r.cancel()
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Add adds the workspace, with the name, such as its namespace's. Returns
// ErrBadParameter when there's no workspace, or the name isn't an
// identifier, and ErrConflict when the registry has a workspace with the
// name.
func (r *Registry) Add(name string, ws Workspace) error {
	if ws == nil {
		return llm.ErrBadParameter.With("missing workspace")
	} else if !types.IsIdentifier(name) {
		return llm.ErrBadParameter.Withf("workspace name %q isn't an identifier", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.workspaces[name]; exists {
		return llm.ErrConflict.Withf("workspace %q already exists", name)
	}
	r.workspaces[name] = ws

	// Publish its changes, when it says them, once it's watching, so a
	// change after it's said to be added isn't missed
	if watcher, ok := ws.(Watcher); ok && r.broadcaster != nil {
		ctx, cancel := context.WithCancel(r.ctx)
		r.watches[name] = cancel
		ready := make(chan struct{})
		go r.watch(ctx, name, watcher, ready)
		select {
		case <-ready:
		case <-time.After(WatchTimeout):
		}
	}
	r.publish(eventschema.EventCreate, WorkspaceTable, name, nil)
	return nil
}

// Remove removes the workspace with the name, and returns it. Returns
// ErrNotFound when there's none.
func (r *Registry) Remove(name string) (Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws, exists := r.workspaces[name]
	if !exists {
		return nil, llm.ErrNotFound.Withf("workspace %q", name)
	}
	r.remove(name)
	return ws, nil
}

// RemoveIf removes the workspace with the name, when it's the workspace,
// rather than one which replaced it, and returns true, or returns false
func (r *Registry) RemoveIf(name string, ws Workspace) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, exists := r.workspaces[name]; !exists || !same(existing, ws) {
		return false
	}
	r.remove(name)
	return true
}

// remove removes the workspace with the name, stops watching it, and
// publishes that it was removed. The caller holds the lock.
func (r *Registry) remove(name string) {
	delete(r.workspaces, name)
	if cancel, exists := r.watches[name]; exists {
		cancel()
		delete(r.watches, name)
	}
	r.publish(eventschema.EventDelete, WorkspaceTable, name, nil)
}

// With calls the function with the workspace with the name, and returns its
// error, while the workspace can't be removed, so the function mustn't add,
// or remove, a workspace. Returns ErrNotFound when there's none.
func (r *Registry) With(name string, fn func(Workspace) error) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ws, exists := r.workspaces[name]
	if !exists || !available(ws) {
		return llm.ErrNotFound.Withf("workspace %q", name)
	}
	return fn(ws)
}

// Resolve returns the resource, or artwork, with the URI, of whichever of
// the workspaces, in order of their names, which read their resources, has
// it. Returns ErrNotFound when none has it.
func (r *Registry) Resolve(ctx context.Context, uri string) (llm.Resource, error) {
	for _, ws := range r.All() {
		reader, ok := ws.(Reader)
		if !ok {
			continue
		}
		resource, err := reader.Read(ctx, uri)
		if err == nil {
			return resource, nil
		} else if !errors.Is(err, llm.ErrNotFound) && !errors.Is(err, llm.ErrBadParameter) {
			return nil, err
		}
	}
	return nil, llm.ErrNotFound.Withf("resource %q isn't in a workspace", uri)
}

// Read returns the resource, or artwork, with the URI, of the workspace with
// the name. Returns ErrNotFound when there's no workspace with the name, and
// ErrNotImplemented when it isn't a Reader, so its resources aren't read.
func (r *Registry) Read(ctx context.Context, name, uri string) (resource llm.Resource, err error) {
	err = r.With(name, func(ws Workspace) error {
		reader, ok := ws.(Reader)
		if !ok {
			return llm.ErrNotImplemented.Withf("workspace %q doesn't read its resources", name)
		}
		resource, err = reader.Read(ctx, uri)
		return err
	})
	return resource, err
}

// All returns the workspaces which are available, by their names, in order of their names, as
// they were when it's ranged over, which can add, or remove, workspaces,
// as it ranges
func (r *Registry) All() iter.Seq2[string, Workspace] {
	return func(yield func(string, Workspace) bool) {
		r.mu.RLock()
		var names []string
		var workspaces []Workspace
		for _, name := range slices.Sorted(maps.Keys(r.workspaces)) {
			if ws := r.workspaces[name]; available(ws) {
				names, workspaces = append(names, name), append(workspaces, ws)
			}
		}
		r.mu.RUnlock()
		for i, ws := range workspaces {
			if !yield(names[i], ws) {
				return
			}
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// watch publishes the workspace's changes, until the context is cancelled,
// as it's removed, or the registry is closed
func (r *Registry) watch(ctx context.Context, name string, watcher Watcher, ready chan struct{}) {
	ch := make(chan schema.Change, 16)
	done := make(chan error, 1)
	go func() { done <- watcher.Watch(ctx, ch) }()

	// The first change says it's watching, which isn't published
	watching := func() {
		if ready != nil {
			close(ready)
			ready = nil
		}
	}
	defer watching()
	publish := func(change schema.Change) {
		if ready != nil {
			watching()
			return
		}
		data, err := json.Marshal(ResourceEvent{URI: change.URI})
		if err != nil {
			return
		}
		r.mu.RLock()
		defer r.mu.RUnlock()
		if ctx.Err() == nil {
			// It's still in the registry, as it isn't removed while the lock
			// is held, and its watch is cancelled once it is
			r.publish(change.Action, ResourceTable, name, data)
		}
	}
	for {
		select {
		case change := <-ch:
			publish(change)
		case <-done:
			// The changes it said before it ended are published
			for {
				select {
				case change := <-ch:
					publish(change)
				default:
					return
				}
			}
		}
	}
}

// publish publishes the change, when there's a broadcaster
func (r *Registry) publish(action eventschema.EventAction, table, row string, data json.RawMessage) {
	if r.broadcaster != nil {
		r.broadcaster.Publish(eventschema.Event{Action: action, Schema: EventSchema, Table: table, Row: row, Data: data})
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// available returns true when the workspace is one now, which it is, unless
// it says it isn't
func available(ws Workspace) bool {
	if a, ok := ws.(Available); ok {
		return a.Available()
	}
	return true
}

// same returns true when the workspaces are the same one, which they are
// when they're equal values, which can be compared
func same(a, b Workspace) (result bool) {
	ta, tb := reflect.TypeOf(a), reflect.TypeOf(b)
	if ta != tb || ta == nil || !ta.Comparable() {
		return false
	}
	defer func() {
		if recover() != nil {
			result = false
		}
	}()
	return a == b
}
