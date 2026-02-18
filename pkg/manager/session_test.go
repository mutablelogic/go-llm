package manager

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	session "github.com/mutablelogic/go-llm/pkg/store"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// SESSION TESTS

// Test default memory store is used when no store is provided
func Test_session_001(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	// Should work with the default in-memory store
	s, err := m.CreateSession(context.TODO(), schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: "model-1"}})
	assert.NoError(err)
	assert.NotEmpty(s.ID)

	got, err := m.GetSession(context.TODO(), s.ID)
	assert.NoError(err)
	assert.Equal(s.ID, got.ID)

	deleted, err := m.DeleteSession(context.TODO(), s.ID)
	assert.NoError(err)
	assert.Equal(s.ID, deleted.ID)

	resp, err := m.ListSessions(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Equal(uint(0), resp.Count)
}

// Test WithSessionStore rejects nil store
func Test_session_002(t *testing.T) {
	assert := assert.New(t)

	_, err := NewManager(WithSessionStore(nil))
	assert.ErrorIs(err, llm.ErrBadParameter)
}

// Test CreateSession creates a session with correct fields
func Test_session_003(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	s, err := m.CreateSession(context.TODO(), schema.SessionMeta{
		Name:          "my chat",
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
	})
	assert.NoError(err)
	assert.NotNil(s)
	assert.NotEmpty(s.ID)
	assert.Equal("my chat", s.Name)
	assert.Equal("model-1", s.Model)
	assert.Equal("provider-1", s.Provider)
	assert.Empty(s.Messages)
	assert.False(s.Created.IsZero())
}

// Test CreateSession with provider filter
func Test_session_004(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "shared", OwnedBy: "provider-1"}}}),
		WithClient(&mockClient{name: "provider-2", models: []schema.Model{{Name: "shared", OwnedBy: "provider-2"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	s, err := m.CreateSession(context.TODO(), schema.SessionMeta{
		GeneratorMeta: schema.GeneratorMeta{Provider: "provider-2", Model: "shared"},
	})
	assert.NoError(err)
	assert.Equal("provider-2", s.Provider)
}

// Test CreateSession with unknown model returns not found
func Test_session_005(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	_, err = m.CreateSession(context.TODO(), schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: "nonexistent"}})
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test GetSession retrieves a created session
func Test_session_006(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	created, err := m.CreateSession(context.TODO(), schema.SessionMeta{
		Name:          "test",
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
	})
	assert.NoError(err)

	got, err := m.GetSession(context.TODO(), created.ID)
	assert.NoError(err)
	assert.Equal(created.ID, got.ID)
	assert.Equal("test", got.Name)
	assert.Equal("model-1", got.Model)
}

// Test GetSession with unknown ID returns not found
func Test_session_007(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	_, err = m.GetSession(context.TODO(), "nonexistent")
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test DeleteSession removes a session
func Test_session_008(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	created, err := m.CreateSession(context.TODO(), schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: "model-1"}})
	assert.NoError(err)

	deleted, err := m.DeleteSession(context.TODO(), created.ID)
	assert.NoError(err)
	assert.Equal(created.ID, deleted.ID)

	_, err = m.GetSession(context.TODO(), created.ID)
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test DeleteSession with unknown ID returns not found
func Test_session_009(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	_, err = m.DeleteSession(context.TODO(), "nonexistent")
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test ListSessions returns all sessions in order
func Test_session_010(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	_, err = m.CreateSession(context.TODO(), schema.SessionMeta{Name: "first", GeneratorMeta: schema.GeneratorMeta{Model: "model-1"}})
	assert.NoError(err)
	_, err = m.CreateSession(context.TODO(), schema.SessionMeta{Name: "second", GeneratorMeta: schema.GeneratorMeta{Model: "model-1"}})
	assert.NoError(err)
	_, err = m.CreateSession(context.TODO(), schema.SessionMeta{Name: "third", GeneratorMeta: schema.GeneratorMeta{Model: "model-1"}})
	assert.NoError(err)

	resp, err := m.ListSessions(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Len(resp.Body, 3)
}

// Test ListSessions with limit
func Test_session_011(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	for range 5 {
		_, err = m.CreateSession(context.TODO(), schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: "model-1"}})
		assert.NoError(err)
	}

	resp, err := m.ListSessions(context.TODO(), schema.ListSessionRequest{Limit: types.Ptr(uint(2))})
	assert.NoError(err)
	assert.Equal(uint(5), resp.Count)
	assert.Len(resp.Body, 2)
}

// Test ListSessions returns empty when no sessions exist
func Test_session_012(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	resp, err := m.ListSessions(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Equal(uint(0), resp.Count)
	assert.Empty(resp.Body)
}

// Test ListSessions with offset
func Test_session_013(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	for range 5 {
		_, err = m.CreateSession(context.TODO(), schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: "model-1"}})
		assert.NoError(err)
	}

	resp, err := m.ListSessions(context.TODO(), schema.ListSessionRequest{Offset: 3})
	assert.NoError(err)
	assert.Equal(uint(5), resp.Count)
	assert.Len(resp.Body, 2)
}

// Test ListSessions with offset and limit
func Test_session_014(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	for range 5 {
		_, err = m.CreateSession(context.TODO(), schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: "model-1"}})
		assert.NoError(err)
	}

	resp, err := m.ListSessions(context.TODO(), schema.ListSessionRequest{Offset: 1, Limit: types.Ptr(uint(2))})
	assert.NoError(err)
	assert.Equal(uint(5), resp.Count)
	assert.Equal(uint(1), resp.Offset)
	assert.Equal(uint(2), types.Value(resp.Limit))
	assert.Len(resp.Body, 2)
}

// Test UpdateSession changes name
func Test_session_015(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	s, err := m.CreateSession(context.TODO(), schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: "model-1"}})
	assert.NoError(err)

	updated, err := m.UpdateSession(context.TODO(), s.ID, schema.SessionMeta{Name: "new-name"})
	assert.NoError(err)
	assert.Equal("new-name", updated.Name)
	assert.Equal("model-1", updated.Model)
	assert.Equal("provider-1", updated.Provider)
}

// Test UpdateSession changes system prompt
func Test_session_016(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	s, err := m.CreateSession(context.TODO(), schema.SessionMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1", SystemPrompt: "old prompt"},
	})
	assert.NoError(err)

	updated, err := m.UpdateSession(context.TODO(), s.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{SystemPrompt: "new prompt"}})
	assert.NoError(err)
	assert.Equal("new prompt", updated.SystemPrompt)
	assert.Equal("model-1", updated.Model)
}

// Test UpdateSession changes model with validation
func Test_session_017(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{
			{Name: "model-1", OwnedBy: "provider-1"},
			{Name: "model-2", OwnedBy: "provider-1"},
		}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	s, err := m.CreateSession(context.TODO(), schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: "model-1"}})
	assert.NoError(err)

	updated, err := m.UpdateSession(context.TODO(), s.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: "model-2"}})
	assert.NoError(err)
	assert.Equal("model-2", updated.Model)
	assert.Equal("provider-1", updated.Provider)
}

// Test UpdateSession rejects invalid model
func Test_session_018(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	s, err := m.CreateSession(context.TODO(), schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: "model-1"}})
	assert.NoError(err)

	_, err = m.UpdateSession(context.TODO(), s.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: "nonexistent"}})
	assert.Error(err)
}

// Test UpdateSession with nonexistent session returns error
func Test_session_019(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	_, err = m.UpdateSession(context.TODO(), "nonexistent-id", schema.SessionMeta{Name: "test"})
	assert.Error(err)
}

// Test CreateSession with labels
func Test_session_020(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	s, err := m.CreateSession(context.TODO(), schema.SessionMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Labels:        map[string]string{"chat-id": "12345", "ui": "telegram"},
	})
	assert.NoError(err)
	assert.Equal("12345", s.Labels["chat-id"])
	assert.Equal("telegram", s.Labels["ui"])
}

// Test CreateSession with invalid label key returns error
func Test_session_021(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	_, err = m.CreateSession(context.TODO(), schema.SessionMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Labels:        map[string]string{"bad key!": "value"},
	})
	assert.Error(err)
}

// Test ListSessions filters by labels
func Test_session_022(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	_, err = m.CreateSession(context.TODO(), schema.SessionMeta{
		Name:          "telegram-chat",
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Labels:        map[string]string{"ui": "telegram", "chat-id": "100"},
	})
	assert.NoError(err)
	_, err = m.CreateSession(context.TODO(), schema.SessionMeta{
		Name:          "web-chat",
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Labels:        map[string]string{"ui": "web"},
	})
	assert.NoError(err)
	_, err = m.CreateSession(context.TODO(), schema.SessionMeta{
		Name:          "no-labels",
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
	})
	assert.NoError(err)

	// Filter by ui:telegram
	resp, err := m.ListSessions(context.TODO(), schema.ListSessionRequest{Label: []string{"ui:telegram"}})
	assert.NoError(err)
	assert.Len(resp.Body, 1)
	assert.Equal("telegram-chat", resp.Body[0].Name)

	// Filter by ui:web
	resp, err = m.ListSessions(context.TODO(), schema.ListSessionRequest{Label: []string{"ui:web"}})
	assert.NoError(err)
	assert.Len(resp.Body, 1)
	assert.Equal("web-chat", resp.Body[0].Name)

	// No filter returns all
	resp, err = m.ListSessions(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Len(resp.Body, 3)
}

// Test UpdateSession merges and removes labels
func Test_session_023(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemorySessionStore()),
	)
	assert.NoError(err)

	s, err := m.CreateSession(context.TODO(), schema.SessionMeta{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Labels:        map[string]string{"env": "prod", "team": "backend"},
	})
	assert.NoError(err)

	// Merge: add new, change existing
	updated, err := m.UpdateSession(context.TODO(), s.ID, schema.SessionMeta{
		Labels: map[string]string{"team": "frontend", "region": "us"},
	})
	assert.NoError(err)
	assert.Equal("prod", updated.Labels["env"])
	assert.Equal("frontend", updated.Labels["team"])
	assert.Equal("us", updated.Labels["region"])

	// Remove label
	updated, err = m.UpdateSession(context.TODO(), s.ID, schema.SessionMeta{
		Labels: map[string]string{"team": ""},
	})
	assert.NoError(err)
	_, exists := updated.Labels["team"]
	assert.False(exists)
	assert.Equal("prod", updated.Labels["env"])
	assert.Equal("us", updated.Labels["region"])
}
