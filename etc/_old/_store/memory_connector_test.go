package store_test

import (
	"testing"

	// Packages
	store "github.com/mutablelogic/go-llm/pkg/_store"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	assert "github.com/stretchr/testify/assert"
)

func Test_memory_connector_001(t *testing.T) {
	assert := assert.New(t)
	s := store.NewMemoryConnectorStore()
	assert.NotNil(s)
}

func Test_memory_connector_002(t *testing.T) {
	runConnectorStoreTests(t, func() schema.ConnectorStore {
		return store.NewMemoryConnectorStore()
	})
}
