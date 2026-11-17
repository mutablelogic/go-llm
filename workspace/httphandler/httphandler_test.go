package httphandler_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
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

// big is the content of big.bin, which is large
var big = strings.Repeat("0123456789", 1024*1024)

func (notes) Read(_ context.Context, uri string) (llm.Resource, error) {
	if uri == "file:///big.bin" {
		return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "application/octet-stream", Data: []byte(big)}}, nil
	} else if uri != "file:///a.md" {
		return nil, llm.ErrNotFound.With(uri)
	}
	return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "text/markdown", Data: []byte("# A\n\nHello")}}, nil
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
		assert.Equal(t, "resource://notes/file:///2026/b.md", list.Body[0].URI, "the workspace's URI")
		assert.Equal(t, http.StatusNotFound, get(t, server, "/llm/workspace/missing/resource", nil))
		assert.Equal(t, http.StatusBadRequest, get(t, server, "/llm/workspace/notes/resource?limit=x", nil))
	})
}

// request requests the path, with the headers, and returns the response,
// and its body
func request(t *testing.T, server *test.Server, path string, headers map[string]string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, server.URL+test.Prefix+path, nil)
	require.NoError(t, err)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(data)
}

func Test_Content(t *testing.T) {
	registry, err := workspace.New()
	require.NoError(t, err)
	require.NoError(t, registry.Add("notes", notes{}))
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(registry, router)
	})
	path := "/llm/workspace/notes/content?uri=" + url.QueryEscape("resource://notes/file:///a.md")

	t.Run("Raw", func(t *testing.T) {
		resp, body := request(t, server, path, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/markdown", resp.Header.Get("Content-Type"))
		assert.Equal(t, "# A\n\nHello", body)
		assert.Empty(t, resp.Header.Get("ETag"), "it's written as it's read")

		// In ranges
		resp, body = request(t, server, path, map[string]string{"Range": "bytes=0-2"})
		assert.Equal(t, http.StatusPartialContent, resp.StatusCode)
		assert.Equal(t, "# A", body)
	})

	t.Run("Large", func(t *testing.T) {
		// Large content is written as it's read, in ranges
		path := "/llm/workspace/notes/content?uri=" + url.QueryEscape("resource://notes/file:///big.bin")
		resp, body := request(t, server, path, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Empty(t, resp.Header.Get("ETag"))
		assert.Equal(t, strconv.Itoa(len(big)), resp.Header.Get("Content-Length"))
		assert.Equal(t, big, body)
		resp, body = request(t, server, path, map[string]string{"Range": "bytes=-5"})
		assert.Equal(t, http.StatusPartialContent, resp.StatusCode)
		assert.Equal(t, big[len(big)-5:], body)
	})

	t.Run("Markdown", func(t *testing.T) {
		resp, body := request(t, server, path+"&format=markdown", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.True(t, strings.HasPrefix(resp.Header.Get("Content-Type"), "text/markdown"))
		assert.True(t, strings.HasPrefix(body, "---\nuri: \"resource://notes/file:///a.md\"\n"), body)
		assert.Contains(t, body, "Hello")

		// Not again, when it's not modified
		etag := resp.Header.Get("ETag")
		require.NotEmpty(t, etag)
		resp, _ = request(t, server, path+"&format=markdown", map[string]string{"If-None-Match": etag})
		assert.Equal(t, http.StatusNotModified, resp.StatusCode)
	})

	t.Run("Errors", func(t *testing.T) {
		resp, _ := request(t, server, path+"&format=pdf", nil)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		resp, _ = request(t, server, "/llm/workspace/notes/content", nil)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "no uri")
		resp, _ = request(t, server, "/llm/workspace/notes/content?uri=file:///missing.md", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		resp, _ = request(t, server, "/llm/workspace/missing/content?uri=file:///a.md", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
