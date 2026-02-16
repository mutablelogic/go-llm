package manager

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// AGENT TESTS

// Test CreateAgent with default in-memory store
func Test_agent_001(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	a, err := m.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "test-agent",
	})
	assert.NoError(err)
	assert.NotNil(a)
	assert.NotEmpty(a.ID)
	assert.Equal("test-agent", a.Name)
	assert.Equal("model-1", a.Model)
	assert.Equal("provider-1", a.Provider)
	assert.Equal(uint(1), a.Version)
}

// Test CreateAgent fills in provider from model resolution
func Test_agent_002(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	a, err := m.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "my-agent",
	})
	assert.NoError(err)
	assert.Equal("provider-1", a.Provider)
}

// Test CreateAgent without model (model is optional)
func Test_agent_003(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	a, err := m.CreateAgent(context.TODO(), schema.AgentMeta{
		Name: "no-model-agent",
	})
	assert.NoError(err)
	assert.NotNil(a)
	assert.Empty(a.Model)
	assert.Empty(a.Provider)
}

// Test CreateAgent with unknown model returns error
func Test_agent_004(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	_, err = m.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "nonexistent"},
		Name:          "bad-model-agent",
	})
	assert.Error(err)
}

// Test WithAgentStore rejects nil
func Test_agent_005(t *testing.T) {
	assert := assert.New(t)

	_, err := NewManager(WithAgentStore(nil))
	assert.ErrorIs(err, llm.ErrBadParameter)
}

// Test GetAgent by ID
func Test_agent_006(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	created, err := m.CreateAgent(context.TODO(), schema.AgentMeta{
		Name: "get-agent",
	})
	assert.NoError(err)

	got, err := m.GetAgent(context.TODO(), created.ID)
	assert.NoError(err)
	assert.Equal(created.ID, got.ID)
	assert.Equal("get-agent", got.Name)
}

// Test GetAgent by name
func Test_agent_007(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	created, err := m.CreateAgent(context.TODO(), schema.AgentMeta{
		Name: "named-agent",
	})
	assert.NoError(err)

	got, err := m.GetAgent(context.TODO(), "named-agent")
	assert.NoError(err)
	assert.Equal(created.ID, got.ID)
}

// Test GetAgent not found
func Test_agent_008(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	_, err = m.GetAgent(context.TODO(), "nonexistent")
	assert.Error(err)
}

// Test DeleteAgent returns deleted agent
func Test_agent_009(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	created, err := m.CreateAgent(context.TODO(), schema.AgentMeta{
		Name: "delete-me",
	})
	assert.NoError(err)

	deleted, err := m.DeleteAgent(context.TODO(), created.ID)
	assert.NoError(err)
	assert.Equal(created.ID, deleted.ID)
	assert.Equal("delete-me", deleted.Name)

	// Should no longer exist
	_, err = m.GetAgent(context.TODO(), created.ID)
	assert.Error(err)
}

// Test DeleteAgent not found
func Test_agent_010(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	_, err = m.DeleteAgent(context.TODO(), "nonexistent")
	assert.Error(err)
}

// Test ListAgents returns created agents
func Test_agent_011(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	_, err = m.CreateAgent(context.TODO(), schema.AgentMeta{Name: "agent-a"})
	assert.NoError(err)
	_, err = m.CreateAgent(context.TODO(), schema.AgentMeta{Name: "agent-b"})
	assert.NoError(err)

	resp, err := m.ListAgents(context.TODO(), schema.ListAgentRequest{})
	assert.NoError(err)
	assert.Equal(uint(2), resp.Count)
	assert.Len(resp.Body, 2)
}

// Test ListAgents empty store
func Test_agent_012(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	resp, err := m.ListAgents(context.TODO(), schema.ListAgentRequest{})
	assert.NoError(err)
	assert.Equal(uint(0), resp.Count)
	assert.Empty(resp.Body)
}

// Test UpdateAgent creates new version
func Test_agent_013(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	created, err := m.CreateAgent(context.TODO(), schema.AgentMeta{
		Name:  "update-agent",
		Title: "Original",
	})
	assert.NoError(err)

	updated, err := m.UpdateAgent(context.TODO(), created.ID, schema.AgentMeta{
		Name:  "update-agent",
		Title: "Updated",
	})
	assert.NoError(err)
	assert.NotEqual(created.ID, updated.ID)
	assert.Equal("Updated", updated.Title)
	assert.Equal(uint(2), updated.Version)
}

// Test UpdateAgent no-op when unchanged
func Test_agent_014(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	meta := schema.AgentMeta{Name: "noop-agent", Title: "Same"}
	created, err := m.CreateAgent(context.TODO(), meta)
	assert.NoError(err)

	updated, err := m.UpdateAgent(context.TODO(), created.ID, meta)
	assert.NoError(err)
	assert.Equal(created.ID, updated.ID)
	assert.Equal(uint(1), updated.Version)
}

// Test UpdateAgent validates model when provided
func Test_agent_015(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	created, err := m.CreateAgent(context.TODO(), schema.AgentMeta{
		Name: "model-update-agent",
	})
	assert.NoError(err)

	// Valid model
	updated, err := m.UpdateAgent(context.TODO(), created.ID, schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "model-update-agent",
	})
	assert.NoError(err)
	assert.Equal("model-1", updated.Model)
	assert.Equal("provider-1", updated.Provider)

	// Invalid model
	_, err = m.UpdateAgent(context.TODO(), created.ID, schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "nonexistent"},
		Name:          "model-update-agent",
	})
	assert.Error(err)
}

// Test full CRUD lifecycle
func Test_agent_016(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	// Create
	a, err := m.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "lifecycle-agent",
		Title:         "V1",
	})
	assert.NoError(err)

	// Read
	got, err := m.GetAgent(context.TODO(), a.ID)
	assert.NoError(err)
	assert.Equal(a.ID, got.ID)

	// Update
	updated, err := m.UpdateAgent(context.TODO(), a.ID, schema.AgentMeta{
		Name:  "lifecycle-agent",
		Title: "V2",
	})
	assert.NoError(err)
	assert.Equal(uint(2), updated.Version)

	// List (should show only latest)
	resp, err := m.ListAgents(context.TODO(), schema.ListAgentRequest{})
	assert.NoError(err)
	assert.Equal(uint(1), resp.Count)
	assert.Equal("V2", resp.Body[0].Title)

	// Delete
	deleted, err := m.DeleteAgent(context.TODO(), "lifecycle-agent")
	assert.NoError(err)
	assert.NotNil(deleted)

	// Confirm gone
	resp, err = m.ListAgents(context.TODO(), schema.ListAgentRequest{})
	assert.NoError(err)
	assert.Equal(uint(0), resp.Count)
}

// Test UpdateAgent preserves old versions alongside new ones
func Test_agent_017(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	// Create v1
	v1, err := m.CreateAgent(context.TODO(), schema.AgentMeta{
		Name:  "versioned-agent",
		Title: "Version 1",
	})
	assert.NoError(err)
	assert.Equal(uint(1), v1.Version)

	// Update to v2
	v2, err := m.UpdateAgent(context.TODO(), v1.ID, schema.AgentMeta{
		Name:  "versioned-agent",
		Title: "Version 2",
	})
	assert.NoError(err)
	assert.Equal(uint(2), v2.Version)
	assert.NotEqual(v1.ID, v2.ID)

	// Update to v3
	v3, err := m.UpdateAgent(context.TODO(), v2.ID, schema.AgentMeta{
		Name:  "versioned-agent",
		Title: "Version 3",
	})
	assert.NoError(err)
	assert.Equal(uint(3), v3.Version)

	// All three versions should still be retrievable by their IDs
	got1, err := m.GetAgent(context.TODO(), v1.ID)
	assert.NoError(err)
	assert.Equal("Version 1", got1.Title)
	assert.Equal(uint(1), got1.Version)

	got2, err := m.GetAgent(context.TODO(), v2.ID)
	assert.NoError(err)
	assert.Equal("Version 2", got2.Title)
	assert.Equal(uint(2), got2.Version)

	got3, err := m.GetAgent(context.TODO(), v3.ID)
	assert.NoError(err)
	assert.Equal("Version 3", got3.Title)
	assert.Equal(uint(3), got3.Version)

	// GetAgent by name returns the latest version
	latest, err := m.GetAgent(context.TODO(), "versioned-agent")
	assert.NoError(err)
	assert.Equal(uint(3), latest.Version)
	assert.Equal(v3.ID, latest.ID)

	// ListAgents without filter returns only the latest
	resp, err := m.ListAgents(context.TODO(), schema.ListAgentRequest{})
	assert.NoError(err)
	assert.Equal(uint(1), resp.Count)
	assert.Equal(uint(3), resp.Body[0].Version)

	// ListAgents filtered by name returns all versions
	resp, err = m.ListAgents(context.TODO(), schema.ListAgentRequest{Name: "versioned-agent"})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Equal(uint(3), resp.Body[0].Version)
	assert.Equal(uint(2), resp.Body[1].Version)
	assert.Equal(uint(1), resp.Body[2].Version)
}
