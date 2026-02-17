package store_test

import (
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	store "github.com/mutablelogic/go-llm/pkg/store"
	assert "github.com/stretchr/testify/assert"
)

func Test_memory_agent_001(t *testing.T) {
	// NewMemoryAgentStore returns a non-nil store
	assert := assert.New(t)
	s := store.NewMemoryAgentStore()
	assert.NotNil(s)
}

func Test_memory_agent_002(t *testing.T) {
	runAgentStoreTests(t, func() schema.AgentStore {
		return store.NewMemoryAgentStore()
	})
}
