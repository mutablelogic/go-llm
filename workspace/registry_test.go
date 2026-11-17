package workspace_test

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	workspace "github.com/mutablelogic/go-llm/workspace"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// empty is a workspace with no resources
type empty struct{ name string }

func (w empty) Name() string        { return w.name }
func (w empty) Title() string       { return "Empty" }
func (w empty) Description() string { return "A workspace with no resources" }
func (w empty) List(context.Context, workspace.ListRequest) (*workspace.ListResponse, error) {
	return &workspace.ListResponse{}, nil
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
