package schema_test

import (
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	assert "github.com/stretchr/testify/assert"
)

func Test_ModelToolName(t *testing.T) {
	// A model calls a tool by its namespace and name, separated by an
	// underscore, as a model's tool names can't have a colon
	assert.Equal(t, "homeassistant_call_service", schema.ModelToolName("homeassistant", "call_service"))
	assert.Equal(t, "homeassistant:call_service", schema.CanonicalToolName("homeassistant", "call_service"))
}
