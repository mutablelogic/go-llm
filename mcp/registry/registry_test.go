package registry_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// connector returns the definition of a connector, with the name and URL
func connector(name, url string) schema.Connector {
	return schema.Connector{Name: name, ConnectorMeta: schema.ConnectorMeta{URL: url}}
}

// names returns the names of the connectors
func names(connectors []schema.Connector) []string {
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
	for _, s := range []schema.Connector{connector("9lives", "https://mcp.example.com"), connector("", "https://mcp.example.com"), connector("local", "stdio://connector"), connector("local", "/usr/bin/connector"), connector("local", "https:///mcp")} {
		_, err = r.CreateConnector(ctx, s)
		assert.ErrorIs(t, err, llm.ErrBadParameter, s.String())
	}

	// A connector is got, and updated, which changes its serial
	got, err := r.GetConnector(ctx, "search")
	require.NoError(t, err)
	assert.Equal(t, created, got)
	updated, err := r.UpdateConnector(ctx, "search", schema.ConnectorMeta{URL: "https://mcp.example.com/v2"})
	require.NoError(t, err)
	assert.Equal(t, "https://mcp.example.com/v2", updated.URL)
	assert.Equal(t, uint64(2), updated.Serial)
	_, err = r.UpdateConnector(ctx, "search", schema.ConnectorMeta{URL: "ftp://mcp.example.com"})
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// And deleted
	deleted, err := r.DeleteConnector(ctx, "search")
	require.NoError(t, err)
	assert.Equal(t, updated, deleted)
	for _, err := range []error{
		func() error { _, err := r.GetConnector(ctx, "search"); return err }(),
		func() error { _, err := r.UpdateConnector(ctx, "search", updated.ConnectorMeta); return err }(),
		func() error { _, err := r.DeleteConnector(ctx, "search"); return err }(),
		func() error { _, err := r.Client(ctx, "search"); return err }(),
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
	_, err = two.UpdateConnector(ctx, "github", schema.ConnectorMeta{URL: "https://mcp.example.com/v2"})
	require.NoError(t, err)
	got, err := one.GetConnector(ctx, "github")
	require.NoError(t, err)
	assert.Equal(t, "https://mcp.example.com/v2", got.URL)

	// And a store is needed
	_, err = registry.New(registry.WithStore(nil))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_Client(t *testing.T) {
	// A connector's client is created once, and again once its definition
	// changes, and isn't connected
	ctx := context.Background()
	r, err := registry.New(registry.WithName("test", "1.0"))
	require.NoError(t, err)
	_, err = r.CreateConnector(ctx, connector("github", "https://mcp.example.com/github"))
	require.NoError(t, err)

	c, err := r.Client(ctx, "github")
	require.NoError(t, err)
	assert.Equal(t, "https://mcp.example.com/github", c.Endpoint())
	again, err := r.Client(ctx, "github")
	require.NoError(t, err)
	assert.Same(t, c, again)

	_, err = r.UpdateConnector(ctx, "github", schema.ConnectorMeta{URL: "https://mcp.example.com/v2"})
	require.NoError(t, err)
	updated, err := r.Client(ctx, "github")
	require.NoError(t, err)
	assert.NotSame(t, c, updated)
	assert.Equal(t, "https://mcp.example.com/v2", updated.Endpoint())

	// And a name is needed
	_, err = registry.New(registry.WithName(" ", ""))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_Events(t *testing.T) {
	// A connector which is created, updated, or deleted is published, with
	// its name, and its serial, unless it was deleted
	ctx := t.Context()
	_, err := registry.New(registry.WithBroadcaster(nil))
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	b := broadcaster.New()
	ch := make(chan eventschema.Event, 16)
	watchCtx, cancel := context.WithCancel(ctx)
	done, err := b.Watch(watchCtx, ch)
	require.NoError(t, err)
	defer func() {
		cancel()
		assert.NoError(t, <-done)
	}()
	r, err := registry.New(registry.WithBroadcaster(b))
	require.NoError(t, err)
	_, err = r.CreateConnector(ctx, connector("github", "https://mcp.example.com/github"))
	require.NoError(t, err)
	_, err = r.CreateConnector(ctx, connector("github", "https://mcp.example.com/github"))
	require.Error(t, err)
	_, err = r.UpdateConnector(ctx, "github", schema.ConnectorMeta{URL: "https://mcp.example.com/v2"})
	require.NoError(t, err)
	_, err = r.DeleteConnector(ctx, "github")
	require.NoError(t, err)

	var events []eventschema.Event
	for len(ch) > 0 {
		events = append(events, <-ch)
	}
	event := func(action eventschema.EventAction, data string) eventschema.Event {
		event := eventschema.Event{Action: action, Schema: schema.Name, Table: schema.ConnectorTable, Row: "github"}
		if data != "" {
			event.Data = json.RawMessage(data)
		}
		return event
	}
	assert.Equal(t, []eventschema.Event{
		event(eventschema.EventCreate, `{"serial":1}`),
		event(eventschema.EventUpdate, `{"serial":2}`),
		event(eventschema.EventDelete, ""),
	}, events, "and nothing for a change which failed")
}
