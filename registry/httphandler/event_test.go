package httphandler_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// streamed is an event of a stream, by its name, with its data
type streamed struct {
	name string
	data string
}

// watch requests the registry's events, with the accepted type, and returns
// the response, and the events which are streamed, which is closed when the
// stream ends
func watch(t *testing.T, server *test.Server, accept string) (*http.Response, <-chan streamed) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+test.Prefix+"/llm/event", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", accept)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	events := make(chan streamed, 16)
	go func() {
		defer close(events)
		var name string
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if value, ok := strings.CutPrefix(line, "event: "); ok {
				name = value
			} else if value, ok := strings.CutPrefix(line, "data: "); ok && name != "" {
				events <- streamed{name, value}
				name = ""
			}
		}
	}()
	return resp, events
}

// next returns the next event of the stream, or fails when there's none
func next(t *testing.T, events <-chan streamed) streamed {
	t.Helper()
	select {
	case event, ok := <-events:
		require.True(t, ok, "the stream ended")
		return event
	case <-time.After(5 * time.Second):
		t.Fatal("no event")
		return streamed{}
	}
}

// providerEvent returns the event of the stream as a provider's event
func providerEvent(t *testing.T, event streamed) providerschema.Event {
	t.Helper()
	require.Equal(t, schema.EventName, event.name)
	var result providerschema.Event
	require.NoError(t, json.Unmarshal([]byte(event.data), &result))
	return result
}

func Test_WatchEvents(t *testing.T) {
	r := test.NewRegistry(t, nil)
	server := test.NewServer(t, r)

	t.Run("Stream", func(t *testing.T) {
		// The registry's events are streamed, from when the stream starts
		resp, events := watch(t, server, types.ContentTypeTextStream)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, types.ContentTypeTextStream, resp.Header.Get(types.ContentTypeHeader))
		client, err := eliza.New()
		require.NoError(t, err)
		require.NoError(t, r.Add("chat", client))
		_, err = r.Remove("chat")
		require.NoError(t, err)
		assert.Equal(t, providerschema.Event{Action: providerschema.EventCreate, Table: registry.ProviderTable, Row: "chat"}, providerEvent(t, next(t, events)))
		assert.Equal(t, providerschema.Event{Action: providerschema.EventDelete, Table: registry.ProviderTable, Row: "chat"}, providerEvent(t, next(t, events)))

		// Including when a provider is created and deleted with the API
		do(t, server, http.MethodPost, "llm/provider", map[string]string{"provider": llm.Eliza}, nil)
		do(t, server, http.MethodDelete, "llm/provider/eliza", nil, nil)
		assert.Equal(t, providerschema.EventCreate, providerEvent(t, next(t, events)).Action)
		assert.Equal(t, providerschema.EventDelete, providerEvent(t, next(t, events)).Action)
	})

	t.Run("Any", func(t *testing.T) {
		// A client which accepts any type is streamed to
		resp, _ := watch(t, server, types.ContentTypeAny)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("NotAcceptable", func(t *testing.T) {
		resp, _ := watch(t, server, types.ContentTypeJSON)
		assert.Equal(t, http.StatusNotAcceptable, resp.StatusCode)
	})
}

func Test_WatchEventsShutdown(t *testing.T) {
	// When the registry stops running, as the server is stopping, a
	// shutdown event is sent, and the stream ends
	r := test.NewRegistry(t, nil)
	server := test.NewServer(t, r)
	ctx, stop := context.WithCancel(context.Background())
	running := make(chan error)
	go func() { running <- r.Run(ctx) }()
	require.Eventually(t, func() bool {
		resp, _ := watch(t, server, types.ContentTypeTextStream)
		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 10*time.Millisecond)

	resp, events := watch(t, server, types.ContentTypeTextStream)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	stop()
	require.NoError(t, <-running)
	assert.Equal(t, schema.EventShutdown, next(t, events).name)
	select {
	case _, ok := <-events:
		assert.False(t, ok, "the stream didn't end")
	case <-time.After(5 * time.Second):
		t.Fatal("the stream didn't end")
	}

	// And it isn't streamed to again, until it runs again
	resp, _ = watch(t, server, types.ContentTypeTextStream)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.True(t, resp.Close, "the connection isn't closed")
}
