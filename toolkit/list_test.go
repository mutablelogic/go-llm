package toolkit_test

import (
	"testing"

	// Packages
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// describedNamespace is a namespace with a description
type describedNamespace struct {
	toolkit.Namespace
}

func (describedNamespace) Description() string {
	return "The weather"
}

func TestListNamespaces(t *testing.T) {
	// The namespaces, in order of name, with their descriptions, when they
	// have them, and how many tools each has
	tk, err := toolkit.New(t.Context())
	require.NoError(t, err)
	require.NoError(t, tk.Add(describedNamespace{toolkit.NewNamespace("weather", "", echoTool{name: "current"}, echoTool{name: "forecast"})}))
	require.NoError(t, tk.Add(toolkit.NewNamespace("news", "", echoTool{name: "search"})))
	require.NoError(t, tk.Add(toolkit.NewNamespace("empty", "")))

	list, err := tk.ListNamespaces(t.Context())
	require.NoError(t, err)
	assert.Equal(t, &schema.NamespaceList{Count: 3, Body: []*schema.Namespace{
		{Name: "empty", Tools: 0},
		{Name: "news", Tools: 1},
		{Name: "weather", Description: "The weather", Tools: 2},
	}}, list)
}
