package store_test

import (
	"context"
	"testing"
	"time"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	store "github.com/mutablelogic/go-llm/pkg/store"
	assert "github.com/stretchr/testify/assert"
)

func Test_memory_session_001(t *testing.T) {
	assert := assert.New(t)
	store := store.NewMemorySessionStore()
	assert.NotNil(store)
}

func Test_memory_session_002(t *testing.T) {
	runSessionStoreTests(t, func() schema.SessionStore {
		return store.NewMemorySessionStore()
	})
}

func Test_session_001(t *testing.T) {
	assert := assert.New(t)
	store := store.NewMemorySessionStore()
	s, _ := store.CreateSession(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	originalModified := s.Modified
	time.Sleep(5 * time.Millisecond)
	msg := schema.Message{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: textPtr("hello")}}}
	s.Append(msg)
	assert.Len(s.Messages, 1)
	assert.True(s.Modified.After(originalModified))
}

func Test_session_002(t *testing.T) {
	assert := assert.New(t)
	store := store.NewMemorySessionStore()
	s, _ := store.CreateSession(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	s.Append(schema.Message{Role: schema.RoleUser, Tokens: 10})
	s.Append(schema.Message{Role: schema.RoleAssistant, Tokens: 25})
	assert.Equal(uint(35), s.Tokens())
}

func Test_session_003(t *testing.T) {
	assert := assert.New(t)
	store := store.NewMemorySessionStore()
	s, _ := store.CreateSession(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	ms := s.Conversation()
	assert.NotNil(ms)
	assert.Len(*ms, 0)
	s.Append(schema.Message{Role: schema.RoleUser})
	assert.Len(*ms, 1)
}

func Test_session_004(t *testing.T) {
	assert := assert.New(t)
	s := &schema.Session{SessionMeta: schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: "test-model"}}}
	err := s.Validate()
	assert.Error(err)
}

func Test_session_005(t *testing.T) {
	assert := assert.New(t)
	s := &schema.Session{ID: "abc"}
	err := s.Validate()
	assert.Error(err)
}

func Test_session_006(t *testing.T) {
	assert := assert.New(t)
	store := store.NewMemorySessionStore()
	s, _ := store.CreateSession(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	assert.NoError(s.Validate())
}

func Test_session_007(t *testing.T) {
	assert := assert.New(t)
	store := store.NewMemorySessionStore()
	s, _ := store.CreateSession(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	assert.NotEmpty(s.String())
}
