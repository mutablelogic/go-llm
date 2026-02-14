package session_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	session "github.com/mutablelogic/go-llm/pkg/session"
	assert "github.com/stretchr/testify/assert"
)

var testModel = schema.Model{Name: "test-model", OwnedBy: "test-provider"}

func textPtr(s string) *string { return &s }

func Test_memory_001(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	assert.NotNil(store)
}

func Test_memory_002(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, err := store.Create(context.TODO(), "my chat", testModel)
	assert.NoError(err)
	assert.NotNil(s)
	assert.NotEmpty(s.ID)
	assert.Equal("my chat", s.Name)
	assert.Equal("test-model", s.Model.Name)
	assert.Equal("test-provider", s.Model.OwnedBy)
	assert.Empty(s.Messages)
	assert.False(s.Created.IsZero())
	assert.False(s.Modified.IsZero())
}

func Test_memory_003(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	_, err := store.Create(context.TODO(), "test", schema.Model{})
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrBadParameter)
}

func Test_memory_004(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s1, err := store.Create(context.TODO(), "first", testModel)
	assert.NoError(err)
	s2, err := store.Create(context.TODO(), "second", testModel)
	assert.NoError(err)
	assert.NotEqual(s1.ID, s2.ID)
}

func Test_memory_005(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, err := store.Create(context.TODO(), "", testModel)
	assert.NoError(err)
	assert.Equal("", s.Name)
}

func Test_memory_006(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	created, _ := store.Create(context.TODO(), "test", testModel)
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
	assert.ErrorIs(err, llm.ErrNotFound)
}

func Test_memory_008(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), "test", testModel)
	err := store.Delete(context.TODO(), s.ID)
	assert.NoError(err)
	_, err = store.Get(context.TODO(), s.ID)
	assert.ErrorIs(err, llm.ErrNotFound)
}

func Test_memory_009(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	err := store.Delete(context.TODO(), "nonexistent")
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrNotFound)
}

func Test_memory_010(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	sessions, err := store.List(context.TODO())
	assert.NoError(err)
	assert.Empty(sessions)
}

func Test_memory_011(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	store.Create(context.TODO(), "first", testModel)
	store.Create(context.TODO(), "second", testModel)
	store.Create(context.TODO(), "third", testModel)
	sessions, err := store.List(context.TODO())
	assert.NoError(err)
	assert.Len(sessions, 3)
}

func Test_memory_012(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s1, _ := store.Create(context.TODO(), "oldest", testModel)
	time.Sleep(10 * time.Millisecond)
	s2, _ := store.Create(context.TODO(), "middle", testModel)
	time.Sleep(10 * time.Millisecond)
	s3, _ := store.Create(context.TODO(), "newest", testModel)
	sessions, err := store.List(context.TODO())
	assert.NoError(err)
	assert.Len(sessions, 3)
	assert.Equal(s3.ID, sessions[0].ID)
	assert.Equal(s2.ID, sessions[1].ID)
	assert.Equal(s1.ID, sessions[2].ID)
}

func Test_memory_013(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s1, _ := store.Create(context.TODO(), "first", testModel)
	time.Sleep(10 * time.Millisecond)
	store.Create(context.TODO(), "second", testModel)
	time.Sleep(10 * time.Millisecond)
	msg := schema.Message{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: textPtr("hello")}}}
	s1.Append(msg)
	sessions, err := store.List(context.TODO())
	assert.NoError(err)
	assert.Equal(s1.ID, sessions[0].ID)
}

func Test_memory_014(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), "doomed", testModel)
	store.Create(context.TODO(), "keeper", testModel)
	store.Delete(context.TODO(), s.ID)
	sessions, err := store.List(context.TODO())
	assert.NoError(err)
	assert.Len(sessions, 1)
	assert.Equal("keeper", sessions[0].Name)
}

// Test WithLimit on MemoryStore
func Test_memory_015(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	store.Create(context.TODO(), "a", testModel)
	store.Create(context.TODO(), "b", testModel)
	store.Create(context.TODO(), "c", testModel)

	// Limit to 2
	sessions, err := store.List(context.TODO(), session.WithLimit(2))
	assert.NoError(err)
	assert.Len(sessions, 2)

	// Limit larger than total returns all
	sessions, err = store.List(context.TODO(), session.WithLimit(100))
	assert.NoError(err)
	assert.Len(sessions, 3)

	// No limit returns all
	sessions, err = store.List(context.TODO())
	assert.NoError(err)
	assert.Len(sessions, 3)
}

func Test_session_001(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), "test", testModel)
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
	s, _ := store.Create(context.TODO(), "test", testModel)
	s.Append(schema.Message{Role: schema.RoleUser, Tokens: 10})
	s.Append(schema.Message{Role: schema.RoleAssistant, Tokens: 25})
	assert.Equal(uint(35), s.Tokens())
}

func Test_session_003(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), "test", testModel)
	ms := s.Conversation()
	assert.NotNil(ms)
	assert.Len(*ms, 0)
	s.Append(schema.Message{Role: schema.RoleUser})
	assert.Len(*ms, 1)
}

func Test_session_004(t *testing.T) {
	assert := assert.New(t)
	s := &session.Session{Model: testModel}
	err := s.Validate()
	assert.Error(err)
}

func Test_session_005(t *testing.T) {
	assert := assert.New(t)
	s := &session.Session{ID: "abc"}
	err := s.Validate()
	assert.Error(err)
}

func Test_session_006(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), "test", testModel)
	assert.NoError(s.Validate())
}

func Test_session_007(t *testing.T) {
	assert := assert.New(t)
	store := session.NewMemoryStore()
	s, _ := store.Create(context.TODO(), "test", testModel)
	assert.NotEmpty(s.String())
}

///////////////////////////////////////////////////////////////////////////////
// FILE STORE TESTS

// Test NewFileStore creates directory
func Test_file_001(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, err := session.NewFileStore(filepath.Join(dir, "sessions"))
	assert.NoError(err)
	assert.NotNil(store)
	_, err = os.Stat(filepath.Join(dir, "sessions"))
	assert.NoError(err)
}

// Test NewFileStore with empty dir returns error
func Test_file_002(t *testing.T) {
	assert := assert.New(t)
	_, err := session.NewFileStore("")
	assert.Error(err)
}

// Test Create writes a JSON file
func Test_file_003(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	s, err := store.Create(context.TODO(), "my chat", testModel)
	assert.NoError(err)
	assert.NotEmpty(s.ID)
	assert.Equal("my chat", s.Name)
	// File should exist
	_, err = os.Stat(filepath.Join(dir, s.ID+".json"))
	assert.NoError(err)
}

// Test Create with empty model returns error
func Test_file_004(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	_, err := store.Create(context.TODO(), "test", schema.Model{})
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrBadParameter)
}

// Test Get reads back a created session
func Test_file_005(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	created, _ := store.Create(context.TODO(), "test", testModel)
	got, err := store.Get(context.TODO(), created.ID)
	assert.NoError(err)
	assert.Equal(created.ID, got.ID)
	assert.Equal(created.Name, got.Name)
	assert.Equal(created.Model.Name, got.Model.Name)
}

// Test Get returns not found for missing ID
func Test_file_006(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	_, err := store.Get(context.TODO(), "nonexistent")
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test Delete removes the file
func Test_file_007(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	s, _ := store.Create(context.TODO(), "test", testModel)
	err := store.Delete(context.TODO(), s.ID)
	assert.NoError(err)
	// File should be gone
	_, err = os.Stat(filepath.Join(dir, s.ID+".json"))
	assert.True(os.IsNotExist(err))
	// Get should fail
	_, err = store.Get(context.TODO(), s.ID)
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test Delete returns not found for missing ID
func Test_file_008(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	err := store.Delete(context.TODO(), "nonexistent")
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test List returns empty for empty directory
func Test_file_009(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	sessions, err := store.List(context.TODO())
	assert.NoError(err)
	assert.Empty(sessions)
}

// Test List returns all sessions
func Test_file_010(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	store.Create(context.TODO(), "first", testModel)
	store.Create(context.TODO(), "second", testModel)
	store.Create(context.TODO(), "third", testModel)
	sessions, err := store.List(context.TODO())
	assert.NoError(err)
	assert.Len(sessions, 3)
}

// Test List orders by modified time (most recent first)
func Test_file_011(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	s1, _ := store.Create(context.TODO(), "oldest", testModel)
	time.Sleep(10 * time.Millisecond)
	s2, _ := store.Create(context.TODO(), "middle", testModel)
	time.Sleep(10 * time.Millisecond)
	s3, _ := store.Create(context.TODO(), "newest", testModel)
	sessions, err := store.List(context.TODO())
	assert.NoError(err)
	assert.Len(sessions, 3)
	assert.Equal(s3.ID, sessions[0].ID)
	assert.Equal(s2.ID, sessions[1].ID)
	assert.Equal(s1.ID, sessions[2].ID)
}

// Test List skips non-JSON files and subdirectories
func Test_file_012(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	store.Create(context.TODO(), "real", testModel)
	// Create junk files
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o600)
	os.Mkdir(filepath.Join(dir, "subdir"), 0o700)
	sessions, err := store.List(context.TODO())
	assert.NoError(err)
	assert.Len(sessions, 1)
}

// Test List skips corrupt JSON files
func Test_file_013(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	store.Create(context.TODO(), "good", testModel)
	os.WriteFile(filepath.Join(dir, "bad-id.json"), []byte("{corrupt"), 0o600)
	sessions, err := store.List(context.TODO())
	assert.NoError(err)
	assert.Len(sessions, 1)
	assert.Equal("good", sessions[0].Name)
}

// Test Write persists mutations to disk
func Test_file_014(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	s, _ := store.Create(context.TODO(), "test", testModel)
	s.Append(schema.Message{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: textPtr("hello")}}})
	err := store.Write(s)
	assert.NoError(err)
	// Read back and verify the message was persisted
	got, err := store.Get(context.TODO(), s.ID)
	assert.NoError(err)
	assert.Len(got.Messages, 1)
	assert.Equal("hello", got.Messages[0].Text())
}

// Test round-trip: create, append, write, read back preserves all fields
func Test_file_015(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	s, _ := store.Create(context.TODO(), "round-trip", testModel)
	s.Append(schema.Message{Role: schema.RoleUser, Tokens: 5, Content: []schema.ContentBlock{{Text: textPtr("hi")}}})
	s.Append(schema.Message{Role: schema.RoleAssistant, Tokens: 10, Content: []schema.ContentBlock{{Text: textPtr("hello")}}})
	store.Write(s)
	got, err := store.Get(context.TODO(), s.ID)
	assert.NoError(err)
	assert.Equal(s.ID, got.ID)
	assert.Equal("round-trip", got.Name)
	assert.Equal("test-model", got.Model.Name)
	assert.Len(got.Messages, 2)
	assert.Equal(uint(15), got.Tokens())
}

// Test unique IDs across creates
func Test_file_016(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	s1, _ := store.Create(context.TODO(), "a", testModel)
	s2, _ := store.Create(context.TODO(), "b", testModel)
	assert.NotEqual(s1.ID, s2.ID)
}

// Test WithLimit on FileStore
func Test_file_017(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	store.Create(context.TODO(), "a", testModel)
	store.Create(context.TODO(), "b", testModel)
	store.Create(context.TODO(), "c", testModel)

	// Limit to 1
	sessions, err := store.List(context.TODO(), session.WithLimit(1))
	assert.NoError(err)
	assert.Len(sessions, 1)

	// Limit larger than total returns all
	sessions, err = store.List(context.TODO(), session.WithLimit(100))
	assert.NoError(err)
	assert.Len(sessions, 3)

	// No limit returns all
	sessions, err = store.List(context.TODO())
	assert.NoError(err)
	assert.Len(sessions, 3)
}
