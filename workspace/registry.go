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
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
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
	watches map[string]*watch

	// Whether it runs
	running atomic.Bool
}

// ResourceEvent is the data of an event of a workspace's folders, and
// resources, which changed: the URI of the folder, or resource, or none,
// when it isn't known which
type ResourceEvent struct {
	URI string `json:"uri,omitempty"`
}

// watch is a workspace's watch, while it's watched
type watch struct {
	cancel context.CancelFunc
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
	r := &Registry{workspaces: make(map[string]Workspace), watches: make(map[string]*watch)}
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
		w := &watch{cancel: cancel}
		r.watches[name] = w
		ready := make(chan struct{})
		go r.watch(ctx, name, w, watcher, ready)
		select {
		case <-ready:
		case <-time.After(WatchTimeout):
		}
	}
	r.publish(eventschema.ChangeCreate, WorkspaceTable, name, nil)
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
	if w, exists := r.watches[name]; exists {
		w.cancel()
		delete(r.watches, name)
	}
	r.publish(eventschema.ChangeDelete, WorkspaceTable, name, nil)
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

// Resolve returns the resource, or artwork, with the URI, which is a
// workspace's, resource://<workspace>/<uri>, which is read from it, or its
// own URI, which is read from whichever of the workspaces, in order of their
// names, which read their resources, has it. Its URI is the workspace's.
// Returns ErrNotFound when none has it.
func (r *Registry) Resolve(ctx context.Context, uri string) (_ llm.Resource, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.workspace.Resolve",
		attribute.String("arg.uri", uri),
	)
	defer func() { endSpan(err) }()

	if name, _, ok := schema.ParseURI(uri); ok {
		return r.Read(ctx, name, uri)
	}
	for name, ws := range r.All() {
		if _, ok := ws.(Reader); !ok {
			continue
		}
		// Each is read as Read does, so it's traced, with its workspace
		resource, err := r.Read(ctx, name, uri)
		if err == nil {
			return resource, nil
		} else if !errors.Is(err, llm.ErrNotFound) && !errors.Is(err, llm.ErrBadParameter) {
			return nil, err
		}
	}
	return nil, llm.ErrNotFound.Withf("resource %q isn't in a workspace", uri)
}

// Read returns the resource, or artwork, with the URI, which is the
// workspace's, resource://<workspace>/<uri>, or its own, of the workspace
// with the name, whose URI is the workspace's. Returns ErrNotFound when
// there's no workspace with the name, or the URI is another's, and
// ErrNotImplemented when it isn't a Reader, so its resources aren't read.
func (r *Registry) Read(ctx context.Context, name, uri string) (resource llm.Resource, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.workspace.Read",
		attribute.String("arg.name", name),
		attribute.String("arg.uri", uri),
	)
	defer func() { endSpan(err) }()

	workspace, rest, isWorkspaces := schema.ParseURI(uri)
	if isWorkspaces && workspace != name {
		return nil, llm.ErrNotFound.Withf("resource %q isn't in workspace %q", uri, name)
	}
	var root string
	err = r.With(name, func(ws Workspace) error {
		reader, ok := ws.(Reader)
		if !ok {
			return llm.ErrNotImplemented.Withf("workspace %q doesn't read its resources", name)
		}
		root = rootOf(ws)
		if isWorkspaces {
			uri = schema.OriginalURI(root, rest)
		}
		resource, err = reader.Read(ctx, uri)
		return err
	})
	if err != nil {
		return nil, err
	}
	return withURI(resource, schema.URIOf(name, root, uri)), nil
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
func (r *Registry) watch(ctx context.Context, name string, w *watch, watcher Watcher, ready chan struct{}) {
	ch := make(chan schema.Change, 16)
	done := make(chan error, 1)
	go func() { done <- watcher.WatchResources(ctx, ch) }()

	// Once it ends, such as when it fails, or the registry is closed, it
	// isn't watched, unless it's been watched again since. This is done
	// after it says it's watching, below, as Add waits for that, with the
	// lock held.
	defer func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.watches[name] == w {
			delete(r.watches, name)
		}
	}()

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
		uri := change.URI
		if uri != "" {
			uri = schema.URIOf(name, rootOf(watcher), uri)
		}
		data, err := json.Marshal(ResourceEvent{URI: uri})
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
func (r *Registry) publish(action eventschema.ChangeAction, table, row string, data json.RawMessage) {
	if r.broadcaster != nil {
		r.broadcaster.Publish(eventschema.Change{Action: action, Schema: EventSchema, Table: table, Row: row, Data: data})
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

// ListWorkspaces returns the workspaces which are available, in order of
// their names, with whether each reads its resources, and is watched now
func (r *Registry) ListWorkspaces(ctx context.Context) *schema.WorkspaceList {
	_, endSpan := otel.StartSpan(r.tracer, ctx, "llm.workspace.ListWorkspaces")
	defer func() { endSpan(nil) }()

	r.mu.RLock()
	defer r.mu.RUnlock()
	result := &schema.WorkspaceList{Body: []schema.Workspace{}}
	for _, name := range slices.Sorted(maps.Keys(r.workspaces)) {
		if ws := r.workspaces[name]; available(ws) {
			result.Body = append(result.Body, r.workspaceOf(name, ws))
		}
	}
	result.Count = uint(len(result.Body))
	return result
}

// GetWorkspace returns the workspace with the name, with whether it reads
// its resources, and is watched. Returns ErrNotFound when there's none.
func (r *Registry) GetWorkspace(ctx context.Context, name string) (result *schema.Workspace, err error) {
	_, endSpan := otel.StartSpan(r.tracer, ctx, "llm.workspace.GetWorkspace",
		attribute.String("arg.name", name),
	)
	defer func() { endSpan(err) }()

	err = r.With(name, func(ws Workspace) error {
		w := r.workspaceOf(name, ws)
		result = &w
		return nil
	})
	return result, err
}

// ListResources returns a page of the folders, and resources, of the
// workspace with the name, whose URIs, and their artwork's, are the
// workspace's, resource://<workspace>/<uri>. Returns ErrNotFound when there's none.
func (r *Registry) ListResources(ctx context.Context, name string, req schema.ListRequest) (result *schema.ListResponse, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.workspace.ListResources",
		attribute.String("arg.name", name),
		attribute.String("arg.req", types.Stringify(req)),
	)
	defer func() { endSpan(err) }()

	var root string
	err = r.With(name, func(ws Workspace) error {
		result, err = ws.List(ctx, req)
		root = rootOf(ws)
		return err
	})
	if err != nil {
		return nil, err
	}

	// Only those of the type, as a workspace can list them all, as an MCP
	// server which isn't a workspace's does
	if req.Type != "" {
		result.Body = slices.DeleteFunc(result.Body, func(r schema.ResourceMeta) bool { return r.Type != req.Type })
	}

	// Their URIs, and their artwork's, are the workspace's
	for i := range result.Body {
		resource := &result.Body[i]
		resource.URI = schema.URIOf(name, root, resource.URI)
		for j := range resource.Artwork {
			resource.Artwork[j].URI = schema.URIOf(name, root, resource.Artwork[j].URI)
		}
	}
	return result, nil
}

// workspaceOf returns the workspace, as it's listed: whether it reads its
// resources, and whether the registry watches it now, which it does with a
// broadcaster, until its watch ends. The caller holds the lock.
func (r *Registry) workspaceOf(name string, ws Workspace) schema.Workspace {
	_, reader := ws.(Reader)
	_, watched := r.watches[name]
	result := schema.Workspace{Name: name, Read: reader, Watch: watched}
	if t, ok := ws.(Tree); ok {
		result.Tree = t.Tree()
	}
	if d, ok := ws.(Describer); ok {
		result.Title, result.Description, result.Version = d.Title(), d.Description(), d.Version()
	}
	return result
}

// rootOf returns the root of the workspace's resources' URIs, or nothing,
// when it has none
func rootOf(ws any) string {
	if rooted, ok := ws.(Rooted); ok {
		return rooted.Root()
	}
	return ""
}
