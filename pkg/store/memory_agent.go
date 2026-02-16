package store

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// MemoryAgentStore is an in-memory implementation of AgentStore.
// It is safe for concurrent use.
type MemoryAgentStore struct {
	mu     sync.RWMutex
	agents map[string]*schema.Agent // keyed by ID
	names  map[string]string        // name -> ID for uniqueness
}

var _ schema.AgentStore = (*MemoryAgentStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewMemoryAgentStore creates a new empty in-memory agent store.
func NewMemoryAgentStore() *MemoryAgentStore {
	return &MemoryAgentStore{
		agents: make(map[string]*schema.Agent),
		names:  make(map[string]string),
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateAgent creates a new agent with a unique ID and returns it.
// The agent name must be unique across all agents.
func (m *MemoryAgentStore) CreateAgent(_ context.Context, meta schema.AgentMeta) (*schema.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check name validity
	if !types.IsIdentifier(meta.Name) {
		return nil, llm.ErrBadParameter.Withf("agent name: must be a valid identifier, got %q", meta.Name)
	} else if _, exists := m.names[meta.Name]; exists {
		return nil, llm.ErrConflict.Withf("agent name %q already exists", meta.Name)
	}

	// Generate a unique ID
	id := uuid.New().String()
	for _, exists := m.agents[id]; exists; _, exists = m.agents[id] {
		id = uuid.New().String()
	}

	// Store the agent
	a := &schema.Agent{
		ID:        id,
		Created:   time.Now(),
		Version:   1,
		AgentMeta: meta,
	}

	m.agents[id] = a
	m.names[meta.Name] = id

	// Return the created agent
	return a, nil
}

// GetAgent retrieves an agent by ID or name. If a matching ID is found it is
// returned directly. Otherwise, the store looks up the name in the name index
// and returns the latest version (highest version number) for that name.
func (m *MemoryAgentStore) GetAgent(_ context.Context, id string) (*schema.Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getAgentLocked(id)
}

// getAgentLocked is the internal lookup helper. The caller must hold at least
// a read lock.
func (m *MemoryAgentStore) getAgentLocked(id string) (*schema.Agent, error) {
	// Try lookup by ID first
	if a, ok := m.agents[id]; ok {
		return a, nil
	}

	// Fall back to lookup by name via the name index (O(1))
	if agentID, ok := m.names[id]; ok {
		return m.agents[agentID], nil
	}

	return nil, llm.ErrNotFound.Withf("agent %q", id)
}

// ListAgents returns agents ordered by creation time (most recent first),
// with pagination support. When filtered by name, all versions for that name
// are returned. Otherwise only the latest version of each agent is returned.
func (m *MemoryAgentStore) ListAgents(_ context.Context, req schema.ListAgentRequest) (*schema.ListAgentResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect candidates
	var candidates []*schema.Agent
	for _, a := range m.agents {
		if req.Name != "" && a.Name != req.Name {
			continue
		}
		if req.Version != nil && a.Version != *req.Version {
			continue
		}
		candidates = append(candidates, a)
	}

	// Sort by creation time, most recent first
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Created.After(candidates[j].Created)
	})

	// When no name filter, keep only the latest version per name
	var result []*schema.Agent
	if req.Name == "" {
		seen := make(map[string]bool)
		for _, a := range candidates {
			if !seen[a.Name] {
				seen[a.Name] = true
				result = append(result, a)
			}
		}
	} else {
		result = candidates
	}

	// Paginate
	total := uint(len(result))
	start := req.Offset
	if start > total {
		start = total
	}
	end := start + types.Value(req.Limit)
	if req.Limit == nil || end > total {
		end = total
	}

	return &schema.ListAgentResponse{
		Count:  total,
		Offset: req.Offset,
		Limit:  req.Limit,
		Body:   result[start:end],
	}, nil
}

// DeleteAgent removes an agent by ID, or all agents with the given name.
// Returns an error if no matching agent is found.
func (m *MemoryAgentStore) DeleteAgent(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Try delete by ID
	if a, ok := m.agents[id]; ok {
		delete(m.agents, id)
		// Remove name mapping if no other agent has this name
		if !m.hasName(a.Name) {
			delete(m.names, a.Name)
		}
		return nil
	}

	// Fall back to delete all agents by name
	var found bool
	for aid, a := range m.agents {
		if a.Name == id {
			delete(m.agents, aid)
			found = true
		}
	}
	if !found {
		return llm.ErrNotFound.Withf("agent %q", id)
	}
	delete(m.names, id)
	return nil
}

// UpdateAgent creates a new version of an existing agent. The id parameter
// can be an agent ID or name. If the metadata is identical to the current
// version, the existing agent is returned unchanged (no-op). Otherwise a
// new agent is stored with a new UUID, the same name, and an incremented
// version number. The entire read-modify-write sequence is atomic.
func (m *MemoryAgentStore) UpdateAgent(_ context.Context, id string, meta schema.AgentMeta) (*schema.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the existing agent (by ID or name)
	existing, err := m.getAgentLocked(id)
	if err != nil {
		return nil, err
	}

	// If the name is changing, validate the new name
	if meta.Name != "" && meta.Name != existing.Name {
		return nil, llm.ErrBadParameter.With("agent name cannot be changed via update")
	}

	// Use the existing name if not provided
	if meta.Name == "" {
		meta.Name = existing.Name
	}

	// No-op if nothing has changed
	if agentMetaEqual(existing.AgentMeta, meta) {
		return existing, nil
	}

	// Generate a unique ID
	newID := uuid.New().String()
	for _, exists := m.agents[newID]; exists; _, exists = m.agents[newID] {
		newID = uuid.New().String()
	}

	a := &schema.Agent{
		ID:        newID,
		Created:   time.Now(),
		Version:   existing.Version + 1,
		AgentMeta: meta,
	}

	m.agents[newID] = a
	m.names[meta.Name] = newID

	return a, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// hasName returns true if any agent in the store has the given name.
// Must be called with the lock held.
func (m *MemoryAgentStore) hasName(name string) bool {
	for _, a := range m.agents {
		if a.Name == name {
			return true
		}
	}
	return false
}

// agentMetaEqual returns true if two AgentMeta values are identical.
func agentMetaEqual(a, b schema.AgentMeta) bool {
	if a.Name != b.Name || a.Title != b.Title || a.Description != b.Description || a.Template != b.Template {
		return false
	}
	if a.Provider != b.Provider || a.Model != b.Model || a.SystemPrompt != b.SystemPrompt {
		return false
	}
	if a.ThinkingBudget != b.ThinkingBudget {
		return false
	}
	// Compare *bool Thinking
	switch {
	case a.Thinking == nil && b.Thinking == nil:
	case a.Thinking == nil || b.Thinking == nil:
		return false
	case *a.Thinking != *b.Thinking:
		return false
	}
	// Compare JSONSchema fields as JSON bytes
	if !jsonEqual(a.Format, b.Format) || !jsonEqual(a.Input, b.Input) {
		return false
	}
	// Compare Tools slices
	if len(a.Tools) != len(b.Tools) {
		return false
	}
	for i := range a.Tools {
		if a.Tools[i] != b.Tools[i] {
			return false
		}
	}
	return true
}

// jsonEqual compares two JSONSchema values as byte slices.
func jsonEqual(a, b schema.JSONSchema) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	// Normalize by re-marshalling through json
	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		return string(a) == string(b)
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return string(a) == string(b)
	}
	na, _ := json.Marshal(va)
	nb, _ := json.Marshal(vb)
	return string(na) == string(nb)
}
