package agent

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	session "github.com/mutablelogic/go-llm/pkg/session"
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
	s, err := m.CreateSession(context.TODO(), schema.SessionMeta{Model: "model-1"})
	assert.NoError(err)
	assert.NotEmpty(s.ID)

	got, err := m.GetSession(context.TODO(), schema.GetSessionRequest{ID: s.ID})
	assert.NoError(err)
	assert.Equal(s.ID, got.ID)

	deleted, err := m.DeleteSession(context.TODO(), schema.DeleteSessionRequest{ID: s.ID})
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
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	s, err := m.CreateSession(context.TODO(), schema.SessionMeta{
		Name:  "my chat",
		Model: "model-1",
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
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	s, err := m.CreateSession(context.TODO(), schema.SessionMeta{
		Provider: "provider-2",
		Model:    "shared",
	})
	assert.NoError(err)
	assert.Equal("provider-2", s.Provider)
}

// Test CreateSession with unknown model returns not found
func Test_session_005(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	_, err = m.CreateSession(context.TODO(), schema.SessionMeta{Model: "nonexistent"})
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test GetSession retrieves a created session
func Test_session_006(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	created, err := m.CreateSession(context.TODO(), schema.SessionMeta{
		Name:  "test",
		Model: "model-1",
	})
	assert.NoError(err)

	got, err := m.GetSession(context.TODO(), schema.GetSessionRequest{ID: created.ID})
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
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	_, err = m.GetSession(context.TODO(), schema.GetSessionRequest{ID: "nonexistent"})
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test DeleteSession removes a session
func Test_session_008(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	created, err := m.CreateSession(context.TODO(), schema.SessionMeta{Model: "model-1"})
	assert.NoError(err)

	deleted, err := m.DeleteSession(context.TODO(), schema.DeleteSessionRequest{ID: created.ID})
	assert.NoError(err)
	assert.Equal(created.ID, deleted.ID)

	_, err = m.GetSession(context.TODO(), schema.GetSessionRequest{ID: created.ID})
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test DeleteSession with unknown ID returns not found
func Test_session_009(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	_, err = m.DeleteSession(context.TODO(), schema.DeleteSessionRequest{ID: "nonexistent"})
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test ListSessions returns all sessions in order
func Test_session_010(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	_, err = m.CreateSession(context.TODO(), schema.SessionMeta{Name: "first", Model: "model-1"})
	assert.NoError(err)
	_, err = m.CreateSession(context.TODO(), schema.SessionMeta{Name: "second", Model: "model-1"})
	assert.NoError(err)
	_, err = m.CreateSession(context.TODO(), schema.SessionMeta{Name: "third", Model: "model-1"})
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
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	for i := 0; i < 5; i++ {
		_, err = m.CreateSession(context.TODO(), schema.SessionMeta{Model: "model-1"})
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
		WithSessionStore(session.NewMemoryStore()),
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
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	for i := 0; i < 5; i++ {
		_, err = m.CreateSession(context.TODO(), schema.SessionMeta{Model: "model-1"})
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
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	for i := 0; i < 5; i++ {
		_, err = m.CreateSession(context.TODO(), schema.SessionMeta{Model: "model-1"})
		assert.NoError(err)
	}

	resp, err := m.ListSessions(context.TODO(), schema.ListSessionRequest{Offset: 1, Limit: types.Ptr(uint(2))})
	assert.NoError(err)
	assert.Equal(uint(5), resp.Count)
	assert.Equal(uint(1), resp.Offset)
	assert.Equal(uint(2), types.Value(resp.Limit))
	assert.Len(resp.Body, 2)
}
