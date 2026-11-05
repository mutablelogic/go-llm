package schema_test

import (
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	assert "github.com/stretchr/testify/assert"
)

func Test_ModelToolName(t *testing.T) {
	// A model calls a tool by its namespace and name, separated by two
	// underscores, as a model's tool names can't have a colon, which is
	// separated where it is, back to the canonical name
	name := schema.ModelToolName("my_tools", "call_service")
	assert.Equal(t, "my_tools__call_service", name)
	assert.Equal(t, "my_tools:call_service", schema.CanonicalFromModelToolName(name))
	for _, name := range []string{"respond", "call_service", "__x", "x__"} {
		assert.Equal(t, name, schema.CanonicalFromModelToolName(name))
	}
}

func Test_IsModelToolPart(t *testing.T) {
	for name, want := range map[string]bool{
		"homeassistant": true, "call_service": true, "a_b_c": true,
		"my__tools": false, "_search": false, "search_": false,
	} {
		assert.Equal(t, want, schema.IsModelToolPart(name), name)
	}
}
