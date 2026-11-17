package memory_test

import (
	"strings"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	memory "github.com/mutablelogic/go-llm/memory"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	workspace "github.com/mutablelogic/go-llm/workspace"
	workspaceschema "github.com/mutablelogic/go-llm/workspace/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Workspace(t *testing.T) {
	// The memory is a workspace of its details, and its memories, as
	// Markdown, which says as they change
	ctx := t.Context()
	b := broadcaster.New()
	m, err := memory.New(ctx, memory.NewStore(), memory.WithBroadcaster(b))
	require.NoError(t, err)
	var _ workspace.Watcher = m
	_, err = m.SetDetail(ctx, schema.FieldName, &schema.Detail{Value: "Sam"})
	require.NoError(t, err)
	_, _, err = m.Remember(ctx, schema.MemoryMeta{Key: "kitchen_lights", Description: "The kitchen's lights", Body: "light.kitchen_main"}, "")
	require.NoError(t, err)

	uris := func(list *workspaceschema.ListResponse) []string {
		var result []string
		for _, r := range list.Body {
			result = append(result, r.URI)
		}
		return result
	}

	t.Run("List", func(t *testing.T) {
		list, err := m.List(ctx, workspaceschema.ListRequest{})
		require.NoError(t, err)
		assert.Equal(t, []string{memory.DetailsURI, memory.MemoriesURI}, uris(list))
		assert.True(t, list.Body[1].IsFolder())
		list, err = m.List(ctx, workspaceschema.ListRequest{Path: "memories"})
		require.NoError(t, err)
		assert.Equal(t, []string{"memory:///memories/kitchen_lights.md"}, uris(list))
		assert.Equal(t, "The kitchen's lights", list.Body[0].Description)
		list, err = m.List(ctx, workspaceschema.ListRequest{Recursive: true, Limit: 2})
		require.NoError(t, err)
		assert.Len(t, list.Body, 2)
		require.Equal(t, "2", list.Next)
		list, err = m.List(ctx, workspaceschema.ListRequest{Recursive: true, Cursor: list.Next})
		require.NoError(t, err)
		assert.Equal(t, []string{"memory:///memories/kitchen_lights.md"}, uris(list))
		_, err = m.List(ctx, workspaceschema.ListRequest{Path: "other"})
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Read", func(t *testing.T) {
		resource, err := m.Read(ctx, memory.DetailsURI)
		require.NoError(t, err)
		data, err := llm.ReadAll(ctx, resource)
		require.NoError(t, err)
		assert.Contains(t, string(data), "- **Name:** Sam")

		resource, err = m.Read(ctx, "memory:///memories/kitchen_lights.md")
		require.NoError(t, err)
		assert.Equal(t, "text/markdown", resource.Type())
		data, err = llm.ReadAll(ctx, resource)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(string(data), "---\nkey: kitchen_lights\n"), string(data))
		assert.Contains(t, string(data), "\nlight.kitchen_main\n")

		_, err = m.Read(ctx, "memory:///memories/missing.md")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = m.Read(ctx, memory.MemoriesURI)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Watch", func(t *testing.T) {
		ch := make(chan workspaceschema.Change, 16)
		done := make(chan error, 1)
		go func() { done <- m.WatchResources(t.Context(), ch) }()
		next := func() workspaceschema.Change {
			t.Helper()
			select {
			case change := <-ch:
				return change
			case <-time.After(5 * time.Second):
				t.Fatal("no change")
				return workspaceschema.Change{}
			}
		}
		assert.Equal(t, workspaceschema.Change{Action: eventschema.EventUpdate}, next(), "it's watching")
		_, _, err := m.Remember(ctx, schema.MemoryMeta{Key: "garden", Description: "The garden", Body: "Roses"}, "")
		require.NoError(t, err)
		assert.Equal(t, workspaceschema.Change{Action: eventschema.EventCreate, URI: "memory:///memories/garden.md"}, next())
		_, err = m.SetDetail(ctx, schema.FieldLocation, &schema.Detail{Value: "Berlin"})
		require.NoError(t, err)
		assert.Equal(t, workspaceschema.Change{Action: eventschema.EventUpdate, URI: memory.DetailsURI}, next())
		_, err = m.Forget(ctx, "garden")
		require.NoError(t, err)
		assert.Equal(t, workspaceschema.Change{Action: eventschema.EventDelete, URI: "memory:///memories/garden.md"}, next())
	})

	t.Run("NoBroadcaster", func(t *testing.T) {
		m, err := memory.New(ctx, memory.NewStore())
		require.NoError(t, err)
		err = m.WatchResources(ctx, make(chan workspaceschema.Change))
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
	})
}
