package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	store "github.com/mutablelogic/go-llm/pkg/store"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// FILE AGENT STORE LIFECYCLE TESTS

// Test NewFileAgentStore creates directory
func Test_file_agent_001(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	s, err := store.NewFileAgentStore(filepath.Join(dir, "agents"))
	assert.NoError(err)
	assert.NotNil(s)
	_, err = os.Stat(filepath.Join(dir, "agents"))
	assert.NoError(err)
}

// Test NewFileAgentStore with empty dir returns error
func Test_file_agent_002(t *testing.T) {
	assert := assert.New(t)
	_, err := store.NewFileAgentStore("")
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrBadParameter)
}

///////////////////////////////////////////////////////////////////////////////
// SHARED AGENT STORE TESTS

func Test_file_agent_003(t *testing.T) {
	runAgentStoreTests(t, func() schema.AgentStore {
		s, _ := store.NewFileAgentStore(t.TempDir())
		return s
	})
}

///////////////////////////////////////////////////////////////////////////////
// FILE-SPECIFIC TESTS

// Test CreateAgent writes a JSON file to disk
func Test_file_agent_004(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	s, _ := store.NewFileAgentStore(dir)

	a, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	_, err = os.Stat(filepath.Join(dir, a.ID+".json"))
	assert.NoError(err)
}

// Test duplicate name detection across separate store instances
// sharing the same directory (persistence check)
func Test_file_agent_005(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()

	s1, _ := store.NewFileAgentStore(dir)
	_, err := s1.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	s2, _ := store.NewFileAgentStore(dir)
	_, err = s2.CreateAgent(context.TODO(), testAgentMeta)
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrConflict)
}

// Test GetAgent persists across store instances
func Test_file_agent_006(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()

	s1, _ := store.NewFileAgentStore(dir)
	created, err := s1.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	// New store instance, same directory
	s2, _ := store.NewFileAgentStore(dir)
	got, err := s2.GetAgent(context.TODO(), created.ID)
	assert.NoError(err)
	assert.Equal(created.ID, got.ID)
	assert.Equal(created.Name, got.Name)

	// Also by name
	got2, err := s2.GetAgent(context.TODO(), created.Name)
	assert.NoError(err)
	assert.Equal(created.ID, got2.ID)
}

// Test ListAgents skips non-JSON files and corrupt files
func Test_file_agent_007(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	s, _ := store.NewFileAgentStore(dir)

	_, err := s.CreateAgent(context.TODO(), testAgentMeta)
	assert.NoError(err)

	// Write non-JSON and corrupt files
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not json"), 0644)
	os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{bad"), 0644)

	resp, err := s.ListAgents(context.TODO(), schema.ListAgentRequest{})
	assert.NoError(err)
	assert.Equal(uint(1), resp.Count)
	assert.Len(resp.Body, 1)
}

// Test ListAgents persists across store instances
func Test_file_agent_008(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()

	s1, _ := store.NewFileAgentStore(dir)
	for _, name := range []string{"alpha", "beta"} {
		_, err := s1.CreateAgent(context.TODO(), schema.AgentMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
			Name:          name,
		})
		assert.NoError(err)
	}

	// New store instance, same directory
	s2, _ := store.NewFileAgentStore(dir)
	resp, err := s2.ListAgents(context.TODO(), schema.ListAgentRequest{})
	assert.NoError(err)
	assert.Equal(uint(2), resp.Count)
	assert.Len(resp.Body, 2)
}
