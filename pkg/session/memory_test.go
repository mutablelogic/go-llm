package session_test

import (
	"context"
	"testing"
	"time"

	schema "github.com/mutablelogic/go-llm/pkg/schema"
	session "github.com/mutablelogic/go-llm/pkg/session"
	assert "github.com/stretchr/testify/assert"
)

var testMeta = schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}}

func textPtr(s string) *string { return &s }

func Test_memory_001(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	assert.NotNil(store)
}

func Test_memory_002(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, err := store.Create(context.TODO(), schema.SessionMeta{Name: "my chat", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	assert.NoError(err)
	assert.NotNil(s)
	assert.NotEmpty(s.ID)
	assert.Equal("my chat", s.Name)
	assert.Equal("test-model", s.Model)
	assert.Equal("test-provider", s.Provider)
	assert.Empty(s.Messages)
	assert.False(s.Created.IsZero())
	assert.False(s.Modified.IsZero())
}

func Test_memory_003(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	_, err := store.Create(context.TODO(), schema.SessionMeta{Name: "test"})
	assert.Error(err)
}

func Test_memory_004(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s1, err := store.Create(context.TODO(), schema.SessionMeta{Name: "first", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	assert.NoError(err)
	s2, err := store.Create(context.TODO(), schema.SessionMeta{Name: "second", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	assert.NoError(err)
	assert.NotEqual(s1.ID, s2.ID)
}

func Test_memory_005(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, err := store.Create(context.TODO(), schema.SessionMeta{Name: "", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	assert.NoError(err)
	assert.Equal("", s.Name)
}

func Test_memory_006(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	created, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	got, err := store.Get(context.TODO(), created.ID)
	assert.NoError(err)
	assert.Equal(created.ID, got.ID)
	assert.Equal(created.Name, got.Name)
}

func Test_memory_007(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	_, err := store.Get(context.TODO(), "nonexistent")
	assert.Error(err)
}

func Test_memory_008(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	err := store.Delete(context.TODO(), s.ID)
	assert.NoError(err)
	_, err = store.Get(context.TODO(), s.ID)
	assert.Error(err)
}

func Test_memory_009(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	err := store.Delete(context.TODO(), "nonexistent")
	assert.Error(err)
}

func Test_memory_010(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	resp, err := store.List(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Empty(resp.Body)
}

func Test_memory_011(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	store.Create(context.TODO(), schema.SessionMeta{Name: "first", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	store.Create(context.TODO(), schema.SessionMeta{Name: "second", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	store.Create(context.TODO(), schema.SessionMeta{Name: "third", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	resp, err := store.List(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Len(resp.Body, 3)
}

func Test_memory_012(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s1, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "oldest", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	time.Sleep(10 * time.Millisecond)
	s2, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "middle", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	time.Sleep(10 * time.Millisecond)
	s3, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "newest", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	resp, err := store.List(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Len(resp.Body, 3)
	assert.Equal(s3.ID, resp.Body[0].ID)
	assert.Equal(s2.ID, resp.Body[1].ID)
	assert.Equal(s1.ID, resp.Body[2].ID)
}

func Test_memory_013(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s1, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "first", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	time.Sleep(10 * time.Millisecond)
	store.Create(context.TODO(), schema.SessionMeta{Name: "second", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	time.Sleep(10 * time.Millisecond)
	msg := schema.Message{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: textPtr("hello")}}}
	s1.Append(msg)
	resp, err := store.List(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Equal(s1.ID, resp.Body[0].ID)
}

func Test_memory_014(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "doomed", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	store.Create(context.TODO(), schema.SessionMeta{Name: "keeper", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	store.Delete(context.TODO(), s.ID)
	resp, err := store.List(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Len(resp.Body, 1)
	assert.Equal("keeper", resp.Body[0].Name)
}

// Test Update changes name
func Test_memory_015(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "original", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	updated, err := store.Update(context.TODO(), s.ID, schema.SessionMeta{Name: "renamed"})
	assert.NoError(err)
	assert.Equal("renamed", updated.Name)
	assert.Equal("test-model", updated.Model)
	assert.Equal("test-provider", updated.Provider)
}

// Test Update changes model and provider
func Test_memory_016(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "model-a", Provider: "provider-a"}})
	updated, err := store.Update(context.TODO(), s.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: "model-b", Provider: "provider-b"}})
	assert.NoError(err)
	assert.Equal("model-b", updated.Model)
	assert.Equal("provider-b", updated.Provider)
	assert.Equal("test", updated.Name)
}

// Test Update with nonexistent ID returns error
func Test_memory_017(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	_, err := store.Update(context.TODO(), "nonexistent", schema.SessionMeta{Name: "x"})
	assert.Error(err)
}

// Test Update only applies non-zero fields
func Test_memory_018(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "keep", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider", SystemPrompt: "original"}})
	updated, err := store.Update(context.TODO(), s.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{SystemPrompt: "changed"}})
	assert.NoError(err)
	assert.Equal("keep", updated.Name)
	assert.Equal("test-model", updated.Model)
	assert.Equal("changed", updated.SystemPrompt)
}

// Test Update advances Modified timestamp
func Test_memory_019(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	original := s.Modified
	time.Sleep(5 * time.Millisecond)
	updated, err := store.Update(context.TODO(), s.ID, schema.SessionMeta{Name: "new"})
	assert.NoError(err)
	assert.True(updated.Modified.After(original))
}

func Test_session_001(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	originalModified := s.Modified
	time.Sleep(5 * time.Millisecond)
	msg := schema.Message{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: textPtr("hello")}}}
	s.Append(msg)
	assert.Len(s.Messages, 1)
	assert.True(s.Modified.After(originalModified))
}

func Test_session_002(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	s.Append(schema.Message{Role: schema.RoleUser, Tokens: 10})
	s.Append(schema.Message{Role: schema.RoleAssistant, Tokens: 25})
	assert.Equal(uint(35), s.Tokens())
}

func Test_session_003(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
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
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	assert.NoError(s.Validate())
}

func Test_session_007(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	assert.NotEmpty(s.String())
}
