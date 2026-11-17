package workspace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// empty is a workspace with no resources
type empty struct{}

func (empty) List(context.Context, schema.ListRequest) (*schema.ListResponse, error) {
	return &schema.ListResponse{}, nil
}

func Test_Registry(t *testing.T) {
	r, err := workspace.New()
	require.NoError(t, err)

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
	_, err = r.Remove("notes")
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
	r, err := workspace.New()
	require.NoError(t, err)
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
	r, err := workspace.New()
	require.NoError(t, err)
	ws := &sometimes{}
	require.NoError(t, r.Add("notes", ws))
	assert.Empty(t, slices.Collect(maps.Keys(maps.Collect(r.All()))))
	assert.ErrorIs(t, r.With("notes", func(workspace.Workspace) error { return nil }), llm.ErrNotFound)
	_, err = r.Resolve(ctx, "file:///note.md")
	assert.ErrorIs(t, err, llm.ErrNotFound)

	ws.available = true
	require.NoError(t, r.With("notes", func(workspace.Workspace) error { return nil }))
	resource, err := r.Resolve(ctx, "file:///note.md")
	require.NoError(t, err)
	assert.Equal(t, "text/markdown", resource.Type())
	assert.Equal(t, "resource://notes/file:///note.md", resource.URI(), "the workspace's URI")
	resource, err = r.Resolve(ctx, "resource://notes/file:///note.md")
	require.NoError(t, err)
	data, err := llm.ReadAll(ctx, resource)
	require.NoError(t, err)
	assert.Equal(t, "# Note", string(data), "read by the workspace's URI")
	_, err = r.Read(ctx, "notes", "resource://other/file:///note.md")
	assert.ErrorIs(t, err, llm.ErrNotFound, "another workspace's")
	_, err = r.Resolve(ctx, "file:///other.md")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

// changing is a workspace whose changes are said
type changing struct {
	empty
	changes chan schema.Change
}

func (c changing) Watch(ctx context.Context, ch chan<- schema.Change) error {
	// It's watching, which is said first
	select {
	case ch <- schema.Change{Action: eventschema.EventUpdate}:
	case <-ctx.Done():
		return nil
	}
	for {
		select {
		case change := <-c.changes:
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

func Test_Events(t *testing.T) {
	// A workspace which is added, or removed, and a watched workspace's
	// changes, are published
	ctx := context.Background()
	b := broadcaster.New()
	events := make(chan eventschema.Event, 16)
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	_, err := b.Watch(watchCtx, events)
	require.NoError(t, err)
	r, err := workspace.New(workspace.WithBroadcaster(b))
	require.NoError(t, err)
	defer r.Close()
	next := func() eventschema.Event {
		t.Helper()
		select {
		case event := <-events:
			return event
		case <-time.After(5 * time.Second):
			t.Fatal("no event")
			return eventschema.Event{}
		}
	}

	ws := changing{changes: make(chan schema.Change)}
	require.NoError(t, r.Add("notes", ws))
	assert.Equal(t, eventschema.Event{Action: eventschema.EventCreate, Schema: workspace.EventSchema, Table: workspace.WorkspaceTable, Row: "notes"}, next())

	ws.changes <- schema.Change{Action: eventschema.EventUpdate, URI: "file:///a.md"}
	assert.Equal(t, eventschema.Event{Action: eventschema.EventUpdate, Schema: workspace.EventSchema, Table: workspace.ResourceTable, Row: "notes", Data: json.RawMessage(`{"uri":"resource://notes/file:///a.md"}`)}, next(), "with the workspace's URI")
	ws.changes <- schema.Change{Action: eventschema.EventUpdate}
	assert.Equal(t, eventschema.Event{Action: eventschema.EventUpdate, Schema: workspace.EventSchema, Table: workspace.ResourceTable, Row: "notes", Data: json.RawMessage(`{}`)}, next())

	_, err = r.Remove("notes")
	require.NoError(t, err)
	assert.Equal(t, eventschema.Event{Action: eventschema.EventDelete, Schema: workspace.EventSchema, Table: workspace.WorkspaceTable, Row: "notes"}, next())

	_, err = workspace.New(workspace.WithBroadcaster(nil))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_RemoveIf(t *testing.T) {
	// A workspace is removed only when it's the one with the name
	r, err := workspace.New()
	require.NoError(t, err)
	first, second := &sometimes{available: true}, &sometimes{available: true}
	require.NoError(t, r.Add("notes", first))
	assert.False(t, r.RemoveIf("notes", second), "another workspace")
	assert.True(t, r.RemoveIf("notes", first))
	assert.False(t, r.RemoveIf("notes", first), "once it's removed")
}

func Test_EventsReady(t *testing.T) {
	// Once Add returns, the workspace is watched, so a change it says then
	// is published, and one it said before its watch ended is, too
	b := broadcaster.New()
	events := make(chan eventschema.Event, 16)
	watchCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := b.Watch(watchCtx, events)
	require.NoError(t, err)
	r, err := workspace.New(workspace.WithBroadcaster(b))
	require.NoError(t, err)
	defer r.Close()
	ws := ending{changes: make(chan schema.Change, 1)}
	require.NoError(t, r.Add("notes", ws))
	assert.Equal(t, workspace.WorkspaceTable, (<-events).Table)
	ws.changes <- schema.Change{Action: eventschema.EventUpdate, URI: "file:///a.md"}
	select {
	case event := <-events:
		assert.Equal(t, workspace.ResourceTable, event.Table)
		assert.JSONEq(t, `{"uri":"resource://notes/file:///a.md"}`, string(event.Data))
	case <-time.After(5 * time.Second):
		t.Fatal("the change wasn't published")
	}
}

// ending is a workspace which says it's watching, and then one change, and
// ends its watch at once, with the change still queued
type ending struct {
	empty
	changes chan schema.Change
}

func (e ending) Watch(ctx context.Context, ch chan<- schema.Change) error {
	ch <- schema.Change{Action: eventschema.EventUpdate}
	select {
	case change := <-e.changes:
		ch <- change
		return llm.ErrServiceUnavailable.With("it ended")
	case <-ctx.Done():
		return nil
	}
}

func Test_Run(t *testing.T) {
	// Run logs the workspaces which are added, and removed, as they're
	// published, until it's cancelled
	var log safeBuffer
	logger := slog.New(slog.NewTextHandler(&log, nil))
	b := broadcaster.New()
	r, err := workspace.New(workspace.WithBroadcaster(b))
	require.NoError(t, err)
	defer r.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	assert.False(t, r.Running())
	go func() { done <- r.Run(ctx, logger) }()
	require.Eventually(t, r.Running, 5*time.Second, time.Millisecond)
	assert.ErrorIs(t, r.Run(context.Background(), logger), llm.ErrConflict, "it's running")

	// Once it's watching, which it is, soon after it's running
	require.Eventually(t, func() bool {
		require.NoError(t, r.Add("notes", empty{}))
		_, err := r.Remove("notes")
		require.NoError(t, err)
		return strings.Contains(log.String(), `msg="workspace removed" workspace=notes`)
	}, 5*time.Second, 10*time.Millisecond)
	assert.Contains(t, log.String(), `msg="workspace added" workspace=notes`)

	cancel()
	require.NoError(t, <-done)
	assert.False(t, r.Running())
}

// safeBuffer is a buffer which is written, and read, concurrently
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func Test_Spans(t *testing.T) {
	// The registry's operations are traced, with their arguments
	ctx := context.Background()
	recorder := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")
	r, err := workspace.New(workspace.WithTracer(tracer))
	require.NoError(t, err)
	require.NoError(t, r.Add("notes", readable{}))

	attrs := func(name string) map[string]string {
		t.Helper()
		for _, span := range recorder.Ended() {
			if span.Name() == name {
				result := map[string]string{}
				for _, kv := range span.Attributes() {
					result[string(kv.Key)] = kv.Value.Emit()
				}
				return result
			}
		}
		t.Fatalf("no span %q", name)
		return nil
	}

	r.ListWorkspaces(ctx)
	attrs("llm.workspace.ListWorkspaces")
	_, err = r.GetWorkspace(ctx, "notes")
	require.NoError(t, err)
	assert.Equal(t, "notes", attrs("llm.workspace.GetWorkspace")["arg.name"])
	_, err = r.ListResources(ctx, "notes", schema.ListRequest{Path: "2026"})
	require.NoError(t, err)
	assert.Equal(t, "notes", attrs("llm.workspace.ListResources")["arg.name"])
	assert.Contains(t, attrs("llm.workspace.ListResources")["arg.req"], "2026")
	_, err = r.Resolve(ctx, "resource://notes/file:///note.md")
	require.NoError(t, err)
	assert.Equal(t, "resource://notes/file:///note.md", attrs("llm.workspace.Resolve")["arg.uri"])
	assert.Equal(t, "file:///note.md", attrs("llm.workspace.Read")["arg.uri"])
}

func Test_Watched(t *testing.T) {
	// A workspace is said to be watched only while the registry watches it:
	// with a broadcaster, until its watch ends
	watched := func(r *workspace.Registry, name string) bool {
		t.Helper()
		ws, err := r.GetWorkspace(context.Background(), name)
		require.NoError(t, err)
		return ws.Watch
	}

	// Without a broadcaster, a Watcher isn't watched
	r, err := workspace.New()
	require.NoError(t, err)
	require.NoError(t, r.Add("notes", changing{changes: make(chan schema.Change)}))
	assert.False(t, watched(r, "notes"))

	// With one, it is, until its watch ends, such as when it fails
	r, err = workspace.New(workspace.WithBroadcaster(broadcaster.New()))
	require.NoError(t, err)
	defer r.Close()
	require.NoError(t, r.Add("notes", changing{changes: make(chan schema.Change)}))
	assert.True(t, watched(r, "notes"))
	assert.True(t, r.ListWorkspaces(context.Background()).Body[0].Watch)
	ending := ending{changes: make(chan schema.Change, 1)}
	require.NoError(t, r.Add("ending", ending))
	ending.changes <- schema.Change{Action: eventschema.EventUpdate, URI: "file:///a.md"}
	require.Eventually(t, func() bool { return !watched(r, "ending") }, 5*time.Second, time.Millisecond)
	assert.True(t, watched(r, "notes"))
	require.NoError(t, r.Close())
	require.Eventually(t, func() bool { return !watched(r, "notes") }, 5*time.Second, time.Millisecond, "once it's closed")
}

// described is a workspace which says what it is
type described struct{ empty }

func (described) Title() string       { return "Notes" }
func (described) Description() string { return "My notes" }
func (described) Version() string     { return "1.0" }

func Test_Describer(t *testing.T) {
	// A workspace which says what it is has its title, description and
	// version, and one which doesn't, none
	ctx := context.Background()
	r, err := workspace.New()
	require.NoError(t, err)
	require.NoError(t, r.Add("notes", described{}))
	require.NoError(t, r.Add("other", empty{}))
	ws, err := r.GetWorkspace(ctx, "notes")
	require.NoError(t, err)
	assert.Equal(t, schema.Workspace{Name: "notes", Title: "Notes", Description: "My notes", Version: "1.0"}, *ws)
	ws, err = r.GetWorkspace(ctx, "other")
	require.NoError(t, err)
	assert.Equal(t, schema.Workspace{Name: "other"}, *ws)
}
