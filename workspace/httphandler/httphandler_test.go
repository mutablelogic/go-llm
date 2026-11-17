package httphandler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	test "github.com/mutablelogic/go-llm/registry/test"
	workspace "github.com/mutablelogic/go-llm/workspace"
	httphandler "github.com/mutablelogic/go-llm/workspace/httphandler"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// notes is a workspace of a folder, and a note, which lists by folder, and
// reads its notes
type notes struct{}

func (notes) List(_ context.Context, req schema.ListRequest) (*schema.ListResponse, error) {
	if req.Path == "2026" {
		return &schema.ListResponse{Body: []schema.ResourceMeta{{URI: "file:///2026/b.md", Name: "b.md", Type: "text/markdown"}}}, nil
	}
	return &schema.ListResponse{Body: []schema.ResourceMeta{
		{URI: "file:///2026/", Name: "2026", Type: schema.FolderType},
		{URI: "file:///a.md", Name: "a.md", Type: "text/markdown"},
	}, Next: "more"}, nil
}

func (notes) Read(context.Context, string) (llm.Resource, error) {
	return nil, llm.ErrNotFound
}

// get requests the path, and decodes its JSON, and returns its status
func get(t *testing.T, server *test.Server, path string, v any) int {
	t.Helper()
	resp, err := http.Get(server.URL + test.Prefix + path)
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK && v != nil {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(v))
	}
	return resp.StatusCode
}

func Test_Workspaces(t *testing.T) {
	registry, err := workspace.New()
	require.NoError(t, err)
	require.NoError(t, registry.Add("notes", notes{}))
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(registry, router)
	})

	t.Run("List", func(t *testing.T) {
		var list schema.WorkspaceList
		require.Equal(t, http.StatusOK, get(t, server, "/llm/workspace", &list))
		assert.Equal(t, schema.WorkspaceList{Count: 1, Body: []schema.Workspace{{Name: "notes", Read: true}}}, list)
	})

	t.Run("Get", func(t *testing.T) {
		var ws schema.Workspace
		require.Equal(t, http.StatusOK, get(t, server, "/llm/workspace/notes", &ws))
		assert.Equal(t, schema.Workspace{Name: "notes", Read: true}, ws)
		assert.Equal(t, http.StatusNotFound, get(t, server, "/llm/workspace/missing", nil))
	})

	t.Run("Resources", func(t *testing.T) {
		var list schema.ListResponse
		require.Equal(t, http.StatusOK, get(t, server, "/llm/workspace/notes/resource", &list))
		require.Len(t, list.Body, 2)
		assert.True(t, list.Body[0].IsFolder())
		assert.Equal(t, "more", list.Next)

		list = schema.ListResponse{}
		require.Equal(t, http.StatusOK, get(t, server, "/llm/workspace/notes/resource?path=2026&limit=10", &list))
		require.Len(t, list.Body, 1)
		assert.Equal(t, "file:///2026/b.md", list.Body[0].URI)
		assert.Equal(t, http.StatusNotFound, get(t, server, "/llm/workspace/missing/resource", nil))
		assert.Equal(t, http.StatusBadRequest, get(t, server, "/llm/workspace/notes/resource?limit=x", nil))
	})
}
