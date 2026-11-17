package server_test

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	client "github.com/mutablelogic/go-llm/mcp/client"
	server "github.com/mutablelogic/go-llm/mcp/server"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// files is a workspace of files, by their paths, such as notes/a.md, in
// memory, whose folders are those of the paths
type files map[string]string

func (files) Name() string        { return "files" }
func (files) Title() string       { return "Files" }
func (files) Description() string { return "Files in memory" }

// List lists the folders and files in, or under, the path, a page at a
// time, whose cursor is the index of the next
func (f files) List(_ context.Context, req schema.ListRequest) (*schema.ListResponse, error) {
	prefix := ""
	if req.Path != "" {
		prefix = strings.TrimSuffix(req.Path, "/") + "/"
	}
	seen := map[string]bool{}
	var all []schema.ResourceMeta
	for _, p := range slices.Sorted(func(yield func(string) bool) {
		for p := range f {
			if !yield(p) {
				return
			}
		}
	}) {
		rest, ok := strings.CutPrefix(p, prefix)
		if !ok {
			continue
		}
		parts := strings.Split(rest, "/")
		for i := range parts[:len(parts)-1] {
			folder := prefix + strings.Join(parts[:i+1], "/") + "/"
			if !seen[folder] && (req.Recursive || i == 0) {
				seen[folder] = true
				all = append(all, schema.ResourceMeta{URI: "file:///" + folder, Name: parts[i], Type: schema.FolderType})
			}
		}
		if req.Recursive || len(parts) == 1 {
			all = append(all, schema.ResourceMeta{
				URI:     "file:///" + p,
				Name:    parts[len(parts)-1],
				Type:    "text/markdown",
				Meta:    []schema.Meta{{NS: "markdown", Key: "title", Text: p, Value: json.RawMessage(strconv.Quote(p))}},
				Artwork: []schema.Artwork{{Key: "thumbnail", Type: "image/png", URI: "artwork:///" + p}},
			})
		}
	}
	start, _ := strconv.Atoi(req.Cursor)
	end := len(all)
	if req.Limit > 0 && start+int(req.Limit) < end {
		end = start + int(req.Limit)
	}
	response := &schema.ListResponse{Body: all[start:end]}
	if end < len(all) {
		response.Next = strconv.Itoa(end)
	}
	return response, nil
}

// Read reads a file, by its URI
func (f files) Read(_ context.Context, uri string) (llm.Resource, error) {
	text, exists := f[strings.TrimPrefix(uri, "file:///")]
	if !exists {
		return nil, llm.ErrNotFound.With(uri)
	}
	return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "text/markdown", Data: []byte(text)}}, nil
}

func Test_Workspace(t *testing.T) {
	// A workspace's folders, and files, are served, and listed, and read,
	// with go-llm's client, as a workspace
	ctx := t.Context()
	source := files{"readme.md": "# Readme", "notes/a.md": "# A", "notes/2026/b.md": "# B"}
	srv, err := server.New(&tools{}, server.WithWorkspace(source))
	require.NoError(t, err)
	c, err := client.New("memory://files", client.WithTransport(srv.InMemoryTransport))
	require.NoError(t, err)
	require.NoError(t, c.Connect(ctx))
	t.Cleanup(func() { assert.NoError(t, c.Disconnect()) })
	assert.True(t, c.HasResources())
	assert.True(t, c.HasExperimental(schema.Capability), "it's a workspace")
	ws, err := workspace.NewClient("files", c)
	require.NoError(t, err)

	uris := func(list *schema.ListResponse) []string {
		var result []string
		for _, r := range list.Body {
			result = append(result, r.URI)
		}
		return result
	}

	t.Run("Folder", func(t *testing.T) {
		// The top has its folder, and file, and a folder has its own
		list, err := ws.List(ctx, schema.ListRequest{})
		require.NoError(t, err)
		assert.Equal(t, []string{"file:///notes/", "file:///readme.md"}, uris(list))
		assert.True(t, list.Body[0].IsFolder())
		list, err = ws.List(ctx, schema.ListRequest{Path: "notes"})
		require.NoError(t, err)
		assert.Equal(t, []string{"file:///notes/2026/", "file:///notes/a.md"}, uris(list))
	})

	t.Run("Recursive", func(t *testing.T) {
		list, err := ws.List(ctx, schema.ListRequest{Path: "notes", Recursive: true})
		require.NoError(t, err)
		assert.Equal(t, []string{"file:///notes/2026/", "file:///notes/2026/b.md", "file:///notes/a.md"}, uris(list))
	})

	t.Run("Pages", func(t *testing.T) {
		list, err := ws.List(ctx, schema.ListRequest{Recursive: true, Limit: 2})
		require.NoError(t, err)
		assert.Len(t, list.Body, 2)
		require.NotEmpty(t, list.Next)
		next, err := ws.List(ctx, schema.ListRequest{Recursive: true, Limit: 2, Cursor: list.Next})
		require.NoError(t, err)
		assert.NotEqual(t, uris(list), uris(next))
	})

	t.Run("Meta", func(t *testing.T) {
		// A file's metadata, and artwork, are as the workspace has them
		list, err := ws.List(ctx, schema.ListRequest{Path: "notes"})
		require.NoError(t, err)
		a := list.Body[1]
		assert.Equal(t, []schema.Meta{{NS: "markdown", Key: "title", Text: "notes/a.md", Value: json.RawMessage(`"notes/a.md"`)}}, a.Meta)
		assert.Equal(t, []schema.Artwork{{Key: "thumbnail", Type: "image/png", URI: "artwork:///notes/a.md"}}, a.Artwork)
	})

	t.Run("Read", func(t *testing.T) {
		resource, err := ws.Read(ctx, "file:///notes/a.md")
		require.NoError(t, err)
		data, err := llm.ReadAll(ctx, resource)
		require.NoError(t, err)
		assert.Equal(t, "# A", string(data))
		assert.Equal(t, "text/markdown", resource.Type())
		_, err = ws.Read(ctx, "file:///missing.md")
		assert.Error(t, err)
	})
}
