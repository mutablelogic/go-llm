package registry_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	test "github.com/mutablelogic/go-llm/mcp/test"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// connector returns the definition of a connector, with the name and URL
func connector(name, url string) schema.Connector {
	return schema.Connector{Name: name, ConnectorMeta: schema.ConnectorMeta{URL: url}}
}

// names returns the names of the connectors
func names(connectors []*schema.Connector) []string {
	result := make([]string, 0, len(connectors))
	for _, s := range connectors {
		result = append(result, s.Name)
	}
	return result
}

func Test_MemoryStore(t *testing.T) {
	ctx := context.Background()
	store := registry.NewMemoryStore()

	// A connector is created with serial zero, and its serial is one
	created, err := store.Put(ctx, connector("github", "https://mcp.example.com/github"))
	require.NoError(t, err)
	assert.Equal(t, uint64(1), created.Serial)
	_, err = store.Put(ctx, connector("github", "https://mcp.example.com/other"))
	assert.ErrorIs(t, err, llm.ErrConflict, "it exists")

	// It's replaced with its serial, which changes, and not another
	created.URL = "https://mcp.example.com/v2"
	updated, err := store.Put(ctx, created)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), updated.Serial)
	_, err = store.Put(ctx, created)
	assert.ErrorIs(t, err, llm.ErrConflict, "it was changed since")

	// It's deleted with its serial, and not another
	assert.ErrorIs(t, store.Delete(ctx, "github", 1), llm.ErrConflict)
	require.NoError(t, store.Delete(ctx, "github", 2))
	require.NoError(t, store.Delete(ctx, "github", 2), "a connector which isn't there")
	connectors, err := store.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, connectors)
	_, err = store.Put(ctx, updated)
	assert.ErrorIs(t, err, llm.ErrConflict, "it was deleted")
}

func Test_MemoryStoreSerials(t *testing.T) {
	// A serial isn't used again for a name, once its connector is deleted,
	// so a change to one isn't made to another, created with its name
	ctx := context.Background()
	store := registry.NewMemoryStore()
	first, err := store.Put(ctx, connector("search", "https://mcp.example.com/one"))
	require.NoError(t, err)
	require.NoError(t, store.Delete(ctx, "search", first.Serial))
	second, err := store.Put(ctx, connector("search", "https://mcp.example.com/two"))
	require.NoError(t, err)
	assert.Equal(t, uint64(2), second.Serial)
	_, err = store.Put(ctx, first)
	assert.ErrorIs(t, err, llm.ErrConflict, "the first, as it was")
	assert.ErrorIs(t, store.Delete(ctx, "search", first.Serial), llm.ErrConflict)

	// And its patterns are its own, not those it was given, or returns
	patterns := connector("patterns", "https://mcp.example.com")
	patterns.Include = []string{"^a"}
	put, err := store.Put(ctx, patterns)
	require.NoError(t, err)
	patterns.Include[0], put.Include[0] = "changed", "changed"
	list, err := store.List(ctx)
	require.NoError(t, err)
	for _, c := range list {
		if c.Name == "patterns" {
			assert.Equal(t, []string{"^a"}, c.Include)
			c.Include[0] = "changed"
		}
	}
	list, err = store.List(ctx)
	require.NoError(t, err)
	for _, c := range list {
		if c.Name == "patterns" {
			assert.Equal(t, []string{"^a"}, c.Include)
		}
	}
}

func Test_Servers(t *testing.T) {
	ctx := context.Background()
	r, err := registry.New()
	require.NoError(t, err)

	// Connectors are created, and listed, in order of their names
	created, err := r.CreateConnector(ctx, connector("search", "https://mcp.example.com/search"))
	require.NoError(t, err)
	assert.Equal(t, uint64(1), created.Serial)
	_, err = r.CreateConnector(ctx, schema.Connector{Name: "github", ConnectorMeta: schema.ConnectorMeta{URL: "https://mcp.example.com/github"}, Serial: 7})
	require.NoError(t, err, "whatever its serial")
	list, err := r.ListConnectors(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint(2), list.Count)
	assert.Equal(t, []string{"github", "search"}, names(list.Body))

	// But not twice, nor with a name which isn't an identifier, or a URL
	// which isn't a remote connector's
	_, err = r.CreateConnector(ctx, connector("search", "https://mcp.example.com/other"))
	assert.ErrorIs(t, err, llm.ErrConflict)
	for _, s := range []schema.Connector{connector("9lives", "https://mcp.example.com"), connector("", "https://mcp.example.com"), connector("local", "stdio://connector"), connector("local", "/usr/bin/connector"), connector("local", "https:///mcp"), connector("local", "https://:443/mcp")} {
		_, err = r.CreateConnector(ctx, s)
		assert.ErrorIs(t, err, llm.ErrBadParameter, s.String())
	}

	// A connector is got, and updated, once its server, as it is now, is
	// connected, which changes its serial, and it's connected
	got, err := r.GetConnector(ctx, "search")
	require.NoError(t, err)
	assert.Equal(t, created, got)
	server := test.NewServer(t)
	updated, err := r.UpdateConnector(ctx, "search", schema.ConnectorMeta{URL: server.URL})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, r.Close()) })
	assert.Equal(t, server.URL, updated.URL)
	assert.Equal(t, uint64(2), updated.Serial)
	assert.True(t, updated.Connected)
	_, err = r.UpdateConnector(ctx, "search", schema.ConnectorMeta{URL: "ftp://mcp.example.com"})
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// But not with a server which can't be connected, when it's left as it
	// was, and connected
	notFound := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(notFound.Close)
	_, err = r.UpdateConnector(ctx, "search", schema.ConnectorMeta{URL: notFound.URL})
	assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	got, err = r.GetConnector(ctx, "search")
	require.NoError(t, err)
	assert.Equal(t, updated, got)

	// And deleted
	deleted, err := r.DeleteConnector(ctx, "search")
	require.NoError(t, err)
	assert.Equal(t, updated.Definition(), *deleted, "and disconnected")
	for _, err := range []error{
		func() error { _, err := r.GetConnector(ctx, "search"); return err }(),
		func() error { _, err := r.UpdateConnector(ctx, "search", updated.ConnectorMeta); return err }(),
		func() error { _, err := r.DeleteConnector(ctx, "search"); return err }(),
	} {
		assert.ErrorIs(t, err, llm.ErrNotFound)
	}
}

func Test_Store(t *testing.T) {
	// The connectors are kept in the registry's store, so another registry
	// with the store, such as once the process restarts, has them, and its
	// changes are seen
	ctx := context.Background()
	store := registry.NewMemoryStore()
	one, err := registry.New(registry.WithStore(store))
	require.NoError(t, err)
	_, err = one.CreateConnector(ctx, connector("github", "https://mcp.example.com/github"))
	require.NoError(t, err)

	two, err := registry.New(registry.WithStore(store))
	require.NoError(t, err)
	list, err := two.ListConnectors(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"github"}, names(list.Body))
	server := test.NewServer(t)
	_, err = two.UpdateConnector(ctx, "github", schema.ConnectorMeta{URL: server.URL})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, two.Close()) })
	got, err := one.GetConnector(ctx, "github")
	require.NoError(t, err)
	assert.Equal(t, server.URL, got.URL)

	// Which has the connectors' definitions, without their status, which a
	// request can't set
	status := connector("status", "https://mcp.example.com/status")
	status.Connected, status.Server, status.Error = true, &schema.ConnectorServer{Name: "fake"}, "fake"
	created, err := one.CreateConnector(ctx, status)
	require.NoError(t, err)
	assert.False(t, created.Connected)
	assert.Nil(t, created.Server)
	assert.Empty(t, created.Error)
	stored, err := store.List(ctx)
	require.NoError(t, err)
	for _, c := range stored {
		assert.Equal(t, c.Definition(), c, c.Name)
	}

	// And a store is needed
	_, err = registry.New(registry.WithStore(nil))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

// watch returns the events which the broadcaster publishes, as they're
// published, until the test ends
func watch(t *testing.T, b *broadcaster.Broadcaster) chan eventschema.Event {
	t.Helper()
	ch := make(chan eventschema.Event, 16)
	ctx, cancel := context.WithCancel(context.Background())
	done, err := b.Watch(ctx, ch)
	require.NoError(t, err)
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-done)
	})
	return ch
}

// published returns the events which were published, until none are, for a
// while
func published(ch chan eventschema.Event) []eventschema.Event {
	var events []eventschema.Event
	for {
		select {
		case event := <-ch:
			events = append(events, event)
		case <-time.After(100 * time.Millisecond):
			return events
		}
	}
}

func Test_Events(t *testing.T) {
	// The broadcaster which the registry is given is told when a connector's
	// server is connected, as a connector which is created, with its server,
	// and disconnected, as one which is deleted, with why, when it wasn't
	// disconnected, but not when a connector is created, or updated, or
	// can't be connected
	ctx := t.Context()
	_, err := registry.New(registry.WithBroadcaster(nil))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	b := broadcaster.New()
	ch := watch(t, b)
	r, err := registry.New(registry.WithBroadcaster(b))
	require.NoError(t, err)
	event := func(action eventschema.EventAction, data string) eventschema.Event {
		return eventschema.Event{Action: action, Schema: schema.Name, Table: schema.ConnectorTable, Row: "github", Data: json.RawMessage(data)}
	}
	connected := `{"serial":2,"server":{"name":"` + test.Name + `","version":"` + test.Version + `"}}`

	// Created, and not connected
	_, err = r.CreateConnector(ctx, connector("github", "https://mcp.example.com/github"))
	require.NoError(t, err)
	assert.Empty(t, published(ch))

	// Updated, and connected, and not updated, as its server can't be
	// connected
	server := test.NewServer(t)
	_, err = r.UpdateConnector(ctx, "github", schema.ConnectorMeta{URL: server.URL})
	require.NoError(t, err)
	_, err = r.UpdateConnector(ctx, "github", schema.ConnectorMeta{URL: "http://localhost:1/mcp"})
	require.Error(t, err)
	assert.Equal(t, []eventschema.Event{event(eventschema.EventCreate, connected)}, published(ch))

	// Updated again, which is said as it's connected, and not as the one it
	// replaces is disconnected
	_, err = r.UpdateConnector(ctx, "github", schema.ConnectorMeta{URL: server.URL + "/"})
	require.NoError(t, err)
	again := `{"serial":3,"server":{"name":"` + test.Name + `","version":"` + test.Version + `"}}`
	assert.Equal(t, []eventschema.Event{event(eventschema.EventCreate, again)}, published(ch))

	// Deleted, and disconnected
	_, err = r.DeleteConnector(ctx, "github")
	require.NoError(t, err)
	assert.Equal(t, []eventschema.Event{event(eventschema.EventDelete, `{"serial":3}`)}, published(ch))
}

func Test_EventsClosed(t *testing.T) {
	// A server which closes the session is said to be disconnected, with
	// why, as is every server, once the registry is closed
	ctx := t.Context()
	b := broadcaster.New()
	ch := watch(t, b)
	r, err := registry.New(registry.WithBroadcaster(b))
	require.NoError(t, err)
	server := test.NewServer(t)
	_, err = r.CreateConnector(ctx, connector("one", "https://mcp.example.com/one"))
	require.NoError(t, err)
	_, err = r.UpdateConnector(ctx, "one", schema.ConnectorMeta{URL: server.URL})
	require.NoError(t, err)
	other := test.NewServer(t, test.WithSSE())
	_, err = r.CreateConnector(ctx, connector("two", "https://mcp.example.com/two"))
	require.NoError(t, err)
	_, err = r.UpdateConnector(ctx, "two", schema.ConnectorMeta{URL: other.URL})
	require.NoError(t, err)
	require.Len(t, published(ch), 2)

	// The server of one closes its session
	other.CloseClientConnections()
	events := published(ch)
	require.Len(t, events, 1)
	assert.Equal(t, eventschema.EventDelete, events[0].Action)
	assert.Equal(t, "two", events[0].Row)
	var data schema.ConnectorEvent
	require.NoError(t, json.Unmarshal(events[0].Data, &data))
	assert.NotEmpty(t, data.Error)

	// And the other is disconnected, as the registry is closed
	require.NoError(t, r.Close())
	events = published(ch)
	require.Len(t, events, 1)
	assert.Equal(t, eventschema.EventDelete, events[0].Action)
	assert.Equal(t, "one", events[0].Row)
}
