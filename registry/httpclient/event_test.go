package httpclient_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// newEventClient returns a client of a test server of a registry without
// providers, and the registry
func newEventClient(t *testing.T) (*httpclient.Client, *registry.Registry) {
	t.Helper()
	r := test.NewRegistry(t, nil)
	server := test.NewServer(t, r)
	client, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)
	return client, r
}

// watchEvents watches the registry's events with the client, until the
// returned function is called, which returns what WatchEvents returned
func watchEvents(client *httpclient.Client, fn func(providerschema.Event) error) (<-chan providerschema.Event, func() error) {
	events := make(chan providerschema.Event, 16)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- client.WatchEvents(ctx, func(event providerschema.Event) error {
			events <- event
			if fn != nil {
				return fn(event)
			}
			return nil
		})
	}()
	return events, func() error {
		cancel()
		return <-done
	}
}

// change creates and deletes a provider with the client until an event is
// received, as the stream starts after the client connects, and returns the
// event
func change(t *testing.T, client *httpclient.Client, events <-chan providerschema.Event) providerschema.Event {
	t.Helper()
	var event providerschema.Event
	require.Eventually(t, func() bool {
		_, err := client.CreateProvider(t.Context(), schema.Provider{Name: "chat", Type: llm.Eliza})
		require.NoError(t, err)
		_, err = client.DeleteProvider(t.Context(), "chat")
		require.NoError(t, err)
		select {
		case event = <-events:
			return true
		case <-time.After(100 * time.Millisecond):
			return false
		}
	}, 10*time.Second, 10*time.Millisecond)
	return event
}

func Test_WatchEvents(t *testing.T) {
	client, _ := newEventClient(t)

	t.Run("Events", func(t *testing.T) {
		// The events are received, as providers are created and deleted,
		// and cancelling the context isn't an error
		events, stop := watchEvents(client, nil)
		assert.Equal(t, providerschema.Event{Action: providerschema.EventCreate, Table: registry.ProviderTable, Row: "chat"}, change(t, client, events))
		select {
		case event := <-events:
			assert.Equal(t, providerschema.Event{Action: providerschema.EventDelete, Table: registry.ProviderTable, Row: "chat"}, event)
		case <-time.After(5 * time.Second):
			t.Fatal("no delete event")
		}
		assert.NoError(t, stop())
	})

	t.Run("Error", func(t *testing.T) {
		// An error from the function stops watching, and is returned
		errStop := errors.New("stop")
		events, stop := watchEvents(client, func(providerschema.Event) error { return errStop })
		change(t, client, events)
		assert.ErrorIs(t, stop(), errStop)
	})

	t.Run("NoFunction", func(t *testing.T) {
		assert.ErrorIs(t, client.WatchEvents(t.Context(), nil), llm.ErrBadParameter)
	})
}

func Test_WatchEventsShutdown(t *testing.T) {
	// When the server's registry stops running, as it's stopping, watching
	// stops, with ErrServiceUnavailable, as there will be no more events
	client, r := newEventClient(t)
	ctx, stop := context.WithCancel(context.Background())
	running := make(chan error, 1)
	go func() { running <- r.Run(ctx, nil) }()
	events := make(chan providerschema.Event, 16)
	done := make(chan error, 1)
	go func() {
		done <- client.WatchEvents(context.Background(), func(event providerschema.Event) error {
			events <- event
			return nil
		})
	}()
	change(t, client, events)
	stop()
	require.NoError(t, <-running)
	select {
	case err := <-done:
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	case <-time.After(5 * time.Second):
		t.Fatal("watching didn't stop")
	}

	// And it can't watch again, until the registry runs again
	err := client.WatchEvents(context.Background(), func(providerschema.Event) error { return nil })
	assert.Equal(t, http.StatusServiceUnavailable, status(t, err))
}
