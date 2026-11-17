package workspace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"slices"
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
	assert.Equal(t, eventschema.Event{Action: eventschema.EventUpdate, Schema: workspace.EventSchema, Table: workspace.ResourceTable, Row: "notes", Data: json.RawMessage(`{"uri":"file:///a.md"}`)}, next())
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
		assert.JSONEq(t, `{"uri":"file:///a.md"}`, string(event.Data))
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

// running is a workspace which says when it runs, and stops
type running struct {
	empty
	started, stopped chan string
}

func (w running) Run(ctx context.Context, logger *slog.Logger) error {
	w.started <- "run"
	<-ctx.Done()
	w.stopped <- "stop"
	return nil
}

func Test_Run(t *testing.T) {
	// A Runner runs while the registry does, from when it's added, until
	// it's removed, and a workspace which is added, or removed, is logged
	var log safeBuffer
	logger := slog.New(slog.NewTextHandler(&log, nil))
	r, err := workspace.New()
	require.NoError(t, err)
	before := running{started: make(chan string, 1), stopped: make(chan string, 1)}
	require.NoError(t, r.Add("before", before))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx, logger) }()
	assert.Equal(t, "run", <-before.started, "one added before it runs")

	after := running{started: make(chan string, 1), stopped: make(chan string, 1)}
	require.NoError(t, r.Add("after", after))
	assert.Equal(t, "run", <-after.started, "one added as it runs")
	_, err = r.Remove("after")
	require.NoError(t, err)
	assert.Equal(t, "stop", <-after.stopped, "once it's removed")
	assert.ErrorIs(t, r.Run(context.Background(), logger), llm.ErrConflict, "it's running")

	cancel()
	require.NoError(t, <-done)
	assert.Equal(t, "stop", <-before.stopped, "once it stops")
	assert.Contains(t, log.String(), `msg="workspace added" workspace=before`)
	assert.Contains(t, log.String(), `msg="workspace added" workspace=after`)
	assert.Contains(t, log.String(), `msg="workspace removed" workspace=after`)
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
