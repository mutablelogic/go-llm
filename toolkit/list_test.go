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
		{Name: "empty", Type: schema.NamespaceBuiltin, Tools: 0},
		{Name: "news", Type: schema.NamespaceBuiltin, Tools: 1},
		{Name: "weather", Type: schema.NamespaceBuiltin, Description: "The weather", Tools: 2},
	}}, list)
}

// titledTool is a tool with a title
type titledTool struct {
	echoTool
}

func (titledTool) Title() string {
	return "Echo its input"
}

func TestListToolsTitle(t *testing.T) {
	// A tool which has a title is listed with it, and one which hasn't,
	// without one
	tk, err := toolkit.New(t.Context())
	require.NoError(t, err)
	require.NoError(t, tk.Add(toolkit.NewNamespace("test", "", titledTool{echoTool{name: "titled"}}, echoTool{name: "plain"})))

	list, err := tk.ListTools(t.Context(), schema.ToolListRequest{})
	require.NoError(t, err)
	titles := map[string]string{}
	for _, tool := range list.Body {
		titles[tool.Name] = tool.Title
	}
	assert.Equal(t, map[string]string{"titled": "Echo its input", "plain": ""}, titles)
}
