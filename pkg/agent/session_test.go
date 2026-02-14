package agent

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	session "github.com/mutablelogic/go-llm/pkg/session"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// SESSION TESTS

// Test operations fail without a session store
func Test_session_001(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
	)
	assert.NoError(err)

	_, err = m.CreateSession(context.TODO(), schema.CreateSessionRequest{Model: "m1"})
	assert.ErrorIs(err, llm.ErrNotImplemented)

	_, err = m.GetSession(context.TODO(), schema.GetSessionRequest{ID: "abc"})
	assert.ErrorIs(err, llm.ErrNotImplemented)

	err = m.DeleteSession(context.TODO(), schema.DeleteSessionRequest{ID: "abc"})
	assert.ErrorIs(err, llm.ErrNotImplemented)

	_, err = m.ListSessions(context.TODO(), schema.ListSessionsRequest{})
	assert.ErrorIs(err, llm.ErrNotImplemented)
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
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	s, err := m.CreateSession(context.TODO(), schema.CreateSessionRequest{
		Name:  "my chat",
		Model: "m1",
	})
	assert.NoError(err)
	assert.NotNil(s)
	assert.NotEmpty(s.ID)
	assert.Equal("my chat", s.Name)
	assert.Equal("m1", s.Model.Name)
	assert.Equal("p1", s.Model.OwnedBy)
	assert.Empty(s.Messages)
	assert.False(s.Created.IsZero())
}

// Test CreateSession with provider filter
func Test_session_004(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "shared", OwnedBy: "p1"}}}),
		WithClient(&mockClient{name: "p2", models: []schema.Model{{Name: "shared", OwnedBy: "p2"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	s, err := m.CreateSession(context.TODO(), schema.CreateSessionRequest{
		Provider: "p2",
		Model:    "shared",
	})
	assert.NoError(err)
	assert.Equal("p2", s.Model.OwnedBy)
}

// Test CreateSession with unknown model returns not found
func Test_session_005(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	_, err = m.CreateSession(context.TODO(), schema.CreateSessionRequest{Model: "nonexistent"})
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test GetSession retrieves a created session
func Test_session_006(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	created, err := m.CreateSession(context.TODO(), schema.CreateSessionRequest{
		Name:  "test",
		Model: "m1",
	})
	assert.NoError(err)

	got, err := m.GetSession(context.TODO(), schema.GetSessionRequest{ID: created.ID})
	assert.NoError(err)
	assert.Equal(created.ID, got.ID)
	assert.Equal("test", got.Name)
	assert.Equal("m1", got.Model.Name)
}

// Test GetSession with unknown ID returns not found
func Test_session_007(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
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
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	created, err := m.CreateSession(context.TODO(), schema.CreateSessionRequest{Model: "m1"})
	assert.NoError(err)

	err = m.DeleteSession(context.TODO(), schema.DeleteSessionRequest{ID: created.ID})
	assert.NoError(err)

	_, err = m.GetSession(context.TODO(), schema.GetSessionRequest{ID: created.ID})
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test DeleteSession with unknown ID returns not found
func Test_session_009(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	err = m.DeleteSession(context.TODO(), schema.DeleteSessionRequest{ID: "nonexistent"})
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test ListSessions returns all sessions in order
func Test_session_010(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	_, err = m.CreateSession(context.TODO(), schema.CreateSessionRequest{Name: "first", Model: "m1"})
	assert.NoError(err)
	_, err = m.CreateSession(context.TODO(), schema.CreateSessionRequest{Name: "second", Model: "m1"})
	assert.NoError(err)
	_, err = m.CreateSession(context.TODO(), schema.CreateSessionRequest{Name: "third", Model: "m1"})
	assert.NoError(err)

	resp, err := m.ListSessions(context.TODO(), schema.ListSessionsRequest{})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Len(resp.Body, 3)
}

// Test ListSessions with limit
func Test_session_011(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	for i := 0; i < 5; i++ {
		_, err = m.CreateSession(context.TODO(), schema.CreateSessionRequest{Model: "m1"})
		assert.NoError(err)
	}

	resp, err := m.ListSessions(context.TODO(), schema.ListSessionsRequest{Limit: 2})
	assert.NoError(err)
	assert.Equal(uint(5), resp.Count)
	assert.Len(resp.Body, 2)
}

// Test ListSessions returns empty when no sessions exist
func Test_session_012(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	resp, err := m.ListSessions(context.TODO(), schema.ListSessionsRequest{})
	assert.NoError(err)
	assert.Equal(uint(0), resp.Count)
	assert.Empty(resp.Body)
}

// Test ListSessions with offset
func Test_session_013(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	for i := 0; i < 5; i++ {
		_, err = m.CreateSession(context.TODO(), schema.CreateSessionRequest{Model: "m1"})
		assert.NoError(err)
	}

	resp, err := m.ListSessions(context.TODO(), schema.ListSessionsRequest{Offset: 3})
	assert.NoError(err)
	assert.Equal(uint(5), resp.Count)
	assert.Len(resp.Body, 2)
}

// Test ListSessions with offset and limit
func Test_session_014(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
		WithSessionStore(session.NewMemoryStore()),
	)
	assert.NoError(err)

	for i := 0; i < 5; i++ {
		_, err = m.CreateSession(context.TODO(), schema.CreateSessionRequest{Model: "m1"})
		assert.NoError(err)
	}

	resp, err := m.ListSessions(context.TODO(), schema.ListSessionsRequest{Offset: 1, Limit: 2})
	assert.NoError(err)
	assert.Equal(uint(5), resp.Count)
	assert.Equal(uint(1), resp.Offset)
	assert.Equal(uint(2), resp.Limit)
	assert.Len(resp.Body, 2)
}
