package manager

import (
	"context"
	"encoding/json"
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

// Test CreateAgent normalizes nil Tools to empty slice
func Test_agent_001a(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	// Tools not set — should be normalized to empty, not nil
	a, err := m.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "no-tools-agent",
	})
	assert.NoError(err)
	assert.NotNil(a.Tools)
	assert.Empty(a.Tools)
}

// Test CreateAgent preserves explicit tools
func Test_agent_001b(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	a, err := m.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "tools-agent",
		Tools:         []string{"search", "calc"},
	})
	assert.NoError(err)
	assert.Equal([]string{"search", "calc"}, a.Tools)
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

///////////////////////////////////////////////////////////////////////////////
// CREATE AGENT SESSION TESTS

// Test CreateAgentSession basic: create agent with template, get session + text
func Test_runagent_001(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	// Create an agent with a simple template
	_, err = m.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "greeter",
		Template:      "Hello, {{ .name }}!",
	})
	assert.NoError(err)

	// Create the agent session
	resp, err := m.CreateAgentSession(context.TODO(), "greeter", schema.CreateAgentSessionRequest{
		Input: json.RawMessage(`{"name": "World"}`),
	})
	assert.NoError(err)
	assert.NotNil(resp)
	assert.NotEmpty(resp.Session)
	assert.Equal("Hello, World!", resp.Text)

	// Verify the session was created with agent labels
	session, err := m.GetSession(context.TODO(), resp.Session)
	assert.NoError(err)
	assert.Equal("greeter", session.Name)
	assert.Contains(session.Labels["agent"], "greeter@")
	assert.NotEmpty(session.Labels["agent_id"])
}

// Test CreateAgentSession with missing agent name
func Test_runagent_002(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	_, err = m.CreateAgentSession(context.TODO(), "", schema.CreateAgentSessionRequest{})
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrBadParameter)
}

// Test CreateAgentSession with non-existent agent
func Test_runagent_003(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	_, err = m.CreateAgentSession(context.TODO(), "nonexistent", schema.CreateAgentSessionRequest{})
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test CreateAgentSession inherits GeneratorMeta from parent session
func Test_runagent_004(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	// Create a parent session with a model
	parentSession, err := m.CreateSession(context.TODO(), schema.SessionMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "parent",
	})
	assert.NoError(err)

	// Agent without a model
	_, err = m.CreateAgent(context.TODO(), schema.AgentMeta{
		Name:     "no-model-agent",
		Template: "do something",
	})
	assert.NoError(err)

	// Create agent session with parent — inherits model from parent
	resp, err := m.CreateAgentSession(context.TODO(), "no-model-agent", schema.CreateAgentSessionRequest{
		Parent: parentSession.ID,
	})
	assert.NoError(err)
	assert.NotNil(resp)
	assert.NotEmpty(resp.Session)
	assert.Equal("do something", resp.Text)

	// Verify the child session inherited the model
	childSession, err := m.GetSession(context.TODO(), resp.Session)
	assert.NoError(err)
	assert.Equal("model-1", childSession.Model)
}

// Test CreateAgentSession with parent session ID in labels
func Test_runagent_005(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	// Create a real parent session
	parentSession, err := m.CreateSession(context.TODO(), schema.SessionMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "parent",
	})
	assert.NoError(err)

	_, err = m.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "child-agent",
		Template:      "child task",
	})
	assert.NoError(err)

	resp, err := m.CreateAgentSession(context.TODO(), "child-agent", schema.CreateAgentSessionRequest{
		Parent: parentSession.ID,
	})
	assert.NoError(err)

	session, err := m.GetSession(context.TODO(), resp.Session)
	assert.NoError(err)
	assert.Equal(parentSession.ID, session.Labels["parent"])
}

// Test CreateAgentSession with input validation failure
func Test_runagent_006(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	// Agent with input schema requiring a "language" field
	_, err = m.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "strict-agent",
		Template:      "translate to {{ .language }}",
		Input:         schema.JSONSchema(`{"type":"object","properties":{"language":{"type":"string"}},"required":["language"]}`),
	})
	assert.NoError(err)

	// Run without providing the required input field
	_, err = m.CreateAgentSession(context.TODO(), "strict-agent", schema.CreateAgentSessionRequest{
		Input: json.RawMessage(`{}`),
	})
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrBadParameter)
}

// Test CreateAgentSession with specific version via ID
func Test_runagent_007(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	// Create v1
	v1, err := m.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "versioned",
		Template:      "version one",
	})
	assert.NoError(err)

	// Update to v2
	_, err = m.UpdateAgent(context.TODO(), "versioned", schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "versioned",
		Template:      "version two",
	})
	assert.NoError(err)

	// Create session from v1 specifically using its ID
	resp, err := m.CreateAgentSession(context.TODO(), v1.ID, schema.CreateAgentSessionRequest{})
	assert.NoError(err)
	assert.Equal("version one", resp.Text)

	// Create session from latest (v2) by name
	resp, err = m.CreateAgentSession(context.TODO(), "versioned", schema.CreateAgentSessionRequest{})
	assert.NoError(err)
	assert.Equal("version two", resp.Text)
}

// Test CreateAgentSession with non-existent agent ID
func Test_runagent_008(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	_, err = m.CreateAgentSession(context.TODO(), "nonexistent-id-12345", schema.CreateAgentSessionRequest{})
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test CreateAgentSession returns tools from agent
func Test_runagent_009(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	_, err = m.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "tool-agent",
		Template:      "use tools",
		Tools:         []string{"search", "calculator"},
	})
	assert.NoError(err)

	resp, err := m.CreateAgentSession(context.TODO(), "tool-agent", schema.CreateAgentSessionRequest{})
	assert.NoError(err)
	assert.Equal([]string{"search", "calculator"}, resp.Tools)
}

// Test CreateAgentSession then Chat (two-phase flow)
func Test_runagent_010(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	_, err = m.CreateAgent(context.TODO(), schema.AgentMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Name:          "two-phase",
		Template:      "Hello, {{ .name }}!",
	})
	assert.NoError(err)

	// Phase 1: create session
	agentResp, err := m.CreateAgentSession(context.TODO(), "two-phase", schema.CreateAgentSessionRequest{
		Input: json.RawMessage(`{"name": "World"}`),
	})
	assert.NoError(err)
	assert.Equal("Hello, World!", agentResp.Text)

	// Phase 2: chat using the returned session, text, and tools
	chatResp, err := m.Chat(context.TODO(), schema.ChatRequest{
		ChatRequestCore: schema.ChatRequestCore{
			Session: agentResp.Session,
			Text:    agentResp.Text,
			Tools:   agentResp.Tools,
		},
	}, nil)
	assert.NoError(err)
	assert.NotNil(chatResp)
	assert.Equal(schema.RoleAssistant, chatResp.Role)
	assert.Equal(agentResp.Session, chatResp.Session)
	assert.Contains(*chatResp.Content[0].Text, "Hello, World!")
}
