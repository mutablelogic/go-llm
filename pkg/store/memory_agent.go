package store

import (
	"context"
	"sync"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
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
	if err := validateAgentName(meta.Name); err != nil {
		return nil, err
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
		if a, exists := m.agents[agentID]; exists {
			return a, nil
		}
		// Name index is stale — repair it
		delete(m.names, id)
	}

	return nil, llm.ErrNotFound.Withf("agent %q", id)
}

// ListAgents returns agents ordered by creation time (most recent first),
// with pagination support. When filtered by name, all versions for that name
// are returned. Otherwise only the latest version of each agent is returned.
func (m *MemoryAgentStore) ListAgents(_ context.Context, req schema.ListAgentRequest) (*schema.ListAgentResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect all agents
	all := make([]*schema.Agent, 0, len(m.agents))
	for _, a := range m.agents {
		all = append(all, a)
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

// DeleteAgent removes an agent by ID, or all agents with the given name.
// Returns an error if no matching agent is found.
func (m *MemoryAgentStore) DeleteAgent(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Try delete by ID
	if a, ok := m.agents[id]; ok {
		delete(m.agents, id)
		// Update name index: point to the latest remaining version, or remove
		m.repairNameIndex(a.Name)
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

	// Validate and create new version
	a, err := newAgentVersion(existing, meta)
	if err != nil {
		return nil, err
	}
	if a == existing {
		return a, nil // no-op
	}

	// Ensure unique ID in our map
	for _, exists := m.agents[a.ID]; exists; _, exists = m.agents[a.ID] {
		a.ID = uuid.New().String()
	}

	m.agents[a.ID] = a
	m.names[a.Name] = a.ID

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

// repairNameIndex updates names[name] to point at the highest-version
// remaining agent with that name, or removes the entry if none remain.
// Must be called with the lock held.
func (m *MemoryAgentStore) repairNameIndex(name string) {
	var best *schema.Agent
	for _, a := range m.agents {
		if a.Name == name && (best == nil || a.Version > best.Version) {
			best = a
		}
	}
	if best == nil {
		delete(m.names, name)
	} else {
		m.names[name] = best.ID
	}
}
