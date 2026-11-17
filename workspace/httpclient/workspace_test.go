package httpclient_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	workspace "github.com/mutablelogic/go-llm/workspace"
	httpclient "github.com/mutablelogic/go-llm/workspace/httpclient"
	httphandler "github.com/mutablelogic/go-llm/workspace/httphandler"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// notes is a workspace of a folder, and a note, which lists by folder, and
// pages, and says what it was asked for
type notes struct {
	req *schema.ListRequest
}

func (n notes) List(_ context.Context, req schema.ListRequest) (*schema.ListResponse, error) {
	*n.req = req
	if req.Path == "2026" {
		return &schema.ListResponse{Body: []schema.ResourceMeta{{URI: "file:///2026/b.md", Name: "b.md", Type: "text/markdown"}}}, nil
	}
	return &schema.ListResponse{Body: []schema.ResourceMeta{
		{URI: "file:///2026/", Name: "2026", Type: schema.FolderType},
		{URI: "file:///a.md", Name: "a.md", Type: "text/markdown", Artwork: []schema.Artwork{{Key: "thumbnail", Type: "image/png", URI: "artwork:///a"}}},
	}, Next: "more"}, nil
}

func (notes) Read(_ context.Context, uri string) (llm.Resource, error) {
	if uri != "file:///a.md" {
		return nil, llm.ErrNotFound.With(uri)
	}
	return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "text/markdown", Data: []byte("# A")}}, nil
}

func Test_Client(t *testing.T) {
	ctx := context.Background()
	registry, err := workspace.New()
	require.NoError(t, err)
	var asked schema.ListRequest
	require.NoError(t, registry.Add("notes", notes{req: &asked}))
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(registry, router)
	})
	c, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)

	t.Run("ListWorkspaces", func(t *testing.T) {
		list, err := c.ListWorkspaces(ctx)
		require.NoError(t, err)
		assert.Equal(t, &schema.WorkspaceList{Count: 1, Body: []schema.Workspace{{Name: "notes", Read: true}}}, list)
	})

	t.Run("GetWorkspace", func(t *testing.T) {
		ws, err := c.GetWorkspace(ctx, "notes")
		require.NoError(t, err)
		assert.Equal(t, &schema.Workspace{Name: "notes", Read: true}, ws)
		_, err = c.GetWorkspace(ctx, "missing")
		assert.Equal(t, http.StatusNotFound, status(t, err))
	})

	t.Run("ReadResource", func(t *testing.T) {
		var buf bytes.Buffer
		contentType, err := c.ReadResource(ctx, "notes", schema.ReadRequest{URI: "resource://notes/file:///a.md"}, &buf)
		require.NoError(t, err)
		assert.Equal(t, "text/markdown", contentType)
		assert.Equal(t, "# A", buf.String())

		buf.Reset()
		_, err = c.ReadResource(ctx, "notes", schema.ReadRequest{URI: "resource://notes/file:///a.md", Format: "markdown"}, &buf)
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "---\n")

		_, err = c.ReadResource(ctx, "notes", schema.ReadRequest{URI: "file:///missing.md"}, &buf)
		assert.Equal(t, http.StatusNotFound, status(t, err))
	})

	t.Run("ListResources", func(t *testing.T) {
		list, err := c.ListResources(ctx, "notes", schema.ListRequest{})
		require.NoError(t, err)
		require.Len(t, list.Body, 2)
		assert.True(t, list.Body[0].IsFolder())
		assert.Equal(t, "resource://notes/file:///a.md", list.Body[1].URI)
		assert.Equal(t, "resource://notes/artwork:///a", list.Body[1].Artwork[0].URI)
		assert.Equal(t, "more", list.Next)

		// With the folder, recursion, cursor and limit, as the workspace is
		// asked
		req := schema.ListRequest{Path: "2026", Recursive: true, Cursor: "more", Limit: 10}
		list, err = c.ListResources(ctx, "notes", req)
		require.NoError(t, err)
		require.Len(t, list.Body, 1)
		assert.Equal(t, "resource://notes/file:///2026/b.md", list.Body[0].URI)
		assert.Equal(t, req, asked)

		// With spaces, as they are
		req = schema.ListRequest{Path: " my notes ", Cursor: " opaque= "}
		_, err = c.ListResources(ctx, "notes", req)
		require.NoError(t, err)
		assert.Equal(t, req, asked, "as they are, with their spaces")

		_, err = c.ListResources(ctx, "missing", schema.ListRequest{})
		assert.Equal(t, http.StatusNotFound, status(t, err))
	})
}

// status returns the status of the error, which is the API's
func status(t *testing.T, err error) int {
	t.Helper()
	var response httpresponse.ErrResponse
	require.True(t, errors.As(err, &response), "error is %T: %v", err, err)
	return response.Code
}
