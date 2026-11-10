package deepseek_test

import (
	"testing"

	// Packages
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	deepseek "github.com/mutablelogic/go-llm/toolkit/deepseek"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestNamespace(t *testing.T) {
	// The namespace has its name, no system prompt, and its tools, which can
	// be added to a toolkit
	ns, err := deepseek.New("test-key")
	require.NoError(t, err)
	assert.Equal(t, deepseek.Name, ns.Name())
	assert.Equal(t, "deepseek", ns.Name())
	assert.Empty(t, ns.SystemPrompt())
	var names []string
	for _, tool := range ns.Tools() {
		names = append(names, tool.Name())
	}
	assert.ElementsMatch(t, []string{"search", "fetch"}, names)

	tk, err := toolkit.New(t.Context())
	require.NoError(t, err)
	require.NoError(t, tk.Add(ns))
	prompts, err := tk.SystemPrompts(t.Context(), "deepseek:*")
	require.NoError(t, err)
	assert.Empty(t, prompts)

	// Without its credential, there's no namespace
	ns, err = deepseek.New("")
	assert.Error(t, err)
	assert.Nil(t, ns)
}
