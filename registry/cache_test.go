package registry_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// setInterval sets the interval, until the test ends
func setInterval(t *testing.T, interval *time.Duration, value time.Duration) {
	t.Helper()
	before := *interval
	*interval = value
	t.Cleanup(func() { *interval = before })
}

func Test_ModelCache(t *testing.T) {
	ctx := context.Background()

	t.Run("Cached", func(t *testing.T) {
		// The models are listed once, and then cached
		openai := &provider{name: "openai", models: []string{"gpt-4o"}}
		r, err := newRegistry(openai)
		require.NoError(t, err)
		for range 3 {
			models, err := r.ListModels(ctx)
			require.NoError(t, err)
			assert.Len(t, models, 1)
			_, err = r.GetModel(ctx, "gpt-4o")
			require.NoError(t, err)
		}
		assert.Equal(t, int32(1), openai.lists.Load())
	})

	t.Run("Stale", func(t *testing.T) {
		// Models which were cached longer ago than the interval are listed
		// again
		setInterval(t, &registry.ModelCacheInterval, 0)
		openai := &provider{name: "openai", models: []string{"gpt-4o"}}
		r, err := newRegistry(openai)
		require.NoError(t, err)
		for range 3 {
			_, err := r.ListModels(ctx)
			require.NoError(t, err)
		}
		assert.Equal(t, int32(3), openai.lists.Load())
	})

	t.Run("Failure", func(t *testing.T) {
		// When the models can't be listed again, those which were cached are
		// used
		setInterval(t, &registry.ModelCacheInterval, 0)
		openai := &provider{name: "openai", models: []string{"gpt-4o"}}
		r, err := newRegistry(openai)
		require.NoError(t, err)
		_, err = r.ListModels(ctx)
		require.NoError(t, err)
		openai.err = errors.New("connection refused")
		models, err := r.ListModels(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{"openai:gpt-4o"}, names(r, models))
		_, err = r.GetModel(ctx, "openai:gpt-4o")
		assert.NoError(t, err)

		// But when none were cached, it's the error
		unreachable := &provider{name: "ollama", err: errors.New("connection refused")}
		require.NoError(t, r.Add("", unreachable))
		_, err = r.ListModels(ctx)
		assert.ErrorIs(t, err, unreachable.err)
	})

	t.Run("Patterns", func(t *testing.T) {
		// The models are those which match an include pattern, and no
		// exclude pattern
		r, err := registry.New()
		require.NoError(t, err)
		openai := &provider{name: "openai", models: []string{"gpt-4o", "gpt-4o-mini", "o1", "text-embedding-3-small"}}
		require.NoError(t, r.Add("", openai, registry.WithInclude("^gpt-", "^o1$"), registry.WithExclude("-mini$")))
		models, err := r.ListModels(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{"openai:gpt-4o", "openai:o1"}, names(r, models))
		_, err = r.GetModel(ctx, "gpt-4o-mini")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = r.GetModel(ctx, "openai:text-embedding-3-small")
		assert.ErrorIs(t, err, llm.ErrNotFound)

		// And a pattern must be valid
		assert.ErrorIs(t, r.Add("bad", &provider{name: "x"}, registry.WithInclude("(")), llm.ErrBadParameter)
		assert.ErrorIs(t, r.Add("bad", &provider{name: "x"}, registry.WithExclude("[")), llm.ErrBadParameter)
		assert.Nil(t, r.Provider("bad"))
	})

	t.Run("Download", func(t *testing.T) {
		// A provider which downloads models has them listed again, once a
		// model is downloaded, or deleted, with it
		ollama := newCapable("ollama", "llama3.2:latest")
		r, err := newRegistry(ollama)
		require.NoError(t, err)
		_, err = r.ListModels(ctx)
		require.NoError(t, err)
		require.Equal(t, int32(1), ollama.lists.Load())

		model, err := r.LoadModel(ctx, "phi4:latest", nil)
		require.NoError(t, err)
		ollama.models = append(ollama.models, "phi4:latest")
		models, err := r.ListModels(ctx)
		require.NoError(t, err)
		assert.Equal(t, int32(2), ollama.lists.Load())
		assert.Len(t, models, 2)

		require.NoError(t, r.DeleteModel(ctx, model))
		_, err = r.ListModels(ctx)
		require.NoError(t, err)
		assert.Equal(t, int32(3), ollama.lists.Load())
	})

	t.Run("DownloaderInterval", func(t *testing.T) {
		// A provider which downloads models has them listed again after the
		// ping interval, rather than the model cache interval
		setInterval(t, &registry.PingInterval, 0)
		ollama := newCapable("ollama", "llama3.2:latest")
		openai := &provider{name: "openai", models: []string{"gpt-4o"}}
		r, err := newRegistry(ollama, openai)
		require.NoError(t, err)
		for range 2 {
			_, err := r.ListModels(ctx)
			require.NoError(t, err)
		}
		assert.Equal(t, int32(2), ollama.lists.Load())
		assert.Equal(t, int32(1), openai.lists.Load())
	})
}

func Test_ModelCacheConcurrent(t *testing.T) {
	ctx := context.Background()

	t.Run("Shared", func(t *testing.T) {
		// Requests which find the models stale, while they're listed, use
		// that listing, rather than listing them again
		started, release := make(chan struct{}), make(chan struct{})
		openai := &provider{name: "openai", models: []string{"gpt-4o"}}
		openai.list = func() {
			if openai.lists.Load() == 1 {
				close(started)
				<-release
			}
		}
		r, err := newRegistry(openai)
		require.NoError(t, err)
		var wg sync.WaitGroup
		wg.Go(func() { r.ListModels(ctx) })
		<-started
		for range 5 {
			wg.Go(func() {
				models, err := r.ListModels(ctx)
				assert.NoError(t, err)
				assert.Len(t, models, 1)
			})
		}
		time.Sleep(10 * time.Millisecond)
		close(release)
		wg.Wait()
		assert.Equal(t, int32(1), openai.lists.Load())
	})

	t.Run("Invalidated", func(t *testing.T) {
		// Models which are invalidated, such as when one is deleted, while
		// they're listed, are listed again
		ollama := newCapable("ollama", "llama3.2:latest", "phi4:latest")
		started, release := make(chan struct{}), make(chan struct{})
		ollama.list = func() {
			if ollama.lists.Load() == 2 {
				close(started)
				<-release
			}
		}
		r, err := newRegistry(ollama)
		require.NoError(t, err)
		model, err := r.GetModel(ctx, "phi4:latest")
		require.NoError(t, err)

		// The models are listed again, with phi4, and then it's deleted
		setInterval(t, &registry.PingInterval, 0)
		done := make(chan struct{})
		go func() { r.ListModels(ctx); close(done) }()
		<-started
		setInterval(t, &registry.PingInterval, time.Minute)
		require.NoError(t, r.DeleteModel(ctx, model))
		ollama.models = []string{"llama3.2:latest"}
		close(release)
		<-done

		// So the models are listed again, without it
		_, err = r.GetModel(ctx, "phi4:latest")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		assert.Equal(t, int32(3), ollama.lists.Load())
	})
}

func Test_PingCache(t *testing.T) {
	ctx := context.Background()
	openai := &provider{name: "openai"}
	r, err := newRegistry(openai)
	require.NoError(t, err)

	// A provider is pinged when it's got, and the result is used within the
	// ping interval
	for range 3 {
		_, err := r.GetProvider(ctx, "openai")
		require.NoError(t, err)
		_, err = r.ListProviders(ctx, schema.ProviderListRequest{})
		require.NoError(t, err)
	}
	assert.Equal(t, int32(1), openai.pings.Load())

	// But Ping always pings
	require.NoError(t, r.Ping(ctx))
	require.NoError(t, r.Ping(ctx))
	assert.Equal(t, int32(3), openai.pings.Load())

	// And the result of the last ping is used
	openai.err = errors.New("connection refused")
	require.Error(t, r.Ping(ctx))
	provider, err := r.GetProvider(ctx, "openai")
	require.NoError(t, err)
	assert.Equal(t, "connection refused", provider.Error)
}

func Test_Run(t *testing.T) {
	// Run pings the providers, and lists their models, when it starts, and
	// every ping interval, until it's cancelled
	setInterval(t, &registry.PingInterval, 10*time.Millisecond)
	setInterval(t, &registry.ModelCacheInterval, 0)
	openai := &provider{name: "openai", models: []string{"gpt-4o"}}
	unreachable := &provider{name: "ollama", err: errors.New("connection refused")}
	r, err := newRegistry(openai, unreachable)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- r.Run(ctx, nil) }()
	require.Eventually(t, func() bool {
		return openai.pings.Load() >= 3 && openai.lists.Load() >= 3
	}, 5*time.Second, 5*time.Millisecond)
	cancel()
	assert.NoError(t, <-done)

	// A provider which can't be reached is pinged, but its models aren't
	// listed
	assert.GreaterOrEqual(t, unreachable.pings.Load(), int32(3))
	assert.Zero(t, unreachable.lists.Load())
}

func Test_RunRecentPing(t *testing.T) {
	// A provider which was pinged within half of PingInterval, such as when
	// it was created, isn't pinged again when Run starts
	openai := &provider{name: "openai", models: []string{"gpt-4o"}}
	r, err := newRegistry(openai)
	require.NoError(t, err)
	_, err = r.GetProvider(context.Background(), "openai")
	require.NoError(t, err)
	require.Equal(t, int32(1), openai.pings.Load())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- r.Run(ctx, nil) }()
	require.Eventually(t, func() bool { return openai.lists.Load() >= 1 }, 5*time.Second, 5*time.Millisecond)
	cancel()
	assert.NoError(t, <-done)
	assert.Equal(t, int32(1), openai.pings.Load())
}

func Test_CreateProviderPatterns(t *testing.T) {
	ctx := context.Background()

	// A stub Ollama server, which counts its requests
	var requests atomic.Int32
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":"0.0.1"}`))
	}))
	t.Cleanup(ollama.Close)
	r, err := registry.New()
	require.NoError(t, err)

	// A provider is created with its patterns, which it's described with
	provider, err := r.CreateProvider(ctx, schema.Provider{Type: llm.Ollama, URL: ollama.URL, Include: []string{"^llama"}, Exclude: []string{":8b$"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"^llama"}, provider.Include)
	assert.Equal(t, []string{":8b$"}, provider.Exclude)

	// And it isn't pinged again, as it was when it was created
	pinged := requests.Load()
	_, err = r.GetProvider(ctx, "ollama")
	require.NoError(t, err)
	assert.Equal(t, pinged, requests.Load())

	// A pattern which isn't valid is an error, before the provider is
	// contacted
	requests.Store(0)
	_, err = r.CreateProvider(ctx, schema.Provider{Name: "bad", Type: llm.Ollama, URL: ollama.URL, Include: []string{"("}})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	assert.Zero(t, requests.Load())
	assert.Nil(t, r.Provider("bad"))
}
