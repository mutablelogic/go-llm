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
	s, err := store.Create(context.TODO(), schema.SessionMeta{Name: "my chat", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
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
	_, err := store.Create(context.TODO(), schema.SessionMeta{Name: "test"})
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrBadParameter)
}

// Test Get reads back a created session
func Test_file_005(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	created, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	got, err := store.Get(context.TODO(), created.ID)
	assert.NoError(err)
	assert.Equal(created.ID, got.ID)
	assert.Equal(created.Name, got.Name)
	assert.Equal(created.Model, got.Model)
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
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
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
	resp, err := store.List(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Empty(resp.Body)
}

// Test List returns all sessions
func Test_file_010(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	store.Create(context.TODO(), schema.SessionMeta{Name: "first", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	store.Create(context.TODO(), schema.SessionMeta{Name: "second", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	store.Create(context.TODO(), schema.SessionMeta{Name: "third", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	resp, err := store.List(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Len(resp.Body, 3)
}

// Test List orders by modified time (most recent first)
func Test_file_011(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
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

// Test List skips non-JSON files and subdirectories
func Test_file_012(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	store.Create(context.TODO(), schema.SessionMeta{Name: "real", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	// Create junk files
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o600)
	os.Mkdir(filepath.Join(dir, "subdir"), 0o700)
	resp, err := store.List(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Len(resp.Body, 1)
}

// Test List skips corrupt JSON files
func Test_file_013(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	store.Create(context.TODO(), schema.SessionMeta{Name: "good", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	os.WriteFile(filepath.Join(dir, "bad-id.json"), []byte("{corrupt"), 0o600)
	resp, err := store.List(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Len(resp.Body, 1)
	assert.Equal("good", resp.Body[0].Name)
}

// Test Write persists mutations to disk
func Test_file_014(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
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
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "round-trip", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	s.Append(schema.Message{Role: schema.RoleUser, Tokens: 5, Content: []schema.ContentBlock{{Text: textPtr("hi")}}})
	s.Append(schema.Message{Role: schema.RoleAssistant, Tokens: 10, Content: []schema.ContentBlock{{Text: textPtr("hello")}}})
	store.Write(s)
	got, err := store.Get(context.TODO(), s.ID)
	assert.NoError(err)
	assert.Equal(s.ID, got.ID)
	assert.Equal("round-trip", got.Name)
	assert.Equal("test-model", got.Model)
	assert.Len(got.Messages, 2)
	assert.Equal(uint(15), got.Tokens())
}

// Test Create with labels
func Test_file_021(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	labels := map[string]string{"env": "prod", "team": "backend"}
	s, err := store.Create(context.TODO(), schema.SessionMeta{
		Name:          "labeled",
		GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		Labels:        labels,
	})
	assert.NoError(err)
	assert.Equal("prod", s.Labels["env"])
	assert.Equal("backend", s.Labels["team"])

	// Verify labels survive round-trip
	got, err := store.Get(context.TODO(), s.ID)
	assert.NoError(err)
	assert.Equal("prod", got.Labels["env"])
	assert.Equal("backend", got.Labels["team"])
}

// Test Create with invalid label key returns error
func Test_file_022(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	_, err := store.Create(context.TODO(), schema.SessionMeta{
		Name:          "bad",
		GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		Labels:        map[string]string{"invalid key!": "value"},
	})
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrBadParameter)
}

// Test List filters by labels
func Test_file_023(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	store.Create(context.TODO(), schema.SessionMeta{
		Name:          "a",
		GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		Labels:        map[string]string{"env": "prod"},
	})
	store.Create(context.TODO(), schema.SessionMeta{
		Name:          "b",
		GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		Labels:        map[string]string{"env": "dev"},
	})
	store.Create(context.TODO(), schema.SessionMeta{
		Name:          "c",
		GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
	})

	// Filter by env:prod
	resp, err := store.List(context.TODO(), schema.ListSessionRequest{Label: []string{"env:prod"}})
	assert.NoError(err)
	assert.Len(resp.Body, 1)
	assert.Equal("a", resp.Body[0].Name)

	// No filter returns all
	resp, err = store.List(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Len(resp.Body, 3)
}

// Test List with multiple label filters (AND logic)
func Test_file_024(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	store.Create(context.TODO(), schema.SessionMeta{
		Name:          "match",
		GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		Labels:        map[string]string{"env": "prod", "team": "backend"},
	})
	store.Create(context.TODO(), schema.SessionMeta{
		Name:          "partial",
		GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		Labels:        map[string]string{"env": "prod"},
	})

	resp, err := store.List(context.TODO(), schema.ListSessionRequest{Label: []string{"env:prod", "team:backend"}})
	assert.NoError(err)
	assert.Len(resp.Body, 1)
	assert.Equal("match", resp.Body[0].Name)
}

// Test Update merges labels
func Test_file_025(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	s, _ := store.Create(context.TODO(), schema.SessionMeta{
		Name:          "test",
		GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		Labels:        map[string]string{"env": "prod", "team": "backend"},
	})

	updated, err := store.Update(context.TODO(), s.ID, schema.SessionMeta{
		Labels: map[string]string{"team": "frontend", "region": "us"},
	})
	assert.NoError(err)
	assert.Equal("prod", updated.Labels["env"])
	assert.Equal("frontend", updated.Labels["team"])
	assert.Equal("us", updated.Labels["region"])

	// Verify persisted
	got, err := store.Get(context.TODO(), s.ID)
	assert.NoError(err)
	assert.Equal("prod", got.Labels["env"])
	assert.Equal("frontend", got.Labels["team"])
	assert.Equal("us", got.Labels["region"])
}

// Test Update removes labels with empty value
func Test_file_026(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	s, _ := store.Create(context.TODO(), schema.SessionMeta{
		Name:          "test",
		GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		Labels:        map[string]string{"env": "prod", "team": "backend"},
	})

	updated, err := store.Update(context.TODO(), s.ID, schema.SessionMeta{
		Labels: map[string]string{"team": ""},
	})
	assert.NoError(err)
	assert.Equal("prod", updated.Labels["env"])
	_, exists := updated.Labels["team"]
	assert.False(exists)

	// Verify persisted
	got, err := store.Get(context.TODO(), s.ID)
	assert.NoError(err)
	_, exists = got.Labels["team"]
	assert.False(exists)
}

// Test Update with invalid label key returns error
func Test_file_027(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	s, _ := store.Create(context.TODO(), schema.SessionMeta{
		Name:          "test",
		GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
	})

	_, err := store.Update(context.TODO(), s.ID, schema.SessionMeta{
		Labels: map[string]string{"bad key!": "value"},
	})
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrBadParameter)
}

// Test unique IDs across creates
func Test_file_016(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	s1, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "a", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	s2, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "b", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	assert.NotEqual(s1.ID, s2.ID)
}

// Test Update changes name and persists to disk
func Test_file_017(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "original", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	updated, err := store.Update(context.TODO(), s.ID, schema.SessionMeta{Name: "renamed"})
	assert.NoError(err)
	assert.Equal("renamed", updated.Name)
	assert.Equal("test-model", updated.Model)

	// Verify persisted by re-reading
	got, err := store.Get(context.TODO(), s.ID)
	assert.NoError(err)
	assert.Equal("renamed", got.Name)
}

// Test Update with nonexistent ID returns error
func Test_file_018(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	_, err := store.Update(context.TODO(), "nonexistent", schema.SessionMeta{Name: "x"})
	assert.Error(err)
}

// Test Update only applies non-zero fields
func Test_file_019(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "keep", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider", SystemPrompt: "original"}})
	updated, err := store.Update(context.TODO(), s.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{SystemPrompt: "changed"}})
	assert.NoError(err)
	assert.Equal("keep", updated.Name)
	assert.Equal("test-model", updated.Model)
	assert.Equal("changed", updated.SystemPrompt)
}

// Test Update advances Modified timestamp
func Test_file_020(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, _ := session.NewFileStore(dir)
	s, _ := store.Create(context.TODO(), schema.SessionMeta{Name: "test", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
	original := s.Modified
	time.Sleep(5 * time.Millisecond)
	updated, err := store.Update(context.TODO(), s.ID, schema.SessionMeta{Name: "new"})
	assert.NoError(err)
	assert.True(updated.Modified.After(original))
}
