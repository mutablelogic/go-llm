package store_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var testAgentMeta = schema.AgentMeta{
	GeneratorMeta: schema.GeneratorMeta{
		Model:    "test-model",
		Provider: "test-provider",
	},
	Name:  "test-agent",
	Title: "Test Agent Title",
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

// agentStoreTests contains tests that every schema.AgentStore
// implementation must pass.
type agentStoreTest struct {
	Name string
	Fn   func(*testing.T, schema.AgentStore)
}

var agentStoreTests = []agentStoreTest{
	// Create
	{"CreateSuccess", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
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
	}},
	{"CreateInvalidNameEmpty", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		_, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
			Name:          "",
		})
		assert.Error(err)
		assert.ErrorIs(err, llm.ErrBadParameter)
	}},
	{"CreateInvalidNameDigit", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		_, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
			Name:          "123bad",
		})
		assert.Error(err)
		assert.ErrorIs(err, llm.ErrBadParameter)
	}},
	{"CreateInvalidNameSpaces", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		_, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
			Name:          "has spaces",
		})
		assert.Error(err)
		assert.ErrorIs(err, llm.ErrBadParameter)
	}},
	{"CreateDuplicateName", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		_, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
		_, err = s.CreateAgent(context.TODO(), testAgentMeta)
		assert.Error(err)
		assert.ErrorIs(err, llm.ErrConflict)
	}},
	{"CreateUniqueIDs", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		a1, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
			Name:          "agent_one",
		})
		assert.NoError(err)
		a2, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
			Name:          "agent_two",
		})
		assert.NoError(err)
		assert.NotEqual(a1.ID, a2.ID)
	}},
	{"CreateAllFields", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		meta := schema.AgentMeta{
			GeneratorMeta: schema.GeneratorMeta{
				Model:        "gpt-4",
				Provider:     "openai",
				SystemPrompt: "You are helpful.",
			},
			Name:        "full_agent",
			Title:       "Full Agent",
			Description: "An agent with all fields set",
			Tools:       []string{"tool_a", "tool_b"},
		}
		a, err := s.CreateAgent(context.TODO(), meta)
		assert.NoError(err)
		assert.Equal("full_agent", a.Name)
		assert.Equal("Full Agent", a.Title)
		assert.Equal("An agent with all fields set", a.Description)
		assert.Equal("gpt-4", a.Model)
		assert.Equal("openai", a.Provider)
		assert.Equal("You are helpful.", a.SystemPrompt)
		assert.Equal([]string{"tool_a", "tool_b"}, a.Tools)
	}},

	// Get
	{"GetByID", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		created, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
		got, err := s.GetAgent(context.TODO(), created.ID)
		assert.NoError(err)
		assert.Equal(created.ID, got.ID)
		assert.Equal(created.Name, got.Name)
		assert.Equal(created.Model, got.Model)
		assert.Equal(created.Version, got.Version)
	}},
	{"GetByName", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		created, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
		got, err := s.GetAgent(context.TODO(), created.Name)
		assert.NoError(err)
		assert.Equal(created.ID, got.ID)
		assert.Equal(created.Name, got.Name)
	}},
	{"GetNotFound", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		_, err := s.GetAgent(context.TODO(), "nonexistent")
		assert.Error(err)
		assert.ErrorIs(err, llm.ErrNotFound)
	}},

	// List
	{"ListEmpty", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{})
		assert.NoError(err)
		assert.Equal(uint(0), resp.Count)
		assert.Empty(resp.Body)
	}},
	{"ListAll", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		for _, name := range []string{"alpha", "beta", "gamma"} {
			_, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
				GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
				Name:          name,
			})
			assert.NoError(err)
		}
		resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{})
		assert.NoError(err)
		assert.Equal(uint(3), resp.Count)
		assert.Len(resp.Body, 3)
	}},
	{"ListFilterByName", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		for _, name := range []string{"alpha", "beta"} {
			_, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
				GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
				Name:          name,
			})
			assert.NoError(err)
		}
		resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{Name: "alpha"})
		assert.NoError(err)
		assert.Equal(uint(1), resp.Count)
		assert.Len(resp.Body, 1)
		assert.Equal("alpha", resp.Body[0].Name)
	}},
	{"ListFilterNotFound", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		_, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
		resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{Name: "nonexistent"})
		assert.NoError(err)
		assert.Equal(uint(0), resp.Count)
		assert.Empty(resp.Body)
	}},
	{"ListPaginationLimit", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		for _, name := range []string{"a_agent", "b_agent", "c_agent", "d_agent"} {
			_, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
				GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
				Name:          name,
			})
			assert.NoError(err)
		}
		resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{Limit: types.Ptr(uint(2))})
		assert.NoError(err)
		assert.Equal(uint(4), resp.Count)
		assert.Len(resp.Body, 2)
	}},
	{"ListPaginationOffset", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		for _, name := range []string{"a_agent", "b_agent", "c_agent"} {
			_, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
				GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
				Name:          name,
			})
			assert.NoError(err)
		}
		resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{Offset: 1, Limit: types.Ptr(uint(2))})
		assert.NoError(err)
		assert.Equal(uint(3), resp.Count)
		assert.Equal(uint(1), resp.Offset)
		assert.Len(resp.Body, 2)
	}},
	{"ListOffsetBeyondTotal", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		_, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
		resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{Offset: 100})
		assert.NoError(err)
		assert.Equal(uint(1), resp.Count)
		assert.Empty(resp.Body)
	}},

	// Delete
	{"DeleteByID", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		a, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
		err = s.DeleteAgent(context.TODO(), a.ID)
		assert.NoError(err)
		_, err = s.GetAgent(context.TODO(), a.ID)
		assert.Error(err)
	}},
	{"DeleteByName", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		_, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
		err = s.DeleteAgent(context.TODO(), "test-agent")
		assert.NoError(err)
		_, err = s.GetAgent(context.TODO(), "test-agent")
		assert.Error(err)
	}},
	{"DeleteNotFound", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		err := s.DeleteAgent(context.TODO(), "nonexistent")
		assert.Error(err)
	}},
	{"DeleteThenReuseName", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		_, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
		err = s.DeleteAgent(context.TODO(), "test-agent")
		assert.NoError(err)
		a, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
		assert.NotNil(a)
	}},
	{"DeletePreservesOthers", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		a1, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
			Name:          "agent_one",
		})
		assert.NoError(err)
		a2, err := s.CreateAgent(context.TODO(), schema.AgentMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
			Name:          "agent_two",
		})
		assert.NoError(err)
		err = s.DeleteAgent(context.TODO(), a1.ID)
		assert.NoError(err)
		_, err = s.GetAgent(context.TODO(), a1.ID)
		assert.Error(err)
		got, err := s.GetAgent(context.TODO(), a2.ID)
		assert.NoError(err)
		assert.Equal(a2.ID, got.ID)
	}},

	// Update
	{"UpdateNoOp", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		created, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
		updated, err := s.UpdateAgent(context.TODO(), created.ID, testAgentMeta)
		assert.NoError(err)
		assert.Equal(created.ID, updated.ID)
		assert.Equal(uint(1), updated.Version)
	}},
	{"UpdateNewVersion", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
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
	}},
	{"UpdateByName", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		_, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
		newMeta := testAgentMeta
		newMeta.Description = "new description"
		updated, err := s.UpdateAgent(context.TODO(), "test-agent", newMeta)
		assert.NoError(err)
		assert.Equal(uint(2), updated.Version)
		assert.Equal("new description", updated.Description)
	}},
	{"UpdateRejectsNameChange", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		created, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
		newMeta := testAgentMeta
		newMeta.Name = "different-name"
		_, err = s.UpdateAgent(context.TODO(), created.ID, newMeta)
		assert.Error(err)
	}},
	{"UpdateNotFound", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		_, err := s.UpdateAgent(context.TODO(), "nonexistent", testAgentMeta)
		assert.Error(err)
	}},
	{"UpdateFillsName", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
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
		assert.Equal("test-provider", updated.Provider, "Provider should be preserved from original")
		assert.Equal(uint(2), updated.Version)
	}},
	{"UpdateMultipleVersions", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
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
	}},
	{"UpdateGetByNameLatest", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
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
	}},
	{"UpdateThinkingNoOp", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		meta := testAgentMeta
		meta.Thinking = types.Ptr(true)
		meta.ThinkingBudget = 1000
		created, err := s.CreateAgent(context.TODO(), meta)
		assert.NoError(err)
		updated, err := s.UpdateAgent(context.TODO(), created.ID, meta)
		assert.NoError(err)
		assert.Equal(created.ID, updated.ID, "should be no-op")
	}},
	{"UpdateThinkingChange", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		meta := testAgentMeta
		meta.Thinking = types.Ptr(true)
		created, err := s.CreateAgent(context.TODO(), meta)
		assert.NoError(err)
		meta.Thinking = types.Ptr(false)
		updated, err := s.UpdateAgent(context.TODO(), created.ID, meta)
		assert.NoError(err)
		assert.NotEqual(created.ID, updated.ID)
		assert.Equal(uint(2), updated.Version)
	}},
	{"UpdateJSONSchemaChange", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		meta := testAgentMeta
		meta.Format = schema.JSONSchema(`{"type":"object"}`)
		created, err := s.CreateAgent(context.TODO(), meta)
		assert.NoError(err)
		meta.Format = schema.JSONSchema(`{"type":"array"}`)
		updated, err := s.UpdateAgent(context.TODO(), created.ID, meta)
		assert.NoError(err)
		assert.NotEqual(created.ID, updated.ID)
		assert.Equal(uint(2), updated.Version)
	}},
	{"UpdateJSONSchemaNoOp", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		meta := testAgentMeta
		meta.Format = schema.JSONSchema(`{"type":"object"}`)
		created, err := s.CreateAgent(context.TODO(), meta)
		assert.NoError(err)
		meta.Format = schema.JSONSchema(`{ "type" : "object" }`)
		updated, err := s.UpdateAgent(context.TODO(), created.ID, meta)
		assert.NoError(err)
		assert.Equal(created.ID, updated.ID, "equivalent JSON should be no-op")
	}},
	{"UpdatePartialMerge", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
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
		partial := schema.AgentMeta{Title: "New Title Only"}
		updated, err := s.UpdateAgent(context.TODO(), created.ID, partial)
		assert.NoError(err)
		assert.Equal(uint(2), updated.Version)
		assert.Equal("New Title Only", updated.Title)
		assert.Equal("full-agent", updated.Name)
		assert.Equal("A fully configured agent", updated.Description)
		assert.Equal("Hello {{.Name}}", updated.Template)
		assert.Equal("gpt-4", updated.Model)
		assert.Equal("openai", updated.Provider)
		assert.Equal("You are helpful", updated.SystemPrompt)
		assert.Equal(uint(500), updated.ThinkingBudget)
		assert.Equal([]string{"tool-a", "tool-b"}, updated.Tools)
	}},

	// Update + List combinations
	{"ListLatestAfterUpdate", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		_, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
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
	}},
	{"ListAllVersionsByName", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
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
		assert.Equal(uint(3), resp.Body[0].Version)
		assert.Equal(uint(2), resp.Body[1].Version)
		assert.Equal(uint(1), resp.Body[2].Version)
	}},
	{"ListVersionFilter", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		created, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
		assert.Equal(uint(1), created.Version)
		meta2 := testAgentMeta
		meta2.Title = "Updated Agent Title"
		updated, err := s.UpdateAgent(context.TODO(), created.ID, meta2)
		assert.NoError(err)
		assert.Equal(uint(2), updated.Version)
		v1 := types.Ptr(uint(1))
		resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{Name: testAgentMeta.Name, Version: v1})
		assert.NoError(err)
		assert.Equal(uint(1), resp.Count)
		assert.Len(resp.Body, 1)
		assert.Equal(uint(1), resp.Body[0].Version)
		assert.Equal("Test Agent Title", resp.Body[0].Title)
		v2 := types.Ptr(uint(2))
		resp, err = s.ListAgents(context.TODO(), schema.ListAgentRequest{Name: testAgentMeta.Name, Version: v2})
		assert.NoError(err)
		assert.Equal(uint(1), resp.Count)
		assert.Len(resp.Body, 1)
		assert.Equal(uint(2), resp.Body[0].Version)
		assert.Equal("Updated Agent Title", resp.Body[0].Title)
		v99 := types.Ptr(uint(99))
		resp, err = s.ListAgents(context.TODO(), schema.ListAgentRequest{Name: testAgentMeta.Name, Version: v99})
		assert.NoError(err)
		assert.Equal(uint(0), resp.Count)
		assert.Empty(resp.Body)
	}},

	// Update + Delete combination
	{"DeleteVersionGetPrevious", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		created, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)
		meta := testAgentMeta
		meta.Title = "Version 2"
		v2, err := s.UpdateAgent(context.TODO(), created.ID, meta)
		assert.NoError(err)
		assert.Equal(uint(2), v2.Version)
		err = s.DeleteAgent(context.TODO(), v2.ID)
		assert.NoError(err)
		got, err := s.GetAgent(context.TODO(), "test-agent")
		assert.NoError(err)
		assert.Equal(uint(1), got.Version)
		assert.Equal(created.ID, got.ID)
		assert.Equal("Test Agent Title", got.Title)
	}},

	// Concurrency
	{"ConcurrentCreates", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		const n = 20
		var wg sync.WaitGroup
		errs := make([]error, n)
		wg.Add(n)
		for i := range n {
			go func(i int) {
				defer wg.Done()
				_, errs[i] = s.CreateAgent(context.TODO(), schema.AgentMeta{
					GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
					Name:          fmt.Sprintf("agent_%03d", i),
				})
			}(i)
		}
		wg.Wait()
		for i, err := range errs {
			assert.NoError(err, "agent_%03d", i)
		}
		resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{})
		assert.NoError(err)
		assert.Equal(uint(n), resp.Count)
	}},
	{"ConcurrentReadsAndWrites", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		created, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)

		const n = 20
		var wg sync.WaitGroup
		wg.Add(n * 2)
		// Half readers, half listers
		for range n {
			go func() {
				defer wg.Done()
				s.GetAgent(context.TODO(), created.ID)
			}()
			go func() {
				defer wg.Done()
				s.ListAgents(context.TODO(), schema.ListAgentRequest{})
			}()
		}
		wg.Wait()
	}},
	{"ConcurrentMixedOps", func(t *testing.T, s schema.AgentStore) {
		// Seed the store
		for i := range 5 {
			s.CreateAgent(context.TODO(), schema.AgentMeta{
				GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
				Name:          fmt.Sprintf("seed_%d", i),
			})
		}

		const n = 10
		var wg sync.WaitGroup
		wg.Add(n * 4)
		for i := range n {
			// Create
			go func(i int) {
				defer wg.Done()
				s.CreateAgent(context.TODO(), schema.AgentMeta{
					GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
					Name:          fmt.Sprintf("mixed_%d", i),
				})
			}(i)
			// Get
			go func() {
				defer wg.Done()
				s.GetAgent(context.TODO(), "seed_0")
			}()
			// List
			go func() {
				defer wg.Done()
				s.ListAgents(context.TODO(), schema.ListAgentRequest{})
			}()
			// Update
			go func(i int) {
				defer wg.Done()
				s.UpdateAgent(context.TODO(), "seed_0", schema.AgentMeta{
					Title: fmt.Sprintf("updated_%d", i),
				})
			}(i)
		}
		wg.Wait()
	}},
	{"ConcurrentUpdates", func(t *testing.T, s schema.AgentStore) {
		assert := assert.New(t)
		_, err := s.CreateAgent(context.TODO(), testAgentMeta)
		assert.NoError(err)

		const n = 20
		var wg sync.WaitGroup
		wg.Add(n)
		for i := range n {
			go func(i int) {
				defer wg.Done()
				s.UpdateAgent(context.TODO(), "test-agent", schema.AgentMeta{
					Title: fmt.Sprintf("title_%d", i),
				})
			}(i)
		}
		wg.Wait()

		// The agent should still be retrievable
		got, err := s.GetAgent(context.TODO(), "test-agent")
		assert.NoError(err)
		assert.NotEmpty(got.Title)
		assert.True(got.Version >= 2, "expected at least one update to succeed")
	}},
}



// runAgentStoreTests runs every shared behavioural test against a store
// implementation. The factory is called once per subtest so each gets a
// clean, independent store.
func runAgentStoreTests(t *testing.T, factory func() schema.AgentStore) {
	t.Helper()
	for _, tt := range agentStoreTests {
		t.Run(tt.Name, func(t *testing.T) {
			tt.Fn(t, factory())
		})
	}
}
