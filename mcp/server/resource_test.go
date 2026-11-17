package server_test

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
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

// gone is a resource which is found, but whose content isn't
type gone struct{ providerschema.Resource }

func (gone) Read(context.Context) (llm.ReadFunc, error) { return nil, llm.ErrNotFound.With("gone") }

// Read reads a file, by its URI, of which gone.md is found, but its content
// isn't
func (f files) Read(_ context.Context, uri string) (llm.Resource, error) {
	if uri == "file:///gone.md" {
		return gone{}, nil
	}
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
	ws, err := workspace.NewClient(c)
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
		assert.ErrorIs(t, err, llm.ErrNotFound, "MCP's error is ErrNotFound")
		_, err = ws.Read(ctx, "file:///gone.md")
		assert.ErrorIs(t, err, llm.ErrNotFound, "as is one whose content is gone")
	})
}

// watched is a workspace of files, whose changes are said
type watched struct {
	files
	changes chan schema.Change
}

func (w *watched) WatchResources(ctx context.Context, ch chan<- schema.Change) error {
	// It's watching, which is said first
	select {
	case ch <- schema.Change{Action: eventschema.EventUpdate}:
	case <-ctx.Done():
		return nil
	}
	for {
		select {
		case change := <-w.changes:
			select {
			case ch <- change:
			case <-ctx.Done():
				return nil
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func Test_WorkspaceWatch(t *testing.T) {
	// A client is told a resource it subscribed to is updated, and that the
	// list changed, when one's created
	ctx := t.Context()
	source := &watched{files: files{"notes/a.md": "# A"}, changes: make(chan schema.Change)}
	srv, err := server.New(&tools{}, server.WithWorkspace(source))
	require.NoError(t, err)
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- srv.Run(runCtx) }()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-done)
	})

	c, err := client.New("memory://files", client.WithTransport(srv.InMemoryTransport))
	require.NoError(t, err)
	require.NoError(t, c.Connect(ctx))
	t.Cleanup(func() { assert.NoError(t, c.Disconnect()) })
	ws, err := workspace.NewClient(c)
	require.NoError(t, err)
	watcher, ok := ws.(workspace.Watcher)
	require.True(t, ok)
	changes := make(chan schema.Change, 4)
	watchCtx, stop := context.WithCancel(ctx)
	defer stop()
	go func() { _ = watcher.WatchResources(watchCtx, changes) }()
	select {
	case change := <-changes:
		assert.Equal(t, schema.Change{Action: eventschema.EventUpdate}, change, "it's watching")
	case <-time.After(5 * time.Second):
		t.Fatal("the watch didn't start")
	}
	require.NoError(t, c.Subscribe(ctx, "file:///notes/a.md"))

	// say says the change until the client is told, as its subscription, and
	// its listening for the list's changes, start as it's connected
	say := func(change schema.Change) schema.Change {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			source.changes <- change
			select {
			case got := <-changes:
				if got.URI == "" && change.Action == eventschema.EventUpdate && change.URI != "" {
					// The list changed, as the server started watching
					continue
				}
				return got
			case <-time.After(50 * time.Millisecond):
			case <-deadline:
				t.Fatal("no change")
				return schema.Change{}
			}
		}
	}

	// Updated, as it was subscribed to
	assert.Equal(t, schema.Change{Action: eventschema.EventUpdate, URI: "file:///notes/a.md"}, say(schema.Change{Action: eventschema.EventUpdate, URI: "file:///notes/a.md"}))

	// Created, which is that the list changed, without a URI, as MCP says
	for len(changes) > 0 {
		<-changes
	}
	assert.Equal(t, schema.Change{Action: eventschema.EventUpdate}, say(schema.Change{Action: eventschema.EventCreate, URI: "file:///notes/b.md"}))

	// The placeholder which says so isn't listed
	list, err := ws.List(ctx, schema.ListRequest{Recursive: true})
	require.NoError(t, err)
	for _, r := range list.Body {
		assert.NotContains(t, r.URI, "list-changed")
	}
}

func Test_WorkspaceSubscribe(t *testing.T) {
	// A workspace of an MCP server, which is served again, is subscribed
	// to, as the second server's clients subscribe, so they're told it's
	// updated
	ctx := t.Context()
	run := func(srv *server.Server) {
		runCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- srv.Run(runCtx) }()
		t.Cleanup(func() {
			cancel()
			assert.NoError(t, <-done)
		})
	}
	connectTo := func(srv *server.Server) *client.Client {
		c, err := client.New("memory://files", client.WithTransport(srv.InMemoryTransport))
		require.NoError(t, err)
		require.NoError(t, c.Connect(ctx))
		t.Cleanup(func() { assert.NoError(t, c.Disconnect()) })
		return c
	}

	// The first server, and the second, which serves the first's workspace
	source := &watched{files: files{"notes/a.md": "# A"}, changes: make(chan schema.Change)}
	first, err := server.New(&tools{}, server.WithWorkspace(source))
	require.NoError(t, err)
	run(first)
	upstream, err := workspace.NewClient(connectTo(first))
	require.NoError(t, err)
	second, err := server.New(&tools{}, server.WithWorkspace(upstream))
	require.NoError(t, err)
	run(second)

	// A client of the second subscribes, and is told of an update
	downstream, err := workspace.NewClient(connectTo(second))
	require.NoError(t, err)
	changes := make(chan schema.Change, 4)
	watchCtx, stop := context.WithCancel(ctx)
	defer stop()
	go func() { _ = downstream.(workspace.Watcher).WatchResources(watchCtx, changes) }()
	require.NoError(t, downstream.(workspace.Subscriber).Subscribe(ctx, "file:///notes/a.md"))
	deadline := time.After(5 * time.Second)
	for {
		source.changes <- schema.Change{Action: eventschema.EventUpdate, URI: "file:///notes/a.md"}
		select {
		case change := <-changes:
			if change.URI == "" {
				continue
			}
			assert.Equal(t, schema.Change{Action: eventschema.EventUpdate, URI: "file:///notes/a.md"}, change)
			return
		case <-time.After(50 * time.Millisecond):
		case <-deadline:
			t.Fatal("the update didn't reach the second server's client")
		}
	}
}
