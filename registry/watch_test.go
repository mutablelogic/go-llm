package registry_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/broadcaster/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	registryschema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// watch watches the registry with a buffered channel, until the test ends,
// and returns the channel
func watch(t *testing.T, r *registry.Registry) chan schema.Event {
	t.Helper()
	ch := make(chan schema.Event, 16)
	ctx, cancel := context.WithCancel(context.Background())
	done, err := r.Watch(ctx, ch)
	require.NoError(t, err)
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-done)
	})
	return ch
}

// events returns the events which have been sent to the channel
func events(ch chan schema.Event) []schema.Event {
	var result []schema.Event
	for {
		select {
		case event := <-ch:
			result = append(result, event)
		default:
			return result
		}
	}
}

// event returns an event of the provider with the name
func event(action schema.EventAction, name string) schema.Event {
	return schema.Event{Action: action, Schema: registry.EventSchema, Table: registry.ProviderTable, Row: name}
}

func Test_Watch(t *testing.T) {
	ctx := context.Background()

	t.Run("Providers", func(t *testing.T) {
		// A provider which is added is created, and one which is removed
		// is deleted
		r, err := registry.New()
		require.NoError(t, err)
		ch := watch(t, r)
		require.NoError(t, r.Add("openai", &provider{name: "openai"}))
		_, err = r.Remove("openai")
		require.NoError(t, err)
		assert.Equal(t, []schema.Event{event(schema.EventCreate, "openai"), event(schema.EventDelete, "openai")}, events(ch))

		// Including when they're created and deleted, as requests are
		ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"version":"0.0.1"}`))
		}))
		t.Cleanup(ollama.Close)
		_, err = r.CreateProvider(ctx, registryschema.Provider{Name: "home", Type: llm.Ollama, ProviderMeta: registryschema.ProviderMeta{URL: ollama.URL}})
		require.NoError(t, err)
		_, err = r.DeleteProvider(ctx, "home")
		require.NoError(t, err)
		assert.Equal(t, []schema.Event{event(schema.EventCreate, "home"), event(schema.EventDelete, "home")}, events(ch))

		// But not when they aren't
		assert.Error(t, r.Add("bad name", &provider{name: "x"}))
		_, err = r.Remove("missing")
		assert.Error(t, err)
		assert.Empty(t, events(ch))
	})

	t.Run("Reachable", func(t *testing.T) {
		// A provider is updated when whether it can be reached changes, but
		// not when it's first pinged, or whether it can be reached doesn't
		openai := &provider{name: "openai"}
		r, err := newRegistry(openai)
		require.NoError(t, err)
		ch := watch(t, r)
		require.NoError(t, r.Ping(ctx))
		require.NoError(t, r.Ping(ctx))
		assert.Empty(t, events(ch))

		openai.err = errors.New("connection refused")
		require.Error(t, r.Ping(ctx))
		require.Error(t, r.Ping(ctx))
		assert.Equal(t, []schema.Event{event(schema.EventUpdate, "openai")}, events(ch))

		openai.err = nil
		require.NoError(t, r.Ping(ctx))
		assert.Equal(t, []schema.Event{event(schema.EventUpdate, "openai")}, events(ch))
	})

	t.Run("Models", func(t *testing.T) {
		// A provider is updated when its models change, but not when they're
		// first listed, or don't change, including their order
		setInterval(t, &registry.ModelCacheInterval, 0)
		openai := &provider{name: "openai", models: []string{"gpt-4o", "o3"}}
		r, err := newRegistry(openai)
		require.NoError(t, err)
		ch := watch(t, r)
		_, err = r.ListModels(ctx)
		require.NoError(t, err)
		openai.models = []string{"o3", "gpt-4o"}
		_, err = r.ListModels(ctx)
		require.NoError(t, err)
		assert.Empty(t, events(ch))

		openai.models = []string{"o3", "gpt-5"}
		_, err = r.ListModels(ctx)
		require.NoError(t, err)
		assert.Equal(t, []schema.Event{event(schema.EventUpdate, "openai")}, events(ch))

		// Or when they can't be listed, and the cached models are used
		openai.err = errors.New("connection refused")
		_, err = r.ListModels(ctx)
		require.NoError(t, err)
		assert.Empty(t, events(ch))
	})

	t.Run("Load", func(t *testing.T) {
		// A provider is updated when a model is loaded with it, as its
		// models are listed again
		ollama := newCapable("ollama", "llama3.2:latest")
		r, err := newRegistry(ollama)
		require.NoError(t, err)
		_, err = r.ListModels(ctx)
		require.NoError(t, err)
		ch := watch(t, r)
		_, err = r.LoadModel(ctx, "phi4:latest", nil)
		require.NoError(t, err)
		ollama.models = append(ollama.models, "phi4:latest")
		_, err = r.ListModels(ctx)
		require.NoError(t, err)
		assert.Equal(t, []schema.Event{event(schema.EventUpdate, "ollama")}, events(ch))
	})

	t.Run("Order", func(t *testing.T) {
		// Events are in the order of the changes, so a provider which is
		// added and removed concurrently is created and deleted in turn
		r, err := registry.New()
		require.NoError(t, err)
		ch := make(chan schema.Event, 1000)
		watchCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		_, err = r.Watch(watchCtx, ch)
		require.NoError(t, err)
		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() {
				for range 100 {
					r.Add("openai", &provider{name: "openai"})
					r.Remove("openai")
				}
			})
		}
		wg.Wait()
		r.Remove("openai")
		received := events(ch)
		require.NotEmpty(t, received)
		for i, event := range received {
			want := schema.EventCreate
			if i%2 == 1 {
				want = schema.EventDelete
			}
			require.Equal(t, want, event.Action, "event %d", i)
		}
	})

	t.Run("Errors", func(t *testing.T) {
		r, err := registry.New()
		require.NoError(t, err)
		_, err = r.Watch(ctx, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)

		// A channel watches once at a time
		ch := watch(t, r)
		_, err = r.Watch(ctx, ch)
		assert.ErrorIs(t, err, llm.ErrConflict)
	})

	t.Run("Full", func(t *testing.T) {
		// A channel which is full misses events, rather than the registry
		// waiting for it
		r, err := registry.New()
		require.NoError(t, err)
		watchCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		_, err = r.Watch(watchCtx, make(chan schema.Event))
		require.NoError(t, err)
		added := make(chan error)
		go func() { added <- r.Add("openai", &provider{name: "openai"}) }()
		select {
		case err := <-added:
			assert.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("adding a provider waited for a watcher")
		}
	})
}
