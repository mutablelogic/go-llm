package newsapi_test

import (
	"testing"

	// Packages
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	newsapi "github.com/mutablelogic/go-llm/toolkit/newsapi"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestNamespace(t *testing.T) {
	// The namespace has its name, no system prompt, and its tools, which can
	// be added to a toolkit
	ns, err := newsapi.New("test-key")
	require.NoError(t, err)
	assert.Equal(t, newsapi.Name, ns.Name())
	assert.Equal(t, "newsapi", ns.Name())
	assert.Empty(t, ns.SystemPrompt())
	var names []string
	for _, tool := range ns.Tools() {
		names = append(names, tool.Name())
	}
	assert.ElementsMatch(t, []string{"articles", "headlines", "sources"}, names)

	tk, err := toolkit.New(t.Context())
	require.NoError(t, err)
	require.NoError(t, tk.Add(ns))
	prompts, err := tk.SystemPrompts(t.Context(), "newsapi:*")
	require.NoError(t, err)
	assert.Empty(t, prompts)

	// Without its credential, there's no namespace
	ns, err = newsapi.New("")
	assert.Error(t, err)
	assert.Nil(t, ns)
}
