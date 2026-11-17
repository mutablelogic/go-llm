package workspace

import (
	"context"
	"iter"
	"maps"
	"slices"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Registry is the workspaces, by their names. It's safe to use
// concurrently.
type Registry struct {
	mu         sync.RWMutex
	workspaces map[string]Workspace
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a registry, with no workspaces
func New() *Registry {
	return &Registry{workspaces: make(map[string]Workspace)}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Add adds the workspace. Returns ErrBadParameter when there's no workspace,
// or its name isn't an identifier, and ErrConflict when the registry has a
// workspace with its name.
func (r *Registry) Add(ws Workspace) error {
	if ws == nil {
		return llm.ErrBadParameter.With("missing workspace")
	}
	name := ws.Name()
	if !types.IsIdentifier(name) {
		return llm.ErrBadParameter.Withf("workspace name %q isn't an identifier", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.workspaces[name]; exists {
		return llm.ErrConflict.Withf("workspace %q already exists", name)
	}
	r.workspaces[name] = ws
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
	delete(r.workspaces, name)
	return ws, nil
}

// With calls the function with the workspace with the name, and returns its
// error, while the workspace can't be removed, so the function mustn't add,
// or remove, a workspace. Returns ErrNotFound when there's none.
func (r *Registry) With(name string, fn func(Workspace) error) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ws, exists := r.workspaces[name]
	if !exists {
		return llm.ErrNotFound.Withf("workspace %q", name)
	}
	return fn(ws)
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

// All returns the workspaces, in order of their names, as they were when
// it's ranged over, which can add, or remove, workspaces, as it ranges
func (r *Registry) All() iter.Seq[Workspace] {
	return func(yield func(Workspace) bool) {
		r.mu.RLock()
		workspaces := make([]Workspace, 0, len(r.workspaces))
		for _, name := range slices.Sorted(maps.Keys(r.workspaces)) {
			workspaces = append(workspaces, r.workspaces[name])
		}
		r.mu.RUnlock()
		for _, ws := range workspaces {
			if !yield(ws) {
				return
			}
		}
	}
}
