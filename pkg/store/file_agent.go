package store

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// FileAgentStore is a file-backed implementation of AgentStore.
// Each agent version is stored as {id}.json in a directory.
// It is safe for concurrent use.
type FileAgentStore struct {
	mu  sync.RWMutex
	dir string
}

var _ schema.AgentStore = (*FileAgentStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewFileAgentStore creates a new file-backed agent store in the given directory.
// The directory is created if it does not exist.
func NewFileAgentStore(dir string) (*FileAgentStore, error) {
	if err := ensureDir(dir); err != nil {
		return nil, err
	}
	return &FileAgentStore{dir: dir}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateAgent creates a new agent with a unique ID and version 1,
// writes it to disk, and returns it.
func (f *FileAgentStore) CreateAgent(_ context.Context, meta schema.AgentMeta) (*schema.Agent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Validate name
	if err := validateAgentName(meta.Name); err != nil {
		return nil, err
	}

	// Check name uniqueness across existing agents on disk
	if _, err := f.readByName(meta.Name); err == nil {
		return nil, llm.ErrConflict.Withf("agent name %q already exists", meta.Name)
	}

	a := &schema.Agent{
		ID:        uuid.New().String(),
		Created:   time.Now(),
		Version:   1,
		AgentMeta: meta,
	}

	if err := f.writeAgent(a); err != nil {
		return nil, err
	}
	return a, nil
}

// GetAgent retrieves an agent by ID or name. If a matching ID is found it is
// returned directly. Otherwise, the store scans all agent files and returns
// the latest version (highest version number) with the given name.
func (f *FileAgentStore) GetAgent(_ context.Context, id string) (*schema.Agent, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Try lookup by ID first
	if a, err := f.readAgent(id); err == nil {
		return a, nil
	}

	// Fall back to lookup by name
	return f.readByName(id)
}

// ListAgents returns agents matching the request, with pagination support.
// When filtered by name, all matching versions are returned; otherwise only
// the latest version of each agent is returned.
func (f *FileAgentStore) ListAgents(_ context.Context, req schema.ListAgentRequest) (*schema.ListAgentResponse, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	all, err := f.readAllAgents()
	if err != nil {
		return nil, err
	}

	// Filter, sort, and deduplicate
	result := filterAgents(all, req)

	// Paginate
	body, total := paginate(result, req.Offset, req.Limit)
	return &schema.ListAgentResponse{
		Count:  total,
		Offset: req.Offset,
		Limit:  req.Limit,
		Body:   body,
	}, nil
}

// DeleteAgent removes an agent by ID or name. When a name is provided,
// all versions of the agent are deleted. Returns an error if no matching
// agent exists.
func (f *FileAgentStore) DeleteAgent(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Try delete by ID first
	path := jsonPath(f.dir, id)
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return llm.ErrInternalServerError.Withf("remove: %v", err)
		}
		return nil
	}

	// Fall back to delete all agents by name
	agents, err := f.readAllAgents()
	if err != nil {
		return err
	}
	var found bool
	for _, a := range agents {
		if a.Name == id {
			if err := os.Remove(jsonPath(f.dir, a.ID)); err != nil {
				return llm.ErrInternalServerError.Withf("remove: %v", err)
			}
			found = true
		}
	}
	if !found {
		return llm.ErrNotFound.Withf("agent %q", id)
	}
	return nil
}

// UpdateAgent creates a new version of an existing agent with the given
// metadata changes. The id parameter can be an agent ID or name. If the
// metadata is identical to the current version the existing agent is
// returned unchanged (no-op). Otherwise a new agent is written to disk
// with a new UUID, the same name, and an incremented version number.
func (f *FileAgentStore) UpdateAgent(_ context.Context, id string, meta schema.AgentMeta) (*schema.Agent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Find the existing agent (by ID or name)
	existing, err := f.readAgent(id)
	if err != nil {
		// Fall back to lookup by name
		existing, err = f.readByName(id)
		if err != nil {
			return nil, err
		}
	}

	// Validate and create new version
	a, err := newAgentVersion(existing, meta)
	if err != nil {
		return nil, err
	}
	if a == existing {
		return a, nil // no-op
	}

	if err := f.writeAgent(a); err != nil {
		return nil, err
	}
	return a, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// writeAgent serialises an agent to its JSON file.
func (f *FileAgentStore) writeAgent(a *schema.Agent) error {
	return writeJSON(jsonPath(f.dir, a.ID), a)
}

// readAgent deserialises an agent from its JSON file.
func (f *FileAgentStore) readAgent(id string) (*schema.Agent, error) {
	var a schema.Agent
	if err := readJSON(jsonPath(f.dir, id), fmt.Sprintf("agent %q", id), &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// readAllAgents reads every agent JSON file from disk.
func (f *FileAgentStore) readAllAgents() ([]*schema.Agent, error) {
	ids, err := readJSONDir(f.dir)
	if err != nil {
		return nil, err
	}
	var agents []*schema.Agent
	for _, id := range ids {
		a, err := f.readAgent(id)
		if err != nil {
			continue // skip corrupt files
		}
		agents = append(agents, a)
	}
	return agents, nil
}

// readByName scans all agent files and returns the latest version (highest
// version number) with the given name. Returns ErrNotFound if none match.
func (f *FileAgentStore) readByName(name string) (*schema.Agent, error) {
	agents, err := f.readAllAgents()
	if err != nil {
		return nil, err
	}
	var best *schema.Agent
	for _, a := range agents {
		if a.Name == name && (best == nil || a.Version > best.Version) {
			best = a
		}
	}
	if best == nil {
		return nil, llm.ErrNotFound.Withf("agent %q", name)
	}
	return best, nil
}
