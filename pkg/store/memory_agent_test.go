package store_test

import (
	"context"
	"testing"

	schema "github.com/mutablelogic/go-llm/pkg/schema"
	store "github.com/mutablelogic/go-llm/pkg/store"
	assert "github.com/stretchr/testify/assert"
)

var testAgentMeta = schema.AgentMeta{
	GeneratorMeta: schema.GeneratorMeta{
		Model:    "test-model",
		Provider: "test-provider",
	},
	Name:  "test-agent",
	Title: "Test Agent Title",
}

func boolPtr(v bool) *bool { return &v }

func Test_memory_agent_001(t *testing.T) {
	// NewMemoryAgentStore returns a non-nil store
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()
	assert.NotNil(s)
}

func Test_memory_agent_002(t *testing.T) {
	// CreateAgent succeeds with valid metadata
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()
	a, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)
	assert.NotNil(a)
	assert.NotEmpty(a.ID)
	assert.Equal("test-agent", a.Name)
	assert.Equal("Test Agent Title", a.Title)
	assert.Equal("test-model", a.Model)
	assert.Equal("test-provider", a.Provider)
	assert.Equal(uint(1), a.Version)
	assert.False(a.Created.IsZero())
}

func Test_memory_agent_003(t *testing.T) {
	// CreateAgent fails with invalid name
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	_, err := s.CreateAgent(context.TODO(), schema.AgentMeta{Name: ""})
	assert.Error(err)

	_, err = s.CreateAgent(context.TODO(), schema.AgentMeta{Name: "123bad"})
	assert.Error(err)

	_, err = s.CreateAgent(context.TODO(), schema.AgentMeta{Name: "has spaces"})
	assert.Error(err)
}

func Test_memory_agent_004(t *testing.T) {
	// CreateAgent fails on duplicate name
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	_, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	_, err = s.CreateAgent(context.TODO(), testAgentMeta)
	assert.Error(err)
}

func Test_memory_agent_005(t *testing.T) {
	// CreateAgent assigns unique IDs
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	a1, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
		Name:          "agent-one",
	})
	assert.NoError(err)

	a2, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
		Name:          "agent-two",
	})
	assert.NoError(err)

	assert.NotEqual(a1.ID, a2.ID)
}

func Test_memory_agent_006(t *testing.T) {
	// GetAgent by ID
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	created, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	got, err := s.GetAgent(context.TODO(), created.ID)
	assert.NoError(err)
	assert.Equal(created.ID, got.ID)
	assert.Equal(created.Name, got.Name)
}

func Test_memory_agent_007(t *testing.T) {
	// GetAgent by name
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	created, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	got, err := s.GetAgent(context.TODO(), "test-agent")
	assert.NoError(err)
	assert.Equal(created.ID, got.ID)
}

func Test_memory_agent_008(t *testing.T) {
	// GetAgent returns error for unknown ID/name
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	_, err := s.GetAgent(context.TODO(), "nonexistent")
	assert.Error(err)
}

func Test_memory_agent_009(t *testing.T) {
	// DeleteAgent by ID
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	a, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	err = s.DeleteAgent(context.TODO(), a.ID)
	assert.NoError(err)

	_, err = s.GetAgent(context.TODO(), a.ID)
	assert.Error(err)
}

func Test_memory_agent_010(t *testing.T) {
	// DeleteAgent by name removes all agents with that name
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	_, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	err = s.DeleteAgent(context.TODO(), "test-agent")
	assert.NoError(err)

	_, err = s.GetAgent(context.TODO(), "test-agent")
	assert.Error(err)
}

func Test_memory_agent_011(t *testing.T) {
	// DeleteAgent returns error for unknown ID/name
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	err := s.DeleteAgent(context.TODO(), "nonexistent")
	assert.Error(err)
}

func Test_memory_agent_012(t *testing.T) {
	// After delete, same name can be reused
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	_, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	err = s.DeleteAgent(context.TODO(), "test-agent")
	assert.NoError(err)

	a, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)
	assert.NotNil(a)
}

func Test_memory_agent_013(t *testing.T) {
	// UpdateAgent no-op when metadata is identical
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	created, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	updated, err := s.UpdateAgent(context.TODO(), created.ID, testAgentMeta)
	assert.NoError(err)
	assert.Equal(created.ID, updated.ID)
	assert.Equal(uint(1), updated.Version)
}

func Test_memory_agent_014(t *testing.T) {
	// UpdateAgent creates a new version with incremented version number
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	created, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	newMeta := testAgentMeta
	newMeta.Title = "Updated Title"
	updated, err := s.UpdateAgent(context.TODO(), created.ID, newMeta)
	assert.NoError(err)

	assert.NotEqual(created.ID, updated.ID)
	assert.Equal("test-agent", updated.Name)
	assert.Equal("Updated Title", updated.Title)
	assert.Equal(uint(2), updated.Version)
	assert.False(updated.Created.IsZero())
}

func Test_memory_agent_015(t *testing.T) {
	// UpdateAgent by name
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	_, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	newMeta := testAgentMeta
	newMeta.Description = "new description"
	updated, err := s.UpdateAgent(context.TODO(), "test-agent", newMeta)
	assert.NoError(err)
	assert.Equal(uint(2), updated.Version)
	assert.Equal("new description", updated.Description)
}

func Test_memory_agent_016(t *testing.T) {
	// UpdateAgent rejects name change
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	created, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	newMeta := testAgentMeta
	newMeta.Name = "different-name"
	_, err = s.UpdateAgent(context.TODO(), created.ID, newMeta)
	assert.Error(err)
}

func Test_memory_agent_017(t *testing.T) {
	// UpdateAgent returns error for nonexistent agent
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	_, err := s.UpdateAgent(context.TODO(), "nonexistent", testAgentMeta)
	assert.Error(err)
}

func Test_memory_agent_018(t *testing.T) {
	// UpdateAgent fills in name from existing agent when meta.Name is empty
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	created, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	meta := schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "new-model"},
		Title:         "New Title",
	}
	updated, err := s.UpdateAgent(context.TODO(), created.ID, meta)
	assert.NoError(err)
	assert.Equal("test-agent", updated.Name)
	assert.Equal("new-model", updated.Model)
	assert.Equal("New Title", updated.Title)
	// Verify existing fields are preserved (not cleared)
	assert.Equal("test-provider", updated.Provider, "Provider should be preserved from original")
	assert.Equal(uint(2), updated.Version)
}

func Test_memory_agent_019(t *testing.T) {
	// Multiple updates increment version correctly
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	a, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)
	assert.Equal(uint(1), a.Version)

	meta := testAgentMeta
	meta.Title = "Version 2"
	a, err = s.UpdateAgent(context.TODO(), a.ID, meta)
	assert.NoError(err)
	assert.Equal(uint(2), a.Version)

	meta.Title = "Version 3"
	a, err = s.UpdateAgent(context.TODO(), a.ID, meta)
	assert.NoError(err)
	assert.Equal(uint(3), a.Version)
}

func Test_memory_agent_020(t *testing.T) {
	// GetAgent by name returns the latest version after update
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	_, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	meta := testAgentMeta
	meta.Title = "Updated"
	_, err = s.UpdateAgent(context.TODO(), "test-agent", meta)
	assert.NoError(err)

	latest, err := s.GetAgent(context.TODO(), "test-agent")
	assert.NoError(err)
	assert.Equal("Updated", latest.Title)
	assert.Equal(uint(2), latest.Version)
}

func Test_memory_agent_021(t *testing.T) {
	// UpdateAgent no-op with Thinking field unchanged
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	meta := testAgentMeta
	meta.Thinking = boolPtr(true)
	meta.ThinkingBudget = 1000
	created, err := s.CreateAgent(context.TODO(), meta)
	assert.NoError(err)

	updated, err := s.UpdateAgent(context.TODO(), created.ID, meta)
	assert.NoError(err)
	assert.Equal(created.ID, updated.ID, "should be no-op")
}

func Test_memory_agent_022(t *testing.T) {
	// UpdateAgent detects Thinking field change
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	meta := testAgentMeta
	meta.Thinking = boolPtr(true)
	created, err := s.CreateAgent(context.TODO(), meta)
	assert.NoError(err)

	meta.Thinking = boolPtr(false)
	updated, err := s.UpdateAgent(context.TODO(), created.ID, meta)
	assert.NoError(err)
	assert.NotEqual(created.ID, updated.ID)
	assert.Equal(uint(2), updated.Version)
}

func Test_memory_agent_023(t *testing.T) {
	// UpdateAgent detects JSONSchema change
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	meta := testAgentMeta
	meta.Format = schema.JSONSchema(`{"type":"object"}`)
	created, err := s.CreateAgent(context.TODO(), meta)
	assert.NoError(err)

	meta.Format = schema.JSONSchema(`{"type":"array"}`)
	updated, err := s.UpdateAgent(context.TODO(), created.ID, meta)
	assert.NoError(err)
	assert.NotEqual(created.ID, updated.ID)
	assert.Equal(uint(2), updated.Version)
}

func Test_memory_agent_024(t *testing.T) {
	// UpdateAgent no-op with equivalent JSONSchema (different whitespace)
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	meta := testAgentMeta
	meta.Format = schema.JSONSchema(`{"type":"object"}`)
	created, err := s.CreateAgent(context.TODO(), meta)
	assert.NoError(err)

	meta.Format = schema.JSONSchema(`{ "type" : "object" }`)
	updated, err := s.UpdateAgent(context.TODO(), created.ID, meta)
	assert.NoError(err)
	assert.Equal(created.ID, updated.ID, "equivalent JSON should be no-op")
}

func Test_memory_agent_025(t *testing.T) {
	// Delete by ID preserves other agents
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	a1, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
		Name:          "agent-one",
	})
	assert.NoError(err)

	a2, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
		Name:          "agent-two",
	})
	assert.NoError(err)

	err = s.DeleteAgent(context.TODO(), a1.ID)
	assert.NoError(err)

	_, err = s.GetAgent(context.TODO(), a1.ID)
	assert.Error(err)

	got, err := s.GetAgent(context.TODO(), a2.ID)
	assert.NoError(err)
	assert.Equal(a2.ID, got.ID)
}

func uintPtr(v uint) *uint { return &v }

func Test_memory_agent_026(t *testing.T) {
	// ListAgents on empty store returns zero results
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{})
	assert.NoError(err)
	assert.Equal(uint(0), resp.Count)
	assert.Empty(resp.Body)
}

func Test_memory_agent_027(t *testing.T) {
	// ListAgents without filter returns latest version per name
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	_, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	// Update to create version 2
	meta := testAgentMeta
	meta.Title = "Updated Title"
	_, err = s.UpdateAgent(context.TODO(), "test-agent", meta)
	assert.NoError(err)

	resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{})
	assert.NoError(err)
	assert.Equal(uint(1), resp.Count)
	assert.Len(resp.Body, 1)
	assert.Equal("Updated Title", resp.Body[0].Title)
	assert.Equal(uint(2), resp.Body[0].Version)
}

func Test_memory_agent_028(t *testing.T) {
	// ListAgents filtered by name returns all versions
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	_, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	meta := testAgentMeta
	meta.Title = "V2"
	_, err = s.UpdateAgent(context.TODO(), "test-agent", meta)
	assert.NoError(err)

	meta.Title = "V3"
	_, err = s.UpdateAgent(context.TODO(), "test-agent", meta)
	assert.NoError(err)

	resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{Name: "test-agent"})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Len(resp.Body, 3)
	// Most recent first
	assert.Equal(uint(3), resp.Body[0].Version)
	assert.Equal(uint(2), resp.Body[1].Version)
	assert.Equal(uint(1), resp.Body[2].Version)
}

func Test_memory_agent_029(t *testing.T) {
	// ListAgents filtered by name that doesn't exist returns empty
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	_, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{Name: "nonexistent"})
	assert.NoError(err)
	assert.Equal(uint(0), resp.Count)
	assert.Empty(resp.Body)
}

func Test_memory_agent_030(t *testing.T) {
	// ListAgents without filter returns one entry per unique name
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	_, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
		Name:          "alpha",
	})
	assert.NoError(err)

	_, err = s.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
		Name:          "beta",
	})
	assert.NoError(err)

	_, err = s.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
		Name:          "gamma",
	})
	assert.NoError(err)

	resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Len(resp.Body, 3)
}

func Test_memory_agent_031(t *testing.T) {
	// ListAgents pagination with limit
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	for _, name := range []string{"a-agent", "b-agent", "c-agent", "d-agent"} {
		_, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
			Name:          name,
		})
		assert.NoError(err)
	}

	resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{Limit: uintPtr(2)})
	assert.NoError(err)
	assert.Equal(uint(4), resp.Count)
	assert.Len(resp.Body, 2)
}

func Test_memory_agent_032(t *testing.T) {
	// ListAgents pagination with offset
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	for _, name := range []string{"a-agent", "b-agent", "c-agent"} {
		_, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
			Name:          name,
		})
		assert.NoError(err)
	}

	resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{Offset: 1, Limit: uintPtr(2)})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Equal(uint(1), resp.Offset)
	assert.Len(resp.Body, 2)
}

func Test_memory_agent_033(t *testing.T) {
	// ListAgents offset beyond total returns empty body
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	_, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{Offset: 100})
	assert.NoError(err)
	assert.Equal(uint(1), resp.Count)
	assert.Empty(resp.Body)
}

func Test_memory_agent_034(t *testing.T) {
	// ListAgents with name and version returns specific version
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	created, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)
	assert.Equal(uint(1), created.Version)

	// Update to create version 2
	meta2 := testAgentMeta
	meta2.Title = "Updated Agent Title"
	updated, err := s.UpdateAgent(context.TODO(), created.ID, meta2)
	assert.NoError(err)
	assert.Equal(uint(2), updated.Version)

	// Get version 1
	v1 := uintPtr(1)
	resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{Name: testAgentMeta.Name, Version: v1})
	assert.NoError(err)
	assert.Equal(uint(1), resp.Count)
	assert.Len(resp.Body, 1)
	assert.Equal(uint(1), resp.Body[0].Version)
	assert.Equal("Test Agent Title", resp.Body[0].Title)

	// Get version 2
	v2 := uintPtr(2)
	resp, err = s.ListAgents(context.TODO(), schema.ListAgentRequest{Name: testAgentMeta.Name, Version: v2})
	assert.NoError(err)
	assert.Equal(uint(1), resp.Count)
	assert.Len(resp.Body, 1)
	assert.Equal(uint(2), resp.Body[0].Version)
	assert.Equal("Updated Agent Title", resp.Body[0].Title)

	// Get non-existent version
	v99 := uintPtr(99)
	resp, err = s.ListAgents(context.TODO(), schema.ListAgentRequest{Name: testAgentMeta.Name, Version: v99})
	assert.NoError(err)
	assert.Equal(uint(0), resp.Count)
	assert.Empty(resp.Body)
}

func Test_memory_agent_035(t *testing.T) {
	// UpdateAgent with partial meta merges onto existing (not full replacement)
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	full := schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{
			Model:          "gpt-4",
			Provider:       "openai",
			SystemPrompt:   "You are helpful",
			Format:         schema.JSONSchema(`{"type":"object"}`),
			ThinkingBudget: 500,
		},
		Name:        "full-agent",
		Title:       "Full Agent",
		Description: "A fully configured agent",
		Template:    "Hello {{.Name}}",
		Tools:       []string{"tool-a", "tool-b"},
	}
	created, err := s.CreateAgent(context.TODO(), full)
	assert.NoError(err)

	// Update with only Title — everything else should be preserved
	partial := schema.AgentMeta{Title: "New Title Only"}
	updated, err := s.UpdateAgent(context.TODO(), created.ID, partial)
	assert.NoError(err)
	assert.Equal(uint(2), updated.Version)
	assert.Equal("New Title Only", updated.Title)

	// All other fields preserved
	assert.Equal("full-agent", updated.Name)
	assert.Equal("A fully configured agent", updated.Description)
	assert.Equal("Hello {{.Name}}", updated.Template)
	assert.Equal("gpt-4", updated.Model)
	assert.Equal("openai", updated.Provider)
	assert.Equal("You are helpful", updated.SystemPrompt)
	assert.Equal(uint(500), updated.ThinkingBudget)
	assert.Equal([]string{"tool-a", "tool-b"}, updated.Tools)
}

func Test_memory_agent_036(t *testing.T) {
	// Delete latest version by ID; GetAgent by name returns previous version
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()

	created, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	meta := testAgentMeta
	meta.Title = "Version 2"
	v2, err := s.UpdateAgent(context.TODO(), created.ID, meta)
	assert.NoError(err)
	assert.Equal(uint(2), v2.Version)

	// Delete version 2 by its ID
	err = s.DeleteAgent(context.TODO(), v2.ID)
	assert.NoError(err)

	// Name lookup should still work and return version 1
	got, err := s.GetAgent(context.TODO(), "test-agent")
	assert.NoError(err)
	assert.Equal(uint(1), got.Version)
	assert.Equal(created.ID, got.ID)
	assert.Equal("Test Agent Title", got.Title)
}
