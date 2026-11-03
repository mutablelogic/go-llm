//go:build !client

package main

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// FAKES

// memDefaults are defaults in memory, which Reload makes as they'd be read
// from disk, as JSON decoded into maps and slices
type memDefaults struct {
	sync.Mutex
	values map[string]any
}

func (d *memDefaults) Get(key string) any {
	d.Lock()
	defer d.Unlock()
	return d.values[key]
}

func (d *memDefaults) Set(key string, value any) error {
	d.Lock()
	defer d.Unlock()
	if d.values == nil {
		d.values = make(map[string]any)
	}
	if value == nil {
		delete(d.values, key)
	} else {
		d.values[key] = value
	}
	return nil
}

// Reload makes the defaults as they'd be when they're read from disk, when
// the command runs again
func (d *memDefaults) Reload(t *testing.T) {
	t.Helper()
	d.Lock()
	defer d.Unlock()
	data, err := json.Marshal(d.values)
	require.NoError(t, err)
	d.values = nil
	require.NoError(t, json.Unmarshal(data, &d.values))
}

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// chat returns the definition of an eliza provider with the name, and serial
func chat(name string, serial uint64) schema.Provider {
	return schema.Provider{Name: name, Type: llm.Eliza, Serial: serial}
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_Store(t *testing.T) {
	ctx := context.Background()
	d := new(memDefaults)
	store, err := newStore(d)
	require.NoError(t, err)

	t.Run("Empty", func(t *testing.T) {
		// Defaults without providers have none
		list, err := store.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, list)
		require.NoError(t, store.Delete(ctx, "missing", 0))
		assert.Nil(t, d.Get("llm.providers"))
	})

	t.Run("Put", func(t *testing.T) {
		// Providers are put in the defaults, in order of their names, with
		// their API keys' placeholders, and serial 1
		openai := schema.Provider{Name: "openai", Type: llm.OpenAI, APIKey: "${OPENAI_KEY}", Include: []string{"^gpt-"}}
		stored, err := store.Put(ctx, openai)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), stored.Serial)
		_, err = store.Put(ctx, chat("chat", 0))
		require.NoError(t, err)
		list, err := store.List(ctx)
		require.NoError(t, err)
		assert.Equal(t, []schema.Provider{chat("chat", 1), stored}, list)

		// And a provider with its name is replaced, as it was seen, with
		// the next serial
		openai = stored
		openai.Exclude = []string{"-mini$"}
		stored, err = store.Put(ctx, openai)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), stored.Serial)
		list, err = store.List(ctx)
		require.NoError(t, err)
		assert.Equal(t, []schema.Provider{chat("chat", 1), stored}, list)

		// But not when it was changed, or created, since it was seen
		_, err = store.Put(ctx, openai)
		assert.ErrorIs(t, err, llm.ErrConflict)
		_, err = store.Put(ctx, chat("chat", 0))
		assert.ErrorIs(t, err, llm.ErrConflict)

		// And one without a name isn't
		_, err = store.Put(ctx, schema.Provider{Type: llm.Eliza})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Reload", func(t *testing.T) {
		// The providers are the same when the defaults are read from disk
		before, err := store.List(ctx)
		require.NoError(t, err)
		d.Reload(t)
		after, err := store.List(ctx)
		require.NoError(t, err)
		assert.Equal(t, before, after)
	})

	t.Run("Delete", func(t *testing.T) {
		// A provider is deleted as it was seen, but not when it was changed
		// since, and the providers are removed from the defaults, when
		// there are none
		assert.ErrorIs(t, store.Delete(ctx, "openai", 1), llm.ErrConflict)
		require.NoError(t, store.Delete(ctx, "openai", 2))
		list, err := store.List(ctx)
		require.NoError(t, err)
		assert.Equal(t, []schema.Provider{chat("chat", 1)}, list)
		require.NoError(t, store.Delete(ctx, "chat", 1))
		assert.Nil(t, d.Get("llm.providers"))
	})

	t.Run("Invalid", func(t *testing.T) {
		// Defaults which aren't providers are an error
		require.NoError(t, d.Set("llm.providers", "not providers"))
		defer d.Set("llm.providers", nil)
		_, err := store.List(ctx)
		assert.Error(t, err)
		_, err = store.Put(ctx, chat("chat", 0))
		assert.Error(t, err)
	})

	t.Run("Watch", func(t *testing.T) {
		// It sends no events, until the context is cancelled
		watchCtx, cancel := context.WithCancel(ctx)
		done, err := store.Watch(watchCtx, make(chan providerschema.Event))
		require.NoError(t, err)
		cancel()
		assert.NoError(t, <-done)
		_, err = store.Watch(ctx, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Errors", func(t *testing.T) {
		_, err := newStore(nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

func Test_StoreRestart(t *testing.T) {
	// A registry which keeps its providers in the defaults has them when the
	// command runs again, as a server's would when it restarts
	ctx := context.Background()
	d := new(memDefaults)
	store, err := newStore(d)
	require.NoError(t, err)

	// run runs a registry with the store, until the returned function is
	// called, which returns what Run returned
	run := func() (*registry.Registry, func() error) {
		r, err := registry.New(registry.WithStore(store))
		require.NoError(t, err)
		runCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- r.Run(runCtx, nil) }()
		t.Cleanup(cancel)
		return r, func() error {
			cancel()
			return <-done
		}
	}

	r, stop := run()
	_, err = r.CreateProvider(ctx, chat("chat", 0))
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		list, err := store.List(ctx)
		return err == nil && len(list) == 1
	}, 5*time.Second, 5*time.Millisecond, "the provider wasn't put in the defaults")
	require.NoError(t, stop())

	// The defaults are read from disk, and a new registry has the provider
	d.Reload(t)
	r, stop = run()
	defer func() { require.NoError(t, stop()) }()
	require.Eventually(t, func() bool {
		provider, err := r.GetProvider(ctx, "chat")
		return err == nil && provider.Type == llm.Eliza && provider.Serial == 1
	}, 5*time.Second, 5*time.Millisecond, "the defaults' provider wasn't added")
}
