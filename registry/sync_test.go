package registry_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	gosync "sync"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// FAKES

// memStore is a store in memory, whose providers have serials, as a
// database's would, which others can change, with Change and Remove, which
// sends the events of, and which counts what it's asked to put and delete
type memStore struct {
	gosync.Mutex
	defs          map[string]schema.Provider
	err           error // returned by List, when it's set
	puts, deletes int
	watcher       chan<- providerschema.Event
	stopped       chan error
}

var _ registry.Store = (*memStore)(nil)

func newStore(defs ...schema.Provider) *memStore {
	s := &memStore{defs: make(map[string]schema.Provider), stopped: make(chan error, 1)}
	for _, def := range defs {
		def.Serial = 1
		s.defs[def.Name] = def
	}
	return s
}

func (s *memStore) List(context.Context) ([]schema.Provider, error) {
	s.Lock()
	defer s.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	var result []schema.Provider
	for _, def := range s.defs {
		result = append(result, def)
	}
	return result, nil
}

func (s *memStore) Put(_ context.Context, def schema.Provider) (schema.Provider, error) {
	s.Lock()
	defer s.Unlock()
	s.puts++
	if def.Serial != s.defs[def.Name].Serial {
		return schema.Provider{}, llm.ErrConflict.Withf("provider %q", def.Name)
	}
	def.Serial++
	s.defs[def.Name] = def
	return def, nil
}

func (s *memStore) Delete(_ context.Context, name string, serial uint64) error {
	s.Lock()
	defer s.Unlock()
	s.deletes++
	if def, exists := s.defs[name]; !exists {
		return nil
	} else if def.Serial != serial {
		return llm.ErrConflict.Withf("provider %q", name)
	}
	delete(s.defs, name)
	return nil
}

func (s *memStore) Watch(ctx context.Context, ch chan<- providerschema.Event) (<-chan error, error) {
	s.Lock()
	s.watcher = ch
	s.Unlock()
	done := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			done <- nil
		case err := <-s.stopped:
			done <- err
		}
	}()
	return done, nil
}

// Change changes the provider in the store, as another server would, with
// its next serial, and sends its event
func (s *memStore) Change(def schema.Provider) {
	s.Lock()
	defer s.Unlock()
	action := providerschema.EventCreate
	if _, exists := s.defs[def.Name]; exists {
		action = providerschema.EventUpdate
	}
	s.change(def)
	s.send(providerschema.Event{Action: action, Table: "provider", Row: def.Name})
}

// Quietly changes the provider in the store, as another server would, with
// its next serial, without its event, which is missed
func (s *memStore) Quietly(def schema.Provider) {
	s.Lock()
	defer s.Unlock()
	s.change(def)
}

func (s *memStore) change(def schema.Provider) {
	def.Serial = s.defs[def.Name].Serial + 1
	s.defs[def.Name] = def
}

// Remove removes the provider from the store, as another server would, and
// sends its event
func (s *memStore) Remove(name string) {
	s.Lock()
	defer s.Unlock()
	delete(s.defs, name)
	s.send(providerschema.Event{Action: providerschema.EventDelete, Table: "provider", Row: name})
}

// Def returns the provider with the name in the store, and whether it has it
func (s *memStore) Def(name string) (schema.Provider, bool) {
	s.Lock()
	defer s.Unlock()
	def, exists := s.defs[name]
	return def, exists
}

// Counts returns how many providers the store was asked to put and delete
func (s *memStore) Counts() (int, int) {
	s.Lock()
	defer s.Unlock()
	return s.puts, s.deletes
}

func (s *memStore) send(event providerschema.Event) {
	if s.watcher != nil {
		s.watcher <- event
	}
}

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// chat returns the definition of an eliza provider with the name
func chat(name string) schema.Provider {
	return schema.Provider{Name: name, Type: llm.Eliza}
}

// synced returns a registry, which is synced with the store, when it runs
func synced(t *testing.T, store registry.Store) *registry.Registry {
	t.Helper()
	r, err := registry.New(registry.WithStore(store))
	require.NoError(t, err)
	return r
}

// run runs the registry, with the logger, until the test ends, or the
// returned function is called, which returns what Run returned
func run(t *testing.T, r *registry.Registry, logger *slog.Logger) func() error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx, logger) }()
	var once gosync.Once
	var result error
	stop := func() error {
		once.Do(func() {
			cancel()
			result = <-done
		})
		return result
	}
	t.Cleanup(func() { stop() })
	return stop
}

// eventually waits for the condition
func eventually(t *testing.T, condition func() bool, message string) {
	t.Helper()
	require.Eventually(t, condition, 5*time.Second, 5*time.Millisecond, message)
}

// serial returns the serial of the registry's provider with the name, or
// zero when there's none
func serial(r *registry.Registry, name string) uint64 {
	provider, err := r.GetProvider(context.Background(), name)
	if err != nil {
		return 0
	}
	return provider.Serial
}

// has returns true when the registry has the provider with the definition's
// name, type and patterns. Its URL isn't compared, as the registry returns
// the provider's endpoint, rather than the URL it was defined with.
func has(r *registry.Registry, def schema.Provider) bool {
	provider, err := r.GetProvider(context.Background(), def.Name)
	return err == nil && provider.Type == def.Type &&
		slices.Equal(provider.Include, def.Include) && slices.Equal(provider.Exclude, def.Exclude)
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_Sync(t *testing.T) {
	ctx := context.Background()

	t.Run("Startup", func(t *testing.T) {
		// The store's providers are added to the registry when it runs
		store := newStore(chat("chat"), chat("bot"))
		r := synced(t, store)
		run(t, r, nil)
		eventually(t, func() bool { return has(r, chat("chat")) && has(r, chat("bot")) }, "the store's providers weren't added")

		// Without being put in the store again
		time.Sleep(50 * time.Millisecond)
		puts, deletes := store.Counts()
		assert.Zero(t, puts)
		assert.Zero(t, deletes)
	})

	t.Run("Local", func(t *testing.T) {
		// A provider which is created with the registry is put in the store,
		// and one which is deleted is deleted from it
		store := newStore()
		r := synced(t, store)
		run(t, r, nil)
		_, err := r.CreateProvider(ctx, chat("chat"))
		require.NoError(t, err)
		eventually(t, func() bool { _, exists := store.Def("chat"); return exists }, "the provider wasn't put in the store")
		_, err = r.DeleteProvider(ctx, "chat")
		require.NoError(t, err)
		eventually(t, func() bool { _, exists := store.Def("chat"); return !exists }, "the provider wasn't deleted from the store")
	})

	t.Run("Remote", func(t *testing.T) {
		// A provider which is changed in the store, as by another server, is
		// changed in the registry, and one which is removed is removed
		store := newStore(chat("chat"))
		r := synced(t, store)
		run(t, r, nil)
		eventually(t, func() bool { return has(r, chat("chat")) }, "the provider wasn't added")

		store.Change(chat("bot"))
		eventually(t, func() bool { return has(r, chat("bot")) }, "the new provider wasn't added")

		changed := schema.Provider{Name: "chat", Type: llm.Eliza, Exclude: []string{"-de$"}}
		store.Change(changed)
		eventually(t, func() bool { return has(r, changed) }, "the provider wasn't changed")

		store.Remove("chat")
		eventually(t, func() bool { return r.Provider("chat") == nil }, "the provider wasn't removed")
		assert.True(t, has(r, chat("bot")))

		// Without the registry's changes being put back in the store
		puts, deletes := store.Counts()
		assert.Zero(t, puts)
		assert.Zero(t, deletes)
	})

	t.Run("Serial", func(t *testing.T) {
		// A provider has the store's serial, which a change made with the
		// registry increments
		store := newStore(chat("chat"))
		r := synced(t, store)
		run(t, r, nil)
		eventually(t, func() bool { return serial(r, "chat") == 1 }, "the store's provider didn't have its serial")

		_, err := r.UpdateProvider(ctx, "chat", schema.ProviderMeta{Exclude: []string{"-de$"}})
		require.NoError(t, err)
		eventually(t, func() bool {
			def, _ := store.Def("chat")
			return def.Serial == 2 && serial(r, "chat") == 2
		}, "the update didn't have the store's next serial")

		_, err = r.CreateProvider(ctx, chat("bot"))
		require.NoError(t, err)
		eventually(t, func() bool {
			def, exists := store.Def("bot")
			return exists && def.Serial == 1 && serial(r, "bot") == 1
		}, "the new provider didn't have the store's serial")
	})

	t.Run("ConflictDelete", func(t *testing.T) {
		// A provider which is deleted with the registry, which was changed in
		// the store since, as by another server, whose event is missed, is
		// the store's, rather than deleted
		store := newStore(chat("chat"))
		r := synced(t, store)
		run(t, r, nil)
		eventually(t, func() bool { return has(r, chat("chat")) }, "the store's provider wasn't added")

		changed := schema.Provider{Name: "chat", Type: llm.Eliza, Exclude: []string{"-de$"}}
		store.Quietly(changed)
		_, err := r.DeleteProvider(ctx, "chat")
		require.NoError(t, err)
		eventually(t, func() bool { return has(r, changed) }, "the store's change wasn't kept")
		def, exists := store.Def("chat")
		assert.True(t, exists)
		assert.Equal(t, uint64(2), def.Serial)
	})

	t.Run("ConflictUpdate", func(t *testing.T) {
		// A provider which is updated with the registry, which was changed in
		// the store since, is the store's, rather than overwritten
		store := newStore(chat("chat"))
		r := synced(t, store)
		run(t, r, nil)
		eventually(t, func() bool { return has(r, chat("chat")) }, "the store's provider wasn't added")

		changed := schema.Provider{Name: "chat", Type: llm.Eliza, Exclude: []string{"-de$"}}
		store.Quietly(changed)
		_, err := r.UpdateProvider(ctx, "chat", schema.ProviderMeta{Exclude: []string{"-fr$"}})
		require.NoError(t, err)
		eventually(t, func() bool { return has(r, changed) && serial(r, "chat") == 2 }, "the store's change wasn't kept")
		def, _ := store.Def("chat")
		assert.Equal(t, []string{"-de$"}, def.Exclude)
	})

	t.Run("Unreachable", func(t *testing.T) {
		// A provider in the store which can't be reached is added, with its
		// error, rather than left out
		closed := httptest.NewServer(http.NotFoundHandler())
		closed.Close()
		away := schema.Provider{Name: "away", Type: llm.Ollama, URL: closed.URL}
		r := synced(t, newStore(away))
		run(t, r, nil)
		eventually(t, func() bool { return has(r, away) }, "the provider wasn't added")
		provider, err := r.GetProvider(ctx, "away")
		require.NoError(t, err)
		assert.NotEmpty(t, provider.Error)
	})

	t.Run("Added", func(t *testing.T) {
		// A provider which is added without a definition isn't synced, nor
		// is it replaced by the store's provider with its name
		client, err := eliza.New()
		require.NoError(t, err)
		store := newStore(schema.Provider{Name: "chat", Type: llm.Eliza, Exclude: []string{"-de$"}}, chat("bot"))
		r := synced(t, store)
		require.NoError(t, r.Add("chat", client))
		run(t, r, nil)
		eventually(t, func() bool { return has(r, chat("bot")) }, "the store's provider wasn't added")
		assert.Equal(t, client, r.Provider("chat"))
		puts, _ := store.Counts()
		assert.Zero(t, puts)
	})

	t.Run("StoreError", func(t *testing.T) {
		// When the store's providers can't be listed, the registry is left
		// as it is, and they're synced when they can be
		setInterval(t, &registry.PingInterval, 10*time.Millisecond)
		store := newStore(chat("bot"))
		store.err = errors.New("connection refused")
		r := synced(t, store)
		_, err := r.CreateProvider(ctx, chat("chat"))
		require.NoError(t, err)
		run(t, r, nil)
		time.Sleep(50 * time.Millisecond)
		assert.True(t, has(r, chat("chat")))
		assert.Nil(t, r.Provider("bot"))

		store.Lock()
		store.err = nil
		store.Unlock()
		eventually(t, func() bool { return has(r, chat("bot")) }, "the store's provider wasn't added")
		eventually(t, func() bool { _, exists := store.Def("chat"); return exists }, "the registry's provider wasn't put in the store")
	})

	t.Run("Logged", func(t *testing.T) {
		// The errors syncing providers are logged, and they're synced again
		setInterval(t, &registry.PingInterval, 10*time.Millisecond)
		store := newStore(chat("chat"))
		store.err = errors.New("connection refused")
		var log syncBuffer
		r := synced(t, store)
		run(t, r, slog.New(slog.NewTextHandler(&log, nil)))
		eventually(t, func() bool {
			return strings.Contains(log.String(), "syncing providers") && strings.Contains(log.String(), "connection refused")
		}, "the error wasn't logged")

		store.Lock()
		store.err = nil
		store.Unlock()
		eventually(t, func() bool { return has(r, chat("chat")) }, "the store's provider wasn't added")
	})

	t.Run("Missed", func(t *testing.T) {
		// Changes whose events are missed are synced every interval
		setInterval(t, &registry.PingInterval, 10*time.Millisecond)
		store := newStore()
		r := synced(t, store)
		run(t, r, nil)
		store.Lock()
		store.defs["chat"] = chat("chat")
		store.Unlock()
		eventually(t, func() bool { return has(r, chat("chat")) }, "the store's provider wasn't added")
		store.Lock()
		delete(store.defs, "chat")
		store.Unlock()
		eventually(t, func() bool { return r.Provider("chat") == nil }, "the provider wasn't removed")
	})

	t.Run("Stopped", func(t *testing.T) {
		// Run returns nil when it's cancelled
		store := newStore(chat("chat"))
		r := synced(t, store)
		stop := run(t, r, nil)
		eventually(t, func() bool { return has(r, chat("chat")) }, "the store's provider wasn't added")
		assert.NoError(t, stop())

		// And the store's error when the store stops
		store = newStore(chat("chat"))
		r = synced(t, store)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- r.Run(ctx, nil) }()
		eventually(t, func() bool { return has(r, chat("chat")) }, "the store's provider wasn't added")
		errStore := errors.New("store stopped")
		store.stopped <- errStore
		select {
		case err := <-done:
			assert.ErrorIs(t, err, errStore)
		case <-time.After(5 * time.Second):
			t.Fatal("Run didn't return")
		}
	})

	t.Run("Errors", func(t *testing.T) {
		_, err := registry.New(registry.WithStore(nil))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

// syncBuffer is a buffer which is safe to write and read concurrently, for
// a logger
type syncBuffer struct {
	gosync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(data []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.buf.Write(data)
}

func (b *syncBuffer) String() string {
	b.Lock()
	defer b.Unlock()
	return b.buf.String()
}
