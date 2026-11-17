package workspace_test

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// rooted is a workspace whose resources are under file:///, and whose
// artwork isn't
type rooted struct{ readable }

func (rooted) Root() string { return "file:///" }

func (rooted) List(context.Context, schema.ListRequest) (*schema.ListResponse, error) {
	return &schema.ListResponse{Body: []schema.ResourceMeta{{
		URI:     "file:///note.md",
		Name:    "note.md",
		Artwork: []schema.Artwork{{Key: "thumbnail", URI: "artwork:///note.png"}},
	}}}, nil
}

func Test_Root(t *testing.T) {
	// A workspace's root is left out of its resources' URIs, and is put
	// back, as they're read, which they are by their URIs as they were too
	ctx := context.Background()
	r, err := workspace.New()
	require.NoError(t, err)
	var ws workspace.Rooted = rooted{}
	require.NoError(t, r.Add("notes", ws))

	list, err := r.ListResources(ctx, "notes", schema.ListRequest{})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	assert.Equal(t, "resource://notes/note.md", list.Body[0].URI)
	assert.Equal(t, "resource://notes/artwork:///note.png", list.Body[0].Artwork[0].URI, "not under its root")

	for _, uri := range []string{"resource://notes/note.md", "resource://notes/file:///note.md", "file:///note.md"} {
		resource, err := r.Resolve(ctx, uri)
		require.NoError(t, err, uri)
		assert.Equal(t, "resource://notes/note.md", resource.URI(), uri)
		data, err := llm.ReadAll(ctx, resource)
		require.NoError(t, err)
		assert.Equal(t, "# Note", string(data))
	}
	_, err = r.Resolve(ctx, "resource://notes/other.md")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

func Test_ReadErrorURI(t *testing.T) {
	// An error names the resource by the URI it was asked for by, rather
	// than the workspace's own
	ctx := context.Background()
	r, err := workspace.New()
	require.NoError(t, err)
	require.NoError(t, r.Add("notes", rooted{}))
	_, err = r.Resolve(ctx, "resource://notes/missing.md")
	require.ErrorIs(t, err, llm.ErrNotFound)
	assert.Contains(t, err.Error(), "resource://notes/missing.md")
	assert.NotContains(t, err.Error(), "file:///missing.md")
}
