package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	// Packages
	store "github.com/mutablelogic/go-llm/pkg/_store"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// FILE CONNECTOR STORE LIFECYCLE TESTS

// Test NewFileConnectorStore creates the directory
func Test_file_connector_001(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	s, err := store.NewFileConnectorStore(filepath.Join(dir, "connectors"))
	assert.NoError(err)
	assert.NotNil(s)
	_, err = os.Stat(filepath.Join(dir, "connectors"))
	assert.NoError(err)
}

// Test NewFileConnectorStore with empty dir returns error
func Test_file_connector_002(t *testing.T) {
	assert := assert.New(t)
	_, err := store.NewFileConnectorStore("")
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// SHARED CONNECTOR STORE TESTS

func Test_file_connector_003(t *testing.T) {
	runConnectorStoreTests(t, func() schema.ConnectorStore {
		s, err := store.NewFileConnectorStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return s
	})
}

///////////////////////////////////////////////////////////////////////////////
// FILE-SPECIFIC TESTS

// Test CreateConnector writes a JSON file to disk
func Test_file_connector_004(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	s, err := store.NewFileConnectorStore(dir)
	assert.NoError(err)

	c, err := s.CreateConnector(context.Background(), connectorInsert("https://example.com/sse", schema.ConnectorMeta{}))
	assert.NoError(err)
	assert.NotNil(c)

	// At least one .json file must exist in dir.
	entries, err := os.ReadDir(dir)
	assert.NoError(err)
	jsonCount := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			jsonCount++
		}
	}
	assert.Greater(jsonCount, 0)
}

// Test that data survives a store restart (reload from disk)
func Test_file_connector_005(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()

	s1, err := store.NewFileConnectorStore(dir)
	assert.NoError(err)
	_, err = s1.CreateConnector(context.Background(), connectorInsert("https://example.com/sse", schema.ConnectorMeta{}))
	assert.NoError(err)

	// Open a second store instance pointing at the same directory.
	s2, err := store.NewFileConnectorStore(dir)
	assert.NoError(err)
	c, err := s2.GetConnector(context.Background(), "https://example.com/sse")
	assert.NoError(err)
	assert.Equal("https://example.com/sse", c.URL)
}
