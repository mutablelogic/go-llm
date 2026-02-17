package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	store "github.com/mutablelogic/go-llm/pkg/store"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// FILE SESSION STORE LIFECYCLE TESTS

// Test NewFileSessionStore creates directory
func Test_file_session_001(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	store, err := store.NewFileSessionStore(filepath.Join(dir, "sessions"))
	assert.NoError(err)
	assert.NotNil(store)
	_, err = os.Stat(filepath.Join(dir, "sessions"))
	assert.NoError(err)
}

// Test NewFileSessionStore with empty dir returns error
func Test_file_session_002(t *testing.T) {
	assert := assert.New(t)
	_, err := store.NewFileSessionStore("")
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// SHARED SESSION STORE TESTS

func Test_file_session_003(t *testing.T) {
	runSessionStoreTests(t, func() schema.SessionStore {
		s, _ := store.NewFileSessionStore(t.TempDir())
		return s
	})
}

///////////////////////////////////////////////////////////////////////////////
// FILE-SPECIFIC TESTS

// Test Create writes a JSON file to disk
func Test_file_session_004(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	s, _ := store.NewFileSessionStore(dir)
	session, err := s.CreateSession(context.TODO(), schema.SessionMeta{
		Name:          "my chat",
		GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
	})
	assert.NoError(err)
	_, err = os.Stat(filepath.Join(dir, session.ID+".json"))
	assert.NoError(err)
}

// Test Delete removes the file from disk
func Test_file_session_005(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	s, _ := store.NewFileSessionStore(dir)
	session, _ := s.CreateSession(context.TODO(), testMeta)
	err := s.DeleteSession(context.TODO(), session.ID)
	assert.NoError(err)
	_, err = os.Stat(filepath.Join(dir, session.ID+".json"))
	assert.True(os.IsNotExist(err))
}

// Test List skips non-JSON files and subdirectories
func Test_file_session_006(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	s, _ := store.NewFileSessionStore(dir)
	s.CreateSession(context.TODO(), schema.SessionMeta{
		Name:          "real",
		GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
	})
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o600)
	os.Mkdir(filepath.Join(dir, "subdir"), 0o700)
	resp, err := s.ListSessions(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Len(resp.Body, 1)
}

// Test List skips corrupt JSON files
func Test_file_session_007(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	s, _ := store.NewFileSessionStore(dir)
	s.CreateSession(context.TODO(), schema.SessionMeta{
		Name:          "good",
		GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
	})
	os.WriteFile(filepath.Join(dir, "bad-id.json"), []byte("{corrupt"), 0o600)
	resp, err := s.ListSessions(context.TODO(), schema.ListSessionRequest{})
	assert.NoError(err)
	assert.Len(resp.Body, 1)
	assert.Equal("good", resp.Body[0].Name)
}
