package workspace_test

import (
	"context"
	"maps"
	"slices"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// empty is a workspace with no resources
type empty struct{}

func (empty) List(context.Context, schema.ListRequest) (*schema.ListResponse, error) {
	return &schema.ListResponse{}, nil
}

func Test_Registry(t *testing.T) {
	r := workspace.New()

	// Workspaces are added, by their names, which are unique identifiers
	require.NoError(t, r.Add("notes", empty{}))
	require.NoError(t, r.Add("files", empty{}))
	assert.ErrorIs(t, r.Add("notes", empty{}), llm.ErrConflict)
	assert.ErrorIs(t, r.Add("not a name", empty{}), llm.ErrBadParameter)
	assert.ErrorIs(t, r.Add("nil", nil), llm.ErrBadParameter)

	// They're got, and listed, in order of their names
	require.NoError(t, r.With("notes", func(ws workspace.Workspace) error {
		assert.Equal(t, empty{}, ws)
		return nil
	}))
	assert.ErrorIs(t, r.With("notes", func(workspace.Workspace) error { return llm.ErrConflict }), llm.ErrConflict, "its error")
	var names []string
	for name := range r.All() {
		names = append(names, name)
	}
	assert.Equal(t, []string{"files", "notes"}, names)

	// And removed
	_, err := r.Remove("notes")
	require.NoError(t, err)
	assert.ErrorIs(t, r.With("notes", func(workspace.Workspace) error { return nil }), llm.ErrNotFound)
	_, err = r.Remove("notes")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

// readable is a workspace whose one resource is read
type readable struct{ empty }

func (readable) Read(_ context.Context, uri string) (llm.Resource, error) {
	if uri != "file:///note.md" {
		return nil, llm.ErrNotFound.With(uri)
	}
	return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "text/markdown", Data: []byte("# Note")}}, nil
}

func Test_Read(t *testing.T) {
	// A workspace which is a Reader is read, by URI, and one which isn't
	// isn't
	ctx := context.Background()
	r := workspace.New()
	require.NoError(t, r.Add("empty", empty{}))
	require.NoError(t, r.Add("notes", readable{}))
	resource, err := r.Read(ctx, "notes", "file:///note.md")
	require.NoError(t, err)
	data, err := llm.ReadAll(ctx, resource)
	require.NoError(t, err)
	assert.Equal(t, "# Note", string(data))
	_, err = r.Read(ctx, "notes", "file:///other.md")
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = r.Read(ctx, "empty", "file:///note.md")
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
	_, err = r.Read(ctx, "missing", "file:///note.md")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

// sometimes is a workspace which is one only while it's available
type sometimes struct {
	readable
	available bool
}

func (s *sometimes) Available() bool { return s.available }

func Test_Available(t *testing.T) {
	// A workspace which isn't available isn't listed, or used, until it is,
	// and Resolve reads from any workspace which has the resource
	ctx := context.Background()
	r := workspace.New()
	ws := &sometimes{}
	require.NoError(t, r.Add("notes", ws))
	assert.Empty(t, slices.Collect(maps.Keys(maps.Collect(r.All()))))
	assert.ErrorIs(t, r.With("notes", func(workspace.Workspace) error { return nil }), llm.ErrNotFound)
	_, err := r.Resolve(ctx, "file:///note.md")
	assert.ErrorIs(t, err, llm.ErrNotFound)

	ws.available = true
	require.NoError(t, r.With("notes", func(workspace.Workspace) error { return nil }))
	resource, err := r.Resolve(ctx, "file:///note.md")
	require.NoError(t, err)
	assert.Equal(t, "text/markdown", resource.Type())
	_, err = r.Resolve(ctx, "file:///other.md")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

func Test_RemoveIf(t *testing.T) {
	// A workspace is removed only when it's the one with the name
	r := workspace.New()
	first, second := &sometimes{available: true}, &sometimes{available: true}
	require.NoError(t, r.Add("notes", first))
	assert.False(t, r.RemoveIf("notes", second), "another workspace")
	assert.True(t, r.RemoveIf("notes", first))
	assert.False(t, r.RemoveIf("notes", first), "once it's removed")
}
