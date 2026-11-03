package registry_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	registry "github.com/mutablelogic/go-llm/registry"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// FAKES

// provider is a provider with the models, by their names, or which fails
// with the error
type provider struct {
	name   string
	models []string
	err    error
	ping   func(context.Context) error // called by Ping, when it's set
	list   func()                      // called by ListModels, when it's set

	// How many times it's been pinged, and its models listed
	pings, lists atomic.Int32
}

var _ llm.Provider = (*provider)(nil)

func (p *provider) Name() string       { return p.name }
func (p *provider) URL() *url.URL      { return nil }
func (p *provider) Self() llm.Provider { return p }

func (p *provider) Ping(ctx context.Context) error {
	p.pings.Add(1)
	if p.ping != nil {
		return p.ping(ctx)
	}
	return p.err
}

func (p *provider) ListModels(context.Context) ([]llm.Model, error) {
	p.lists.Add(1)
	if p.list != nil {
		p.list()
	}
	if p.err != nil {
		return nil, p.err
	}
	var result []llm.Model
	for _, name := range p.models {
		result = append(result, &model{name: name, provider: p})
	}
	return result, nil
}

func (p *provider) GetModel(_ context.Context, name string) (llm.Model, error) {
	if p.err != nil {
		return nil, p.err
	} else if !slices.Contains(p.models, name) {
		return nil, llm.ErrNotFound.Withf("model %q", name)
	}
	return &model{name: name, provider: p}, nil
}

// model is a model of a provider
type model struct {
	name     string
	provider llm.Provider
	caps     llm.ModelCap
}

var _ llm.Model = (*model)(nil)

func (m *model) Name() string                     { return m.name }
func (m *model) Provider() llm.Provider           { return m.provider }
func (m *model) Description() string              { return "" }
func (m *model) Created() time.Time               { return time.Time{} }
func (m *model) Aliases() []string                { return nil }
func (m *model) Meta() map[string]json.RawMessage { return nil }
func (m *model) InputTokenLimit() *uint           { return nil }
func (m *model) Capabilities() llm.ModelCap       { return m.caps }
func (m *model) OutputTokenLimit() *uint          { return nil }

// newRegistry returns a registry of the providers, each by its own name,
// or the error adding one
func newRegistry(providers ...llm.Provider) (*registry.Registry, error) {
	r, err := registry.New()
	if err != nil {
		return nil, err
	}
	for _, provider := range providers {
		if err := r.Add("", provider); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// names returns the canonical names of the models
func names(r *registry.Registry, models []llm.Model) []string {
	var result []string
	for _, model := range models {
		result = append(result, r.Name(model))
	}
	return result
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_New(t *testing.T) {
	t.Run("Providers", func(t *testing.T) {
		// The providers are by their names, in order
		ollama := &provider{name: "ollama"}
		openai := &provider{name: "openai"}
		r, err := newRegistry(openai, ollama)
		require.NoError(t, err)
		assert.Equal(t, []llm.Provider{ollama, openai}, r.Providers())
		assert.Equal(t, openai, r.Provider("openai"))
		assert.Nil(t, r.Provider("missing"))
	})

	t.Run("Errors", func(t *testing.T) {
		_, err := newRegistry(nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = newRegistry(&provider{name: ""})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = newRegistry(&provider{name: "a:b"})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = newRegistry(&provider{name: "ollama"}, &provider{name: "ollama"})
		assert.ErrorIs(t, err, llm.ErrConflict)
	})
}

func Test_AddRemove(t *testing.T) {
	ctx := context.Background()
	r, err := newRegistry(&provider{name: "openai", models: []string{"gpt-4o"}})
	require.NoError(t, err)

	t.Run("Add", func(t *testing.T) {
		// An added provider's models are found
		ollama := &provider{name: "ollama", models: []string{"llama3.2:latest"}}
		require.NoError(t, r.Add("", ollama))
		assert.Equal(t, ollama, r.Provider("ollama"))
		model, err := r.GetModel(ctx, "llama3.2:latest")
		require.NoError(t, err)
		assert.Equal(t, "ollama:llama3.2:latest", r.Name(model))
		models, err := r.ListModels(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{"ollama:llama3.2:latest", "openai:gpt-4o"}, names(r, models))
	})

	t.Run("AddErrors", func(t *testing.T) {
		assert.ErrorIs(t, r.Add("", nil), llm.ErrBadParameter)
		assert.ErrorIs(t, r.Add("", &provider{name: ""}), llm.ErrBadParameter)
		assert.ErrorIs(t, r.Add("a:b", &provider{name: "x"}), llm.ErrBadParameter)
		for _, name := range []string{"1st", "-ollama", "my ollama", "my.ollama", strings.Repeat("a", 65)} {
			assert.ErrorIs(t, r.Add(name, &provider{name: "x"}), llm.ErrBadParameter, name)
		}
		require.NoError(t, r.Add("my-ollama_2", &provider{name: "x"}))
		_, err := r.Remove("my-ollama_2")
		require.NoError(t, err)
		assert.ErrorIs(t, r.Add("openai", &provider{name: "other"}), llm.ErrConflict)
	})

	t.Run("Remove", func(t *testing.T) {
		// A removed provider is returned, and its models aren't found
		ollama := r.Provider("ollama")
		require.NotNil(t, ollama)
		removed, err := r.Remove("ollama")
		require.NoError(t, err)
		assert.Equal(t, ollama, removed)
		assert.Nil(t, r.Provider("ollama"))
		_, err = r.GetModel(ctx, "llama3.2:latest")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = r.GetModel(ctx, "ollama:llama3.2:latest")
		assert.ErrorIs(t, err, llm.ErrNotFound)

		// And it can't be removed again
		_, err = r.Remove("ollama")
		assert.ErrorIs(t, err, llm.ErrNotFound)

		// But it can be added again
		require.NoError(t, r.Add("", ollama))
		assert.Len(t, r.Providers(), 2)
	})
}

func Test_Ping(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		r, err := newRegistry(&provider{name: "openai"}, &provider{name: "ollama"})
		require.NoError(t, err)
		assert.NoError(t, r.Ping(ctx))

		// And with no providers
		r, err = registry.New()
		require.NoError(t, err)
		assert.NoError(t, r.Ping(ctx))
	})

	t.Run("Failures", func(t *testing.T) {
		// The errors of the providers which fail are joined, each saying
		// which provider it is
		refused := errors.New("connection refused")
		unauthorized := errors.New("unauthorized")
		r, err := newRegistry(
			&provider{name: "ollama", err: refused},
			&provider{name: "openai", err: unauthorized},
			&provider{name: "eliza"},
		)
		require.NoError(t, err)
		err = r.Ping(ctx)
		assert.ErrorIs(t, err, refused)
		assert.ErrorIs(t, err, unauthorized)
		assert.ErrorContains(t, err, "ollama: connection refused")
		assert.ErrorContains(t, err, "openai: unauthorized")
		assert.NotContains(t, err.Error(), "eliza")
	})

	t.Run("Simultaneous", func(t *testing.T) {
		// The providers are pinged at once: each ping waits for every ping
		// to start, which only happens when they run at the same time
		const n = 5
		var started sync.WaitGroup
		started.Add(n)
		all := make(chan struct{})
		go func() { started.Wait(); close(all) }()
		var providers []llm.Provider
		for i := range n {
			providers = append(providers, &provider{name: fmt.Sprint("p", i), ping: func(ctx context.Context) error {
				started.Done()
				select {
				case <-all:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}})
		}
		r, err := newRegistry(providers...)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		assert.NoError(t, r.Ping(ctx))
	})
}

func Test_ListModels(t *testing.T) {
	ctx := context.Background()

	t.Run("Models", func(t *testing.T) {
		// Every provider's models, in order of their providers, and their
		// names
		r, err := newRegistry(
			&provider{name: "openai", models: []string{"gpt-4o", "gpt-4o-mini"}},
			&provider{name: "ollama", models: []string{"phi4:latest", "llama3.2:latest"}},
		)
		require.NoError(t, err)
		models, err := r.ListModels(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{"ollama:llama3.2:latest", "ollama:phi4:latest", "openai:gpt-4o", "openai:gpt-4o-mini"}, names(r, models))
	})

	t.Run("Unreachable", func(t *testing.T) {
		// A provider which can't be reached doesn't stop the others' models
		// being listed, and its error says which it is
		unreachable := errors.New("connection refused")
		r, err := newRegistry(
			&provider{name: "openai", models: []string{"gpt-4o"}},
			&provider{name: "ollama", err: unreachable},
		)
		require.NoError(t, err)
		models, err := r.ListModels(ctx)
		assert.ErrorIs(t, err, unreachable)
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
		assert.ErrorContains(t, err, "ollama: service unavailable: connection refused")
		assert.Equal(t, []string{"openai:gpt-4o"}, names(r, models))
	})

	t.Run("Eliza", func(t *testing.T) {
		client, err := eliza.New()
		require.NoError(t, err)
		r, err := newRegistry(client)
		require.NoError(t, err)
		models, err := r.ListModels(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{"eliza:eliza-1966-de", "eliza:eliza-1966-en", "eliza:eliza-1966-fr"}, names(r, models))
	})
}

func Test_GetModel(t *testing.T) {
	ctx := context.Background()
	unreachable := errors.New("connection refused")
	client, err := eliza.New()
	require.NoError(t, err)
	r, err := newRegistry(
		client,
		&provider{name: "openai", models: []string{"gpt-4o", "shared"}},
		&provider{name: "ollama", models: []string{"llama3.2:latest", "shared"}},
	)
	require.NoError(t, err)

	t.Run("Canonical", func(t *testing.T) {
		// The canonical name is the provider's name, and the model's, which
		// can have the separator
		for _, name := range []string{"openai:gpt-4o", "ollama:llama3.2:latest", "ollama:shared"} {
			model, err := r.GetModel(ctx, name)
			require.NoError(t, err, name)
			assert.Equal(t, name, r.Name(model))
		}

		// Which isn't found in its provider
		_, err := r.GetModel(ctx, "openai:llama3.2:latest")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Name", func(t *testing.T) {
		// A name, or an alias, is found in the one provider which has it
		model, err := r.GetModel(ctx, "gpt-4o")
		require.NoError(t, err)
		assert.Equal(t, "openai:gpt-4o", r.Name(model))

		model, err = r.GetModel(ctx, "llama3.2:latest")
		require.NoError(t, err)
		assert.Equal(t, "ollama:llama3.2:latest", r.Name(model))

		model, err = r.GetModel(ctx, eliza.ModelName)
		require.NoError(t, err)
		assert.Equal(t, "eliza:eliza-1966-en", r.Name(model))
	})

	t.Run("Ambiguous", func(t *testing.T) {
		// A name which more than one provider has needs its canonical name
		_, err := r.GetModel(ctx, "shared")
		assert.ErrorIs(t, err, llm.ErrConflict)
		assert.ErrorContains(t, err, "ollama:shared and openai:shared")
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := r.GetModel(ctx, "missing")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = r.GetModel(ctx, "")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Unreachable", func(t *testing.T) {
		// A model is found when a provider which doesn't have it can't be
		// reached, but when none has it, the provider's error is returned,
		// as it could have it
		r, err := newRegistry(
			&provider{name: "openai", models: []string{"gpt-4o"}},
			&provider{name: "ollama", err: unreachable},
		)
		require.NoError(t, err)
		model, err := r.GetModel(ctx, "gpt-4o")
		require.NoError(t, err)
		assert.Equal(t, "openai:gpt-4o", r.Name(model))

		_, err = r.GetModel(ctx, "llama3.2:latest")
		assert.ErrorIs(t, err, unreachable)
		assert.NotErrorIs(t, err, llm.ErrNotFound)
	})
}

func Test_Concurrent(t *testing.T) {
	// The registry can be used from many goroutines at once
	ctx := context.Background()
	r, err := newRegistry(
		&provider{name: "openai", models: []string{"gpt-4o"}},
		&provider{name: "ollama", models: []string{"llama3.2:latest"}},
	)
	require.NoError(t, err)
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			_, err := r.ListModels(ctx)
			assert.NoError(t, err)
			_, err = r.GetModel(ctx, "gpt-4o")
			assert.NoError(t, err)
			_, err = r.GetModel(ctx, "ollama:llama3.2:latest")
			assert.NoError(t, err)
			assert.NotNil(t, r.Provider("openai"))
		})
	}

	// While a provider is added and removed
	for i := range 50 {
		wg.Go(func() {
			name := fmt.Sprint("extra", i)
			assert.NoError(t, r.Add(name, &provider{name: "extra"}))
			_, err := r.Remove(name)
			assert.NoError(t, err)
		})
	}
	wg.Wait()
	assert.Len(t, r.Providers(), 2)
}

// valueProvider is a provider which isn't comparable, as it's a slice
type valueProvider []string

func (valueProvider) Name() string                                    { return "value" }
func (valueProvider) URL() *url.URL                                   { return nil }
func (p valueProvider) Self() llm.Provider                            { return p }
func (valueProvider) Ping(context.Context) error                      { return nil }
func (valueProvider) ListModels(context.Context) ([]llm.Model, error) { return nil, nil }
func (valueProvider) GetModel(context.Context, string) (llm.Model, error) {
	return nil, llm.ErrNotFound
}

func Test_Names(t *testing.T) {
	ctx := context.Background()

	// Two providers of the same type, by names which the registry gives them
	first := newCapable("ollama", "llama3.2:latest")
	second := newCapable("ollama", "phi4:latest", "llama3.2:latest")
	r, err := registry.New()
	require.NoError(t, err)
	require.NoError(t, r.Add("home", first))
	require.NoError(t, r.Add("work", second))
	assert.Equal(t, []string{"home", "work"}, r.Names())

	t.Run("Canonical", func(t *testing.T) {
		// The models' canonical names are the providers' names in the
		// registry, not their types
		models, err := r.ListModels(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{"home:llama3.2:latest", "work:llama3.2:latest", "work:phi4:latest"}, names(r, models))

		model, err := r.GetModel(ctx, "work:llama3.2:latest")
		require.NoError(t, err)
		assert.Equal(t, "work:llama3.2:latest", r.Name(model))
		assert.Equal(t, second, model.Provider())

		// A name which both have is ambiguous, by their names
		_, err = r.GetModel(ctx, "llama3.2:latest")
		assert.ErrorIs(t, err, llm.ErrConflict)
		assert.ErrorContains(t, err, "home:llama3.2:latest and work:llama3.2:latest")

		// And their type isn't a name
		_, err = r.GetModel(ctx, "ollama:phi4:latest")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Calls", func(t *testing.T) {
		// A model's provider is the one it's of, whatever its name
		first.called, second.called = nil, nil
		model, err := r.GetModel(ctx, "home:llama3.2:latest")
		require.NoError(t, err)
		_, _, err = r.Embed(ctx, model, []llm.Message{user(t, "a")})
		require.NoError(t, err)
		assert.Equal(t, []string{"embed llama3.2:latest"}, first.called)
		assert.Empty(t, second.called)

		// And the download needs a name, as both download
		_, err = r.LoadModel(ctx, "phi4:latest", nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		assert.ErrorContains(t, err, "home and work")
		model, err = r.LoadModel(ctx, "home:phi4:latest", nil)
		require.NoError(t, err)
		assert.Equal(t, "home:phi4:latest", r.Name(model))
	})

	t.Run("Errors", func(t *testing.T) {
		// A provider can only be added once, by one name
		assert.ErrorIs(t, r.Add("again", first), llm.ErrConflict)

		// And it must be comparable, so its models' names are found
		assert.ErrorIs(t, r.Add("value", valueProvider{}), llm.ErrBadParameter)

		// And a model of a provider which isn't in the registry is named by
		// its provider's own name
		other := newCapable("ollama", "gemma")
		model, err := other.GetModel(ctx, "gemma")
		require.NoError(t, err)
		assert.Equal(t, "ollama:gemma", r.Name(model))
	})

	t.Run("Remove", func(t *testing.T) {
		removed, err := r.Remove("work")
		require.NoError(t, err)
		assert.Equal(t, second, removed)
		model, err := r.GetModel(ctx, "llama3.2:latest")
		require.NoError(t, err)
		assert.Equal(t, "home:llama3.2:latest", r.Name(model))
	})
}

func Test_ListModelsOf(t *testing.T) {
	// The models of the providers with the names, in order of their names,
	// which must exist
	ctx := context.Background()
	r, err := newRegistry(
		&provider{name: "openai", models: []string{"gpt-4o"}},
		&provider{name: "ollama", models: []string{"llama3.2:latest"}},
		&provider{name: "deepseek", models: []string{"deepseek-chat"}},
	)
	require.NoError(t, err)
	models, err := r.ListModels(ctx, "openai", "deepseek", "openai")
	require.NoError(t, err)
	assert.Equal(t, []string{"deepseek:deepseek-chat", "openai:gpt-4o"}, names(r, models))
	_, err = r.ListModels(ctx, "missing")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}
