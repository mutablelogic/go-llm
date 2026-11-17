package workspace

import (
	"context"
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

// Get returns the workspace with the name. Returns ErrNotFound when there's
// none.
func (r *Registry) Get(name string) (Workspace, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ws, exists := r.workspaces[name]
	if !exists {
		return nil, llm.ErrNotFound.Withf("workspace %q", name)
	}
	return ws, nil
}

// Read returns the resource, or artwork, with the URI, of the workspace with
// the name. Returns ErrNotFound when there's no workspace with the name, and
// ErrNotImplemented when it isn't a Reader, so its resources aren't read.
func (r *Registry) Read(ctx context.Context, name, uri string) (llm.Resource, error) {
	ws, err := r.Get(name)
	if err != nil {
		return nil, err
	}
	reader, ok := ws.(Reader)
	if !ok {
		return nil, llm.ErrNotImplemented.Withf("workspace %q doesn't read its resources", name)
	}
	return reader.Read(ctx, uri)
}

// Workspaces returns the workspaces, in order of their names
func (r *Registry) Workspaces() []Workspace {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Workspace, 0, len(r.workspaces))
	for _, name := range slices.Sorted(maps.Keys(r.workspaces)) {
		result = append(result, r.workspaces[name])
	}
	return result
}
