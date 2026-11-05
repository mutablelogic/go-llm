package toolkit_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestGetTool(t *testing.T) {
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(toolkit.NewNamespace("test", "", schemaTool{echoTool: echoTool{name: "weather"}}, echoTool{name: "echo"})))

	t.Run("Get", func(t *testing.T) {
		tool, err := tk.GetTool(ctx, "test:weather")
		require.NoError(t, err)
		assert.Equal(t, "weather", tool.Name)
		assert.Equal(t, "test:weather", tool.Canonical)
		assert.Equal(t, "Returns its input", tool.Description)
		var input map[string]any
		require.NoError(t, json.Unmarshal(tool.Input, &input))
		assert.Contains(t, input["properties"], "city")
		assert.Empty(t, tool.Output)
	})

	t.Run("NoSchema", func(t *testing.T) {
		tool, err := tk.GetTool(ctx, "test:echo")
		require.NoError(t, err)
		assert.Equal(t, "test:echo", tool.Canonical)
		assert.Empty(t, tool.Input)
	})

	t.Run("Name", func(t *testing.T) {
		// A name without a namespace refers to the only tool with the name
		tool, err := tk.GetTool(ctx, "weather")
		require.NoError(t, err)
		assert.Equal(t, "test:weather", tool.Canonical)
	})

	t.Run("Ambiguous", func(t *testing.T) {
		// A name in more than one namespace is ambiguous, but the canonical
		// name of each tool isn't
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		require.NoError(t, tk.Add(toolkit.NewNamespace("b", "", echoTool{name: "search"})))
		require.NoError(t, tk.Add(toolkit.NewNamespace("a", "", echoTool{name: "search"}, echoTool{name: "other"})))

		_, err = tk.GetTool(ctx, "search")
		assert.ErrorIs(t, err, llm.ErrConflict)
		assert.ErrorContains(t, err, "a:search, b:search")

		for _, name := range []string{"a:search", "b:search"} {
			tool, err := tk.GetTool(ctx, name)
			require.NoError(t, err, name)
			assert.Equal(t, name, tool.Canonical)
		}

		// A name in one namespace isn't ambiguous
		tool, err := tk.GetTool(ctx, "other")
		require.NoError(t, err)
		assert.Equal(t, "a:other", tool.Canonical)
	})

	t.Run("NotFound", func(t *testing.T) {
		for _, name := range []string{"test:other", "other", "other:weather", "test:", ":weather", ""} {
			_, err := tk.GetTool(ctx, name)
			assert.ErrorIs(t, err, llm.ErrNotFound, name)
		}
	})
}

func TestTools(t *testing.T) {
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(toolkit.NewNamespace("weather", "", echoTool{name: "current"}, echoTool{name: "search"})))
	require.NoError(t, tk.Add(toolkit.NewNamespace("news", "", echoTool{name: "search"})))

	t.Run("Tools", func(t *testing.T) {
		// The tools have new names, and the tool's other methods are
		// unchanged
		tools, err := tk.Tools(ctx, "weather:current", "news:search")
		require.NoError(t, err)
		require.Len(t, tools, 2)
		assert.Equal(t, "weather__current", tools[0].Name())
		assert.Equal(t, "news__search", tools[1].Name())
		assert.Equal(t, "Returns its input", tools[0].Description())
		canonical, ok := tools[0].(interface{ CanonicalName() string })
		require.True(t, ok)
		assert.Equal(t, "weather:current", canonical.CanonicalName())
		result, err := tools[0].Run(ctx, json.RawMessage(`{"a":1}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"a":1}`, string(result.(json.RawMessage)))
	})

	t.Run("Name", func(t *testing.T) {
		// A name without a namespace refers to the only tool with the name
		tools, err := tk.Tools(ctx, "current")
		require.NoError(t, err)
		require.Len(t, tools, 1)
		assert.Equal(t, "weather__current", tools[0].Name())
	})

	t.Run("Namespace", func(t *testing.T) {
		// All the tools in a namespace, in order of canonical name, with
		// other tools
		tools, err := tk.Tools(ctx, "weather:*", "news:search")
		require.NoError(t, err)
		var names []string
		for _, tool := range tools {
			names = append(names, tool.Name())
		}
		assert.Equal(t, []string{"weather__current", "weather__search", "news__search"}, names)

		// The namespace must exist, and its tools can't also be named
		_, err = tk.Tools(ctx, "other:*")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = tk.Tools(ctx, "weather:*", "weather:current")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("All", func(t *testing.T) {
		// "*" is all the tools, in order of canonical name, which can't also
		// be named
		tools, err := tk.Tools(ctx, "*")
		require.NoError(t, err)
		var names []string
		for _, tool := range tools {
			names = append(names, tool.Name())
		}
		assert.Equal(t, []string{"news__search", "weather__current", "weather__search"}, names)
		_, err = tk.Tools(ctx, "*", "weather:current")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = tk.Tools(ctx, "*", "weather:*")
		assert.ErrorIs(t, err, llm.ErrBadParameter)

		// And it's kept as it is, as a canonical name
		canonical, err := tk.ToolNames(ctx, "*")
		require.NoError(t, err)
		assert.Equal(t, []string{"*"}, canonical)
	})

	t.Run("None", func(t *testing.T) {
		tools, err := tk.Tools(ctx)
		require.NoError(t, err)
		assert.Empty(t, tools)
	})

	t.Run("Errors", func(t *testing.T) {
		_, err := tk.Tools(ctx, "weather:other")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = tk.Tools(ctx, "search")
		assert.ErrorIs(t, err, llm.ErrConflict)
		_, err = tk.Tools(ctx, "current", "weather:current")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("AvailableTools", func(t *testing.T) {
		// A tool, or namespace, which isn't in the toolkit is skipped
		tools, skipped, err := tk.AvailableTools(ctx, "weather:current", "ollama:*", "weather:other", "news:search")
		require.NoError(t, err)
		require.Len(t, tools, 2)
		assert.Equal(t, "weather__current", tools[0].Name())
		assert.Equal(t, "news__search", tools[1].Name())
		assert.Equal(t, []string{"ollama:*", "weather:other"}, skipped)

		// Other errors aren't skipped
		_, _, err = tk.AvailableTools(ctx, "search")
		assert.ErrorIs(t, err, llm.ErrConflict)
		_, _, err = tk.AvailableTools(ctx, "current", "weather:current")
		assert.ErrorIs(t, err, llm.ErrBadParameter)

		// Tools doesn't skip them
		_, err = tk.Tools(ctx, "weather:current", "ollama:*")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})
}

func TestToolsNames(t *testing.T) {
	ctx := context.Background()

	t.Run("Distinct", func(t *testing.T) {
		// Tools whose names have underscores have new names of their own,
		// as the namespace is separated by two underscores, which a
		// namespace, or a tool's name, can't have, or start or end with one
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		require.NoError(t, tk.Add(toolkit.NewNamespace("my_tools", "", echoTool{name: "search"})))
		require.NoError(t, tk.Add(toolkit.NewNamespace("my", "", echoTool{name: "tools_search"})))
		tools, err := tk.Tools(ctx, "my_tools:search", "my:tools_search")
		require.NoError(t, err)
		assert.Equal(t, "my_tools__search", tools[0].Name())
		assert.Equal(t, "my__tools_search", tools[1].Name())
		for _, ns := range []toolkit.Namespace{
			toolkit.NewNamespace("my__tools", "", echoTool{name: "search"}),
			toolkit.NewNamespace("tools_", "", echoTool{name: "search"}),
			toolkit.NewNamespace("other", "", echoTool{name: "tools__search"}),
			toolkit.NewNamespace("other", "", echoTool{name: "_search"}),
		} {
			assert.ErrorIs(t, tk.Add(ns), llm.ErrBadParameter, ns.Name())
		}
	})

	t.Run("TooLong", func(t *testing.T) {
		// The new name is at most 64 characters, which is the namespace, two
		// underscores and the name
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		namespace := "n" + strings.Repeat("a", 39)
		require.NoError(t, tk.Add(toolkit.NewNamespace(namespace, "", echoTool{name: strings.Repeat("b", 23)}, echoTool{name: strings.Repeat("c", 22)})))
		_, err = tk.Tools(ctx, namespace+":"+strings.Repeat("b", 23))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = tk.Tools(ctx, namespace+":"+strings.Repeat("c", 22))
		assert.NoError(t, err)
	})
}

func TestToolsRun(t *testing.T) {
	// A tool from Tools is run as with CallTool, so its input is validated,
	// and its result is JSON
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(toolkit.NewNamespace("test", "", schemaTool{echoTool: echoTool{name: "weather"}},
		resultTool{echoTool: echoTool{name: "text"}, result: "It's sunny"},
		resultTool{echoTool: echoTool{name: "none"}, result: nil})))
	tools, err := tk.Tools(ctx, "test:*")
	require.NoError(t, err)
	byName := make(map[string]llm.Tool, len(tools))
	for _, tool := range tools {
		byName[tool.Name()] = tool
	}

	result, err := byName["test__weather"].Run(ctx, json.RawMessage(`{"city":"London"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"city":"London"}`, string(result.(json.RawMessage)))

	_, err = byName["test__weather"].Run(ctx, json.RawMessage(`{"days":3}`))
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	result, err = byName["test__text"].Run(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, json.RawMessage(`"It's sunny"`), result)

	result, err = byName["test__none"].Run(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}
