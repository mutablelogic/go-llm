package httpclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	httpclient "github.com/mutablelogic/go-llm/broadcaster/httpclient"
	httphandler "github.com/mutablelogic/go-llm/broadcaster/httphandler"
	schema "github.com/mutablelogic/go-llm/broadcaster/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// newClient returns a client of a test server of a broadcaster, and the
// broadcaster
func newClient(t *testing.T) (*httpclient.Client, *broadcaster.Broadcaster) {
	t.Helper()
	b := broadcaster.New()
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(b, router)
	})
	client, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)
	return client, b
}

// status returns the HTTP status of the error
func status(t *testing.T, err error) int {
	t.Helper()
	var response httpresponse.ErrResponse
	require.True(t, errors.As(err, &response), "error is %T: %v", err, err)
	return response.Code
}

// watchEvents watches the events with the client, until the returned
// function is called, which returns what WatchEvents returned
func watchEvents(client *httpclient.Client, fn func(schema.Event) error) (<-chan schema.Event, func() error) {
	events := make(chan schema.Event, 16)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- client.WatchEvents(ctx, func(event schema.Event) error {
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

// publish publishes the event until it's received, as the stream starts
// after the client connects, and returns it
func publish(t *testing.T, b *broadcaster.Broadcaster, events <-chan schema.Event, event schema.Event) schema.Event {
	t.Helper()
	var received schema.Event
	require.Eventually(t, func() bool {
		b.Publish(event)
		select {
		case received = <-events:
			return true
		case <-time.After(100 * time.Millisecond):
			return false
		}
	}, 10*time.Second, 10*time.Millisecond)
	return received
}

func Test_WatchEvents(t *testing.T) {
	client, b := newClient(t)
	created := schema.Event{Action: schema.EventCreate, Table: "provider", Row: "chat"}

	t.Run("Events", func(t *testing.T) {
		// The events are received, with their data, and cancelling the
		// context isn't an error
		events, stop := watchEvents(client, nil)
		assert.Equal(t, created, publish(t, b, events, created))
		updated := schema.Event{Action: schema.EventUpdate, Table: "session", Row: "1", Data: json.RawMessage(`{"serial":2}`)}
		b.Publish(updated)
		select {
		case event := <-events:
			assert.Equal(t, updated, event)
		case <-time.After(5 * time.Second):
			t.Fatal("no update event")
		}
		assert.NoError(t, stop())
	})

	t.Run("Error", func(t *testing.T) {
		// An error from the function stops watching, and is returned
		errStop := errors.New("stop")
		events, stop := watchEvents(client, func(schema.Event) error { return errStop })
		publish(t, b, events, created)
		assert.ErrorIs(t, stop(), errStop)
	})

	t.Run("NoFunction", func(t *testing.T) {
		assert.ErrorIs(t, client.WatchEvents(t.Context(), nil), llm.ErrBadParameter)
	})
}

func Test_WatchEventsShutdown(t *testing.T) {
	// When the server's broadcaster stops running, as it's stopping,
	// watching stops, with ErrServiceUnavailable, as there will be no more
	// events
	client, b := newClient(t)
	ctx, stop := context.WithCancel(context.Background())
	running := make(chan error, 1)
	go func() { running <- b.Run(ctx) }()
	events := make(chan schema.Event, 16)
	done := make(chan error, 1)
	go func() {
		done <- client.WatchEvents(context.Background(), func(event schema.Event) error {
			events <- event
			return nil
		})
	}()
	publish(t, b, events, schema.Event{Action: schema.EventCreate, Table: "provider", Row: "chat"})
	stop()
	require.NoError(t, <-running)
	select {
	case err := <-done:
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	case <-time.After(5 * time.Second):
		t.Fatal("watching didn't stop")
	}

	// And it can't watch again, until the broadcaster runs again
	err := client.WatchEvents(context.Background(), func(schema.Event) error { return nil })
	assert.Equal(t, http.StatusServiceUnavailable, status(t, err))
}
