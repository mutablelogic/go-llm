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
	require.NoError(t, tk.Add("test", schemaTool{echoTool: echoTool{name: "weather"}}, echoTool{name: "echo"}))

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
		require.NoError(t, tk.Add("b", echoTool{name: "search"}))
		require.NoError(t, tk.Add("a", echoTool{name: "search"}, echoTool{name: "other"}))

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
	require.NoError(t, tk.Add("weather", echoTool{name: "current"}, echoTool{name: "search"}))
	require.NoError(t, tk.Add("news", echoTool{name: "search"}))

	t.Run("Tools", func(t *testing.T) {
		// The tools have new names, and the tool's other methods are
		// unchanged
		tools, err := tk.Tools(ctx, "weather:current", "news:search")
		require.NoError(t, err)
		require.Len(t, tools, 2)
		assert.Equal(t, "weather_current", tools[0].Name())
		assert.Equal(t, "news_search", tools[1].Name())
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
		assert.Equal(t, "weather_current", tools[0].Name())
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
		assert.Equal(t, []string{"weather_current", "weather_search", "news_search"}, names)

		// The namespace must exist, and its tools can't also be named
		_, err = tk.Tools(ctx, "other:*")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = tk.Tools(ctx, "weather:*", "weather:current")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = tk.Tools(ctx, "*")
		assert.ErrorIs(t, err, llm.ErrNotFound)
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
		assert.Equal(t, "weather_current", tools[0].Name())
		assert.Equal(t, "news_search", tools[1].Name())
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

	t.Run("Conflict", func(t *testing.T) {
		// Two tools can't have the same new name in the same call, but each
		// can be got
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		require.NoError(t, tk.Add("my_tools", echoTool{name: "search"}))
		require.NoError(t, tk.Add("my", echoTool{name: "tools_search"}))
		_, err = tk.Tools(ctx, "my_tools:search", "my:tools_search")
		assert.ErrorIs(t, err, llm.ErrConflict)
		for _, name := range []string{"my_tools:search", "my:tools_search"} {
			tools, err := tk.Tools(ctx, name)
			require.NoError(t, err, name)
			assert.Equal(t, "my_tools_search", tools[0].Name(), name)
		}
	})

	t.Run("TooLong", func(t *testing.T) {
		// The new name is at most 64 characters, which is the namespace, an
		// underscore and the name
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		namespace := "n" + strings.Repeat("a", 39)
		require.NoError(t, tk.Add(namespace, echoTool{name: strings.Repeat("b", 24)}, echoTool{name: strings.Repeat("c", 23)}))
		_, err = tk.Tools(ctx, namespace+":"+strings.Repeat("b", 24))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = tk.Tools(ctx, namespace+":"+strings.Repeat("c", 23))
		assert.NoError(t, err)
	})
}

func TestToolsRun(t *testing.T) {
	// A tool from Tools is run as with CallTool, so its input is validated,
	// and its result is JSON
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add("test",
		schemaTool{echoTool: echoTool{name: "weather"}},
		resultTool{echoTool: echoTool{name: "text"}, result: "It's sunny"},
		resultTool{echoTool: echoTool{name: "none"}, result: nil},
	))
	tools, err := tk.Tools(ctx, "test:*")
	require.NoError(t, err)
	byName := make(map[string]llm.Tool, len(tools))
	for _, tool := range tools {
		byName[tool.Name()] = tool
	}

	result, err := byName["test_weather"].Run(ctx, json.RawMessage(`{"city":"London"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"city":"London"}`, string(result.(json.RawMessage)))

	_, err = byName["test_weather"].Run(ctx, json.RawMessage(`{"days":3}`))
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	result, err = byName["test_text"].Run(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, json.RawMessage(`"It's sunny"`), result)

	result, err = byName["test_none"].Run(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}
