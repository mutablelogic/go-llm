package workspace_test

import (
	"context"
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
type empty struct{ name string }

func (w empty) Name() string        { return w.name }
func (w empty) Title() string       { return "Empty" }
func (w empty) Description() string { return "A workspace with no resources" }
func (w empty) List(context.Context, schema.ListRequest) (*schema.ListResponse, error) {
	return &schema.ListResponse{}, nil
}

func Test_Registry(t *testing.T) {
	r := workspace.New()

	// Workspaces are added, by their names, which are unique identifiers
	require.NoError(t, r.Add(empty{"notes"}))
	require.NoError(t, r.Add(empty{"files"}))
	assert.ErrorIs(t, r.Add(empty{"notes"}), llm.ErrConflict)
	assert.ErrorIs(t, r.Add(empty{"not a name"}), llm.ErrBadParameter)
	assert.ErrorIs(t, r.Add(nil), llm.ErrBadParameter)

	// They're got, and listed, in order of their names
	ws, err := r.Get("notes")
	require.NoError(t, err)
	assert.Equal(t, "notes", ws.Name())
	var names []string
	for _, ws := range r.Workspaces() {
		names = append(names, ws.Name())
	}
	assert.Equal(t, []string{"files", "notes"}, names)

	// And removed
	_, err = r.Remove("notes")
	require.NoError(t, err)
	_, err = r.Get("notes")
	assert.ErrorIs(t, err, llm.ErrNotFound)
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
	require.NoError(t, r.Add(empty{"empty"}))
	require.NoError(t, r.Add(readable{empty{"notes"}}))
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
