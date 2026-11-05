package homeassistant_test

import (
	"testing"

	// Packages
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	homeassistant "github.com/mutablelogic/go-llm/toolkit/homeassistant"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestNamespace(t *testing.T) {
	// The namespace has its name, no system prompt, and its tools, which can
	// be added to a toolkit
	ns, err := homeassistant.New("http://localhost:8123", "test-token")
	require.NoError(t, err)
	assert.Equal(t, homeassistant.Name, ns.Name())
	assert.Equal(t, "homeassistant", ns.Name())
	assert.Empty(t, ns.SystemPrompt())
	var names []string
	for _, tool := range ns.Tools() {
		names = append(names, tool.Name())
	}
	assert.ElementsMatch(t, []string{"query", "control"}, names)

	tk, err := toolkit.New(t.Context())
	require.NoError(t, err)
	require.NoError(t, tk.Add(ns))
	prompts, err := tk.SystemPrompts(t.Context(), "homeassistant:*")
	require.NoError(t, err)
	assert.Empty(t, prompts)

	// Without its credential, there's no namespace
	ns, err = homeassistant.New("http://localhost:8123", "")
	assert.Error(t, err)
	assert.Nil(t, ns)
}
