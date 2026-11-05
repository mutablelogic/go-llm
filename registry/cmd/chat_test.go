package cmd

import (
	"testing"

	// Packages
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	assert "github.com/stretchr/testify/assert"
)

func Test_ModelToolNames(t *testing.T) {
	// Tools are translated from the names models call them by, apart from a
	// name which more than one tool has
	names := modelToolNames([]*toolschema.Tool{
		{Canonical: "homeassistant:call_service"},
		{Canonical: "my_tools:search"},
		{Canonical: "my:tools_search"},
		{Canonical: "nonamespace"},
	})
	assert.Equal(t, map[string]string{"homeassistant_call_service": "homeassistant:call_service"}, names)
}
