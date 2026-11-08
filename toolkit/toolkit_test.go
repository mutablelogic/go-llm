package toolkit_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// tool is a tool for tests
type tool struct {
	toolkit.Base
	name string
}

func (t tool) Name() string                                      { return t.name }
func (t tool) Description() string                               { return "A tool for tests" }
func (t tool) InputSchema() *jsonschema.Schema                   { return nil }
func (t tool) Run(context.Context, json.RawMessage) (any, error) { return nil, nil }

// names returns the canonical names of all the tools in the toolkit
func names(t *testing.T, tk *toolkit.Toolkit) []string {
	t.Helper()
	list, err := tk.ListTools(context.Background(), schema.ToolListRequest{})
	require.NoError(t, err)
	var result []string
	for _, tool := range list.Body {
		result = append(result, tool.Canonical)
	}
	return result
}

func TestAdd(t *testing.T) {
	ctx := context.Background()

	t.Run("Empty", func(t *testing.T) {
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		assert.Empty(t, names(t, tk))
	})

	t.Run("Add", func(t *testing.T) {
		// The tools are in order of canonical name, which is the namespace
		// and the tool's name separated by a colon
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		require.NoError(t, tk.Add(toolkit.NewNamespace("b", "", tool{name: "a"})))
		require.NoError(t, tk.Add(toolkit.NewNamespace("a", "", tool{name: "z"}, tool{name: "y"})))
		assert.Equal(t, []string{"a:y", "a:z", "b:a"}, names(t, tk))
	})

	t.Run("SameName", func(t *testing.T) {
		// Tools in different namespaces can have the same name
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		require.NoError(t, tk.Add(toolkit.NewNamespace("a", "", tool{name: "search"})))
		require.NoError(t, tk.Add(toolkit.NewNamespace("b", "", tool{name: "search"})))
		assert.Equal(t, []string{"a:search", "b:search"}, names(t, tk))
	})

	t.Run("Namespace", func(t *testing.T) {
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		for _, namespace := range []string{"", "1abc", "not valid", "a:b"} {
			assert.ErrorIs(t, tk.Add(toolkit.NewNamespace(namespace, "", tool{name: "a"})), llm.ErrBadParameter, namespace)
		}
		assert.Empty(t, names(t, tk))
	})

	t.Run("NamespaceConflict", func(t *testing.T) {
		// A namespace can only be added once
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		require.NoError(t, tk.Add(toolkit.NewNamespace("weather", "", tool{name: "a"})))
		assert.ErrorIs(t, tk.Add(toolkit.NewNamespace("weather", "", tool{name: "b"})), llm.ErrConflict)
		assert.Equal(t, []string{"weather:a"}, names(t, tk))
	})

	t.Run("Invalid", func(t *testing.T) {
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		assert.ErrorIs(t, tk.Add(toolkit.NewNamespace("a", "", nil)), llm.ErrBadParameter)
		assert.ErrorIs(t, tk.Add(toolkit.NewNamespace("a", "", tool{name: "not valid"})), llm.ErrBadParameter)
		assert.Empty(t, names(t, tk))

		// The namespace wasn't added, so it can be added later
		assert.NoError(t, tk.Add(toolkit.NewNamespace("a", "", tool{name: "a"})))
	})

	t.Run("Duplicate", func(t *testing.T) {
		// None of the tools are added when two have the same name
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		assert.ErrorIs(t, tk.Add(toolkit.NewNamespace("a", "", tool{name: "b"}, tool{name: "b"})), llm.ErrConflict)
		assert.Empty(t, names(t, tk))
	})
}

// credentials is a credential provider for tests, with one credential
type credentials struct{}

// Get isn't used by the tests
func (credentials) Get(context.Context, string) (json.RawMessage, error) {
	return nil, llm.ErrNotImplemented
}

// Set isn't used by the tests
func (credentials) Set(context.Context, string, json.RawMessage) error {
	return llm.ErrNotImplemented
}

func (credentials) Expand(_ context.Context, value string) (string, error) {
	switch value {
	case "${weather_key}":
		return "secret", nil
	case "${empty_key}":
		return " ", nil
	}
	return "", llm.ErrNotFound.Withf("credential %q", value)
}

func TestCredential(t *testing.T) {
	ctx := context.Background()

	t.Run("Credential", func(t *testing.T) {
		tk, err := toolkit.New(ctx, toolkit.WithCredentialsProvider(credentials{}))
		require.NoError(t, err)
		value, err := tk.Credential(ctx, " weather_key ")
		require.NoError(t, err)
		assert.Equal(t, "secret", value)
	})

	t.Run("NotFound", func(t *testing.T) {
		tk, err := toolkit.New(ctx, toolkit.WithCredentialsProvider(credentials{}))
		require.NoError(t, err)
		_, err = tk.Credential(ctx, "other_key")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Empty", func(t *testing.T) {
		// A credential which is empty is as if it isn't set
		tk, err := toolkit.New(ctx, toolkit.WithCredentialsProvider(credentials{}))
		require.NoError(t, err)
		_, err = tk.Credential(ctx, "empty_key")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		assert.ErrorContains(t, err, "is empty")
	})

	t.Run("EmptyEnvironment", func(t *testing.T) {
		// An environment variable which is set to nothing is as if it isn't
		t.Setenv("TOOLKIT_TEST_EMPTY", "")
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		_, err = tk.Credential(ctx, "TOOLKIT_TEST_EMPTY")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("MissingKey", func(t *testing.T) {
		tk, err := toolkit.New(ctx, toolkit.WithCredentialsProvider(credentials{}))
		require.NoError(t, err)
		_, err = tk.Credential(ctx, " ")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("NoCredentials", func(t *testing.T) {
		// Without a credential provider, there are no credentials
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		_, err = tk.Credential(ctx, "weather_key")
		assert.Error(t, err)
	})
}

func TestClientOpts(t *testing.T) {
	ctx := context.Background()

	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	assert.Empty(t, tk.ClientOpts())

	tk, err = toolkit.New(ctx, toolkit.WithClientOpts(client.OptTrace(nil, false)))
	require.NoError(t, err)
	assert.Len(t, tk.ClientOpts(), 1)
}

func TestListTools(t *testing.T) {
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(toolkit.NewNamespace("a", "", tool{name: "one"}, tool{name: "two"}, tool{name: "three"})))
	require.NoError(t, tk.Add(toolkit.NewNamespace("ab", "", tool{name: "four"})))

	t.Run("All", func(t *testing.T) {
		list, err := tk.ListTools(ctx, schema.ToolListRequest{})
		require.NoError(t, err)
		assert.Equal(t, uint64(4), list.Count)
		require.Len(t, list.Body, 4)

		// The tool's names, description and schemas are in the list
		assert.Equal(t, "one", list.Body[0].Name)
		assert.Equal(t, "a:one", list.Body[0].Canonical)
		assert.Equal(t, "A tool for tests", list.Body[0].Description)
		assert.Nil(t, list.Body[0].Input)
		assert.Nil(t, list.Body[0].Output)
	})

	t.Run("Namespace", func(t *testing.T) {
		// Only the tools in the namespace, and not a namespace which starts
		// with the same letters
		list, err := tk.ListTools(ctx, schema.ToolListRequest{Namespace: "a"})
		require.NoError(t, err)
		assert.Equal(t, uint64(3), list.Count)
		var names []string
		for _, tool := range list.Body {
			names = append(names, tool.Canonical)
		}
		assert.Equal(t, []string{"a:one", "a:three", "a:two"}, names)

		list, err = tk.ListTools(ctx, schema.ToolListRequest{Namespace: "ab"})
		require.NoError(t, err)
		assert.Equal(t, uint64(1), list.Count)
	})

	t.Run("UnknownNamespace", func(t *testing.T) {
		list, err := tk.ListTools(ctx, schema.ToolListRequest{Namespace: "other"})
		require.NoError(t, err)
		assert.Zero(t, list.Count)
		assert.Empty(t, list.Body)
	})

	t.Run("InvalidNamespace", func(t *testing.T) {
		_, err := tk.ListTools(ctx, schema.ToolListRequest{Namespace: "not valid"})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Query", func(t *testing.T) {
		assert.Empty(t, schema.ToolListRequest{}.Query())
		q := schema.ToolListRequest{Namespace: " weatherapi "}.Query()
		assert.Equal(t, "namespace=weatherapi", q.Encode())
	})
}

func TestCanonicalToolName(t *testing.T) {
	assert.Equal(t, "weatherapi:current", schema.CanonicalToolName("weatherapi", "current"))
}

func TestConcurrent(t *testing.T) {
	// Tools can be added while they're listed
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			assert.NoError(t, tk.Add(toolkit.NewNamespace(fmt.Sprint("ns", i), "", tool{name: "a"}, tool{name: "b"})))
		}()
		go func() {
			defer wg.Done()
			_, err := tk.ListTools(ctx, schema.ToolListRequest{})
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	list, err := tk.ListTools(ctx, schema.ToolListRequest{})
	require.NoError(t, err)
	assert.Equal(t, uint64(20), list.Count)
}

func TestToolTable(t *testing.T) {
	tool := schema.Tool{Name: "current", Canonical: "weatherapi:current", Type: schema.NamespaceBuiltin, Description: "Get the weather"}
	assert.Equal(t, []string{"Name", "Type", "Description"}, tool.Header())
	assert.Equal(t, "weatherapi:current", tool.Cell(0))
	assert.Equal(t, schema.NamespaceBuiltin, tool.Cell(1))
	assert.Equal(t, "Get the weather", tool.Cell(2))
	assert.Equal(t, "", tool.Cell(3))
}

func TestRemove(t *testing.T) {
	ctx := context.Background()

	// A namespace is removed, with its tools, which are said to be deleted
	b := broadcaster.New()
	ch := make(chan eventschema.Event, 16)
	watchCtx, cancel := context.WithCancel(ctx)
	done, err := b.Watch(watchCtx, ch)
	require.NoError(t, err)
	defer func() {
		cancel()
		assert.NoError(t, <-done)
	}()
	tk, err := toolkit.New(ctx, toolkit.WithBroadcaster(b))
	require.NoError(t, err)
	ns := toolkit.NewNamespace("a", "", tool{name: "z"}, tool{name: "y"})
	require.NoError(t, tk.Add(ns))
	require.NoError(t, tk.Add(toolkit.NewNamespace("b", "", tool{name: "x"})))
	assert.True(t, tk.HasNamespace("a"))
	for len(ch) > 0 {
		<-ch
	}
	require.NoError(t, tk.Remove(ns))
	assert.False(t, tk.HasNamespace("a"))
	assert.Equal(t, []string{"b:x"}, names(t, tk))
	var deleted []string
	for len(ch) > 0 {
		event := <-ch
		assert.Equal(t, eventschema.EventDelete, event.Action)
		deleted = append(deleted, event.Row)
	}
	assert.Equal(t, []string{"a:y", "a:z"}, deleted)

	// But not twice, nor one which replaced it, nor none
	assert.ErrorIs(t, tk.Remove(ns), llm.ErrNotFound)
	require.NoError(t, tk.Add(toolkit.NewNamespace("a", "", tool{name: "w"})))
	assert.ErrorIs(t, tk.Remove(ns), llm.ErrNotFound)
	assert.True(t, tk.HasNamespace("a"))
	assert.ErrorIs(t, tk.Remove(nil), llm.ErrBadParameter)
}

// typedNamespace is a namespace of the type
type typedNamespace struct {
	toolkit.Namespace
	typ string
}

func (n typedNamespace) Type() string { return n.typ }

func TestNamespaceType(t *testing.T) {
	// A namespace, and its tools, have its type, by which tools are listed
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(toolkit.NewNamespace("builtin", "", tool{name: "one"})))
	require.NoError(t, tk.Add(&typedNamespace{toolkit.NewNamespace("server", "", tool{name: "two"}), schema.NamespaceMCP}))
	require.NoError(t, tk.Add(&typedNamespace{toolkit.NewNamespace("agent", "", tool{name: "three"}), schema.NamespaceAgent}))

	namespaces, err := tk.ListNamespaces(ctx)
	require.NoError(t, err)
	types := map[string]string{}
	for _, ns := range namespaces.Body {
		types[ns.Name] = ns.Type
	}
	assert.Equal(t, map[string]string{"builtin": schema.NamespaceBuiltin, "server": schema.NamespaceMCP, "agent": schema.NamespaceAgent}, types)

	list, err := tk.ListTools(ctx, schema.ToolListRequest{})
	require.NoError(t, err)
	types = map[string]string{}
	for _, tool := range list.Body {
		types[tool.Canonical] = tool.Type
	}
	assert.Equal(t, map[string]string{"builtin:one": schema.NamespaceBuiltin, "server:two": schema.NamespaceMCP, "agent:three": schema.NamespaceAgent}, types)

	list, err = tk.ListTools(ctx, schema.ToolListRequest{Type: schema.NamespaceMCP})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	assert.Equal(t, "server:two", list.Body[0].Canonical)
	_, err = tk.ListTools(ctx, schema.ToolListRequest{Type: "other"})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	got, err := tk.GetTool(ctx, "agent:three")
	require.NoError(t, err)
	assert.Equal(t, schema.NamespaceAgent, got.Type)
}
