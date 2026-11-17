package filer_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	// Packages
	gofiler "github.com/mutablelogic/go-filer"
	filerfs "github.com/mutablelogic/go-filer/filer/fs"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	client "github.com/mutablelogic/go-llm/mcp/client"
	workspace "github.com/mutablelogic/go-llm/workspace"
	filer "github.com/mutablelogic/go-llm/workspace/filer"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// backend returns a go-filer backend of the directory
func backend(t *testing.T, dir string) gofiler.Filer {
	t.Helper()
	f, err := filerfs.New("code", dir, nil, gofiler.Opts{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// uris returns the URIs of the folders, and files, listed
func uris(list *schema.ListResponse) []string {
	var result []string
	for _, r := range list.Body {
		result = append(result, r.URI)
	}
	return result
}

func Test_Workspace(t *testing.T) {
	// The root of the code tree, as a workspace
	ctx := t.Context()
	ws, err := filer.New("code", backend(t, "../.."))
	require.NoError(t, err)

	t.Run("Top", func(t *testing.T) {
		list, err := ws.List(ctx, schema.ListRequest{Limit: filer.MaxLimit})
		require.NoError(t, err)
		assert.Contains(t, uris(list), "file:///go.mod")
		assert.Contains(t, uris(list), "file:///workspace/")
		for _, r := range list.Body {
			if r.URI == "file:///workspace/" {
				assert.True(t, r.IsFolder())
				assert.Equal(t, "workspace", r.Name)
			}
			if r.URI == "file:///go.mod" {
				assert.Positive(t, r.Size)
				assert.False(t, r.Modified.IsZero())
			}
			assert.False(t, strings.HasPrefix(r.Name, "."), "hidden files are skipped: %s", r.Name)
		}
	})

	t.Run("Folder", func(t *testing.T) {
		list, err := ws.List(ctx, schema.ListRequest{Path: "workspace/filer"})
		require.NoError(t, err)
		assert.Contains(t, uris(list), "file:///workspace/filer/filer.go")
		assert.NotContains(t, uris(list), "file:///workspace/filer/", "the folder isn't in itself")
	})

	t.Run("File", func(t *testing.T) {
		// A file's path lists the file
		list, err := ws.List(ctx, schema.ListRequest{Path: "workspace/filer/filer.go"})
		require.NoError(t, err)
		assert.Equal(t, []string{"file:///workspace/filer/filer.go"}, uris(list))
		assert.False(t, list.Body[0].IsFolder())
		assert.Positive(t, list.Body[0].Size)
	})

	t.Run("Recursive", func(t *testing.T) {
		list, err := ws.List(ctx, schema.ListRequest{Path: "workspace", Recursive: true, Limit: filer.MaxLimit})
		require.NoError(t, err)
		assert.Contains(t, uris(list), "file:///workspace/schema/uri.go")
	})

	t.Run("Pages", func(t *testing.T) {
		// Pages, from a cursor, which don't overlap, and are in order
		var all []string
		req := schema.ListRequest{Path: "workspace", Recursive: true, Limit: 3}
		for range 100 {
			list, err := ws.List(ctx, req)
			require.NoError(t, err)
			assert.LessOrEqual(t, len(list.Body), 3)
			all = append(all, uris(list)...)
			if list.Next == "" {
				break
			}
			req.Cursor = list.Next
		}
		whole, err := ws.List(ctx, schema.ListRequest{Path: "workspace", Recursive: true, Limit: filer.MaxLimit})
		require.NoError(t, err)
		assert.Equal(t, uris(whole), all)
	})

	t.Run("Read", func(t *testing.T) {
		resource, err := ws.Read(ctx, "file:///go.mod")
		require.NoError(t, err)
		assert.Equal(t, "go.mod", resource.Name())
		data, err := llm.ReadAll(ctx, resource)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(string(data), "module github.com/mutablelogic/go-llm"))
		n, ok := llm.ContentLength(resource)
		assert.True(t, ok)
		assert.Equal(t, int64(len(data)), n)

		_, err = ws.Read(ctx, "file:///workspace/")
		assert.ErrorIs(t, err, llm.ErrBadParameter, "a folder")
		_, err = ws.Read(ctx, "file:///missing.md")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = ws.Read(ctx, "http://example.com/go.mod")
		assert.ErrorIs(t, err, llm.ErrBadParameter, "not a file's URI")
	})
}

func Test_Watch(t *testing.T) {
	// A file which is created is said to be, once it's watching
	ctx := t.Context()
	dir := t.TempDir()
	ws, err := filer.New("code", backend(t, dir))
	require.NoError(t, err)
	changes := make(chan schema.Change, 16)
	watchCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- ws.WatchResources(watchCtx, changes) }()
	select {
	case change := <-changes:
		assert.Equal(t, schema.Change{Action: eventschema.EventUpdate}, change, "it's watching")
	case err := <-done:
		// A backend which can't watch, such as on macOS without cgo, says so
		require.ErrorIs(t, err, llm.ErrNotImplemented)
		t.Skip("the backend doesn't watch here:", err)
	case <-time.After(10 * time.Second):
		t.Fatal("the watch didn't start")
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A"), 0o600))
	deadline := time.After(10 * time.Second)
	for {
		select {
		case change := <-changes:
			if change.URI == "file:///a.md" {
				assert.Contains(t, []eventschema.EventAction{eventschema.EventCreate, eventschema.EventUpdate}, change.Action)
				cancel()
				require.NoError(t, <-done)
				return
			}
		case <-deadline:
			t.Fatal("not told the file was created")
		}
	}
}

func Test_Server(t *testing.T) {
	// The backend is served, as an MCP server, and is a workspace of its
	// client, in memory
	ctx := t.Context()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "notes"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes", "a.md"), []byte("# A"), 0o600))
	srv, err := filer.NewServer("notes", "My notes", backend(t, dir))
	require.NoError(t, err)
	runCtx, cancel := context.WithCancel(ctx)
	ran := make(chan error, 1)
	go func() { ran <- srv.Run(runCtx, nil) }()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-ran)
	})

	c, err := client.New("memory://notes", client.WithTransport(srv.InMemoryTransport))
	require.NoError(t, err)
	require.NoError(t, c.Connect(ctx))
	t.Cleanup(func() { _ = c.Disconnect() })
	server, _ := c.Status()
	require.NotNil(t, server)
	assert.Equal(t, "notes", server.Name)
	assert.Equal(t, "My notes", server.Title)
	assert.True(t, c.HasExperimental(schema.Capability))

	ws, err := workspace.NewClient(c)
	require.NoError(t, err)
	list, err := ws.List(ctx, schema.ListRequest{})
	require.NoError(t, err)
	assert.Equal(t, []string{"file:///notes/"}, uris(list))
	list, err = ws.List(ctx, schema.ListRequest{Path: "notes"})
	require.NoError(t, err)
	assert.Equal(t, []string{"file:///notes/a.md"}, uris(list))
	resource, err := ws.Read(ctx, "file:///notes/a.md")
	require.NoError(t, err)
	data, err := llm.ReadAll(ctx, resource)
	require.NoError(t, err)
	assert.Equal(t, "# A", string(data))

	_, err = filer.NewServer("not a name", "", backend(t, dir))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
