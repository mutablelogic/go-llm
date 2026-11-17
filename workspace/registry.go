package workspace

import (
	"context"
	"errors"
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
// PRIVATE FUNCTIONS

// available returns true when the workspace is one now, which it is, unless
// it says it isn't
func available(ws Workspace) bool {
	if a, ok := ws.(Available); ok {
		return a.Available()
	}
	return true
}
