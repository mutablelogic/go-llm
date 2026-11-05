package test

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// AgentStore is a store of agents in memory, by their names, for tests
type AgentStore struct {
	mu     sync.Mutex
	agents map[string]schema.Agent
}

var _ registry.AgentStore = (*AgentStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewAgentStore returns a store of agents in memory, which has none
func NewAgentStore() *AgentStore {
	return &AgentStore{agents: make(map[string]schema.Agent)}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (m *AgentStore) Create(_ context.Context, meta schema.AgentMeta) (schema.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.agents[meta.Name]; exists {
		return schema.Agent{}, llm.ErrConflict.Withf("agent %q exists", meta.Name)
	}
	now := time.Now()
	agent := schema.Agent{Serial: 1, Created: now, Modified: now, AgentMeta: meta}
	m.agents[meta.Name] = cloneAgent(agent)
	return cloneAgent(agent), nil
}

func (m *AgentStore) Get(_ context.Context, name string) (schema.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	agent, exists := m.agents[name]
	if !exists {
		return schema.Agent{}, llm.ErrNotFound.Withf("agent %q", name)
	}
	return cloneAgent(agent), nil
}

func (m *AgentStore) List(_ context.Context, req schema.AgentListRequest) (schema.AgentList, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	body := make([]schema.Agent, 0, len(m.agents))
	for _, agent := range m.agents {
		body = append(body, cloneAgent(agent))
	}
	slices.SortFunc(body, func(a, b schema.Agent) int { return cmp.Compare(a.Name, b.Name) })
	count := uint(len(body))
	body = body[min(req.Offset, count):]
	if req.Limit != nil {
		body = body[:min(*req.Limit, uint(len(body)))]
	}
	return schema.AgentList{AgentListRequest: req, Count: count, Body: body}, nil
}

func (m *AgentStore) Put(_ context.Context, agent schema.Agent) (schema.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, exists := m.agents[agent.Name]
	if !exists {
		return schema.Agent{}, llm.ErrNotFound.Withf("agent %q", agent.Name)
	} else if existing.Serial != agent.Serial {
		return schema.Agent{}, llm.ErrConflict.Withf("agent %q was changed", agent.Name)
	}
	agent.Serial++
	agent.Created = existing.Created
	agent.Modified = time.Now()
	m.agents[agent.Name] = cloneAgent(agent)
	return cloneAgent(agent), nil
}

func (m *AgentStore) Delete(_ context.Context, name string, serial uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, exists := m.agents[name]
	if !exists {
		return nil
	} else if existing.Serial != serial {
		return llm.ErrConflict.Withf("agent %q was changed", name)
	}
	delete(m.agents, name)
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// cloneAgent returns a copy of the agent, which shares none of its pointers,
// maps or slices, so an agent which is stored, or returned, can't be changed
// but by Put
func cloneAgent(agent schema.Agent) schema.Agent {
	data, err := json.Marshal(agent)
	if err != nil {
		panic(err)
	}
	var result schema.Agent
	if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return result
}
