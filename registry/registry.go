// Package registry has the providers of models, by names which the registry
// gives them, and finds the models across them
package registry

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Registry is the providers of models, by names which it gives them, so
// there can be more than one provider of a type, such as two Ollama servers.
// Providers can be added and removed. It's safe to use concurrently.
type Registry struct {
	opt
	mu        sync.RWMutex
	providers map[string]llm.Provider
}

// entry is a provider, and its name in the registry
type entry struct {
	name     string
	provider llm.Provider
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// separator separates a provider's name from a model's, in a model's
// canonical name, such as ollama:llama3.2:latest
const separator = ":"

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a registry without providers, which are added with Add, with
// the options, such as its tracer
func New(opts ...Opt) (*Registry, error) {
	r := &Registry{providers: make(map[string]llm.Provider)}
	if err := r.opt.apply(opts); err != nil {
		return nil, err
	}
	return r, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Add adds the provider, with the name, which must be an identifier, such as
// my-ollama, which starts with a letter, followed by letters, numbers,
// underscores or hyphens, or
// with its own name, such as its type, when the name is empty. Returns
// ErrConflict when a provider with the name, or the provider itself, is
// added already.
func (r *Registry) Add(name string, provider llm.Provider) error {
	if provider == nil {
		return llm.ErrBadParameter.With("provider is nil")
	} else if name == "" {
		name = provider.Name()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.add(name, provider)
}

// Remove removes the provider with the name, and returns it, so it can be
// closed, or returns ErrNotFound when there's none. Calls which have started
// already, such as listing models, still use it.
func (r *Registry) Remove(name string) (llm.Provider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	provider, exists := r.providers[name]
	if !exists {
		return nil, llm.ErrNotFound.Withf("provider %q", name)
	}
	delete(r.providers, name)
	return provider, nil
}

// Names returns the names of the providers, in order
func (r *Registry) Names() []string {
	var names []string
	for _, e := range r.entries() {
		names = append(names, e.name)
	}
	return names
}

// Providers returns the providers, in order of their names
func (r *Registry) Providers() []llm.Provider {
	var providers []llm.Provider
	for _, e := range r.entries() {
		providers = append(providers, e.provider)
	}
	return providers
}

// Provider returns the provider with the name, or nil when there's none
func (r *Registry) Provider(name string) llm.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.providers[name]
}

// Name returns the canonical name of the model, which is the name of its
// provider in the registry, and its name, such as my-ollama:llama3.2:latest,
// or its provider's own name, when its provider isn't in the registry
func (r *Registry) Name(model llm.Model) string {
	provider := model.Provider()
	if provider == nil {
		return model.Name()
	} else if name, ok := r.nameOf(provider); ok {
		return name + separator + model.Name()
	}
	return provider.Name() + separator + model.Name()
}

// Ping pings every provider at once, and returns when they've replied, with
// the errors of those which failed joined, each saying which provider it
// is, or nil when every ping succeeded
func (r *Registry) Ping(ctx context.Context) (err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Ping")
	defer func() { endSpan(err) }()

	entries := r.entries()
	errs := make([]error, len(entries))
	each(entries, func(i int, e entry) {
		if err := e.provider.Ping(ctx); err != nil {
			errs[i] = fmt.Errorf("%s: %w", e.name, err)
		}
	})
	return errors.Join(errs...)
}

// ListModels returns the models of every provider, which are listed at
// once, in order of their providers' names, and then their names. When a
// provider's models can't be listed, such as when it can't be reached, the
// others' models are returned, with an error saying which providers'
// couldn't be.
func (r *Registry) ListModels(ctx context.Context) (_ []llm.Model, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.ListModels")
	defer func() { endSpan(err) }()

	entries := r.entries()
	models := make([][]llm.Model, len(entries))
	errs := make([]error, len(entries))
	each(entries, func(i int, e entry) {
		models[i], errs[i] = e.provider.ListModels(ctx)
		if errs[i] != nil {
			errs[i] = fmt.Errorf("%s: %w", e.name, errs[i])
		}
	})

	// Sort each provider's models by their names, and join them
	var result []llm.Model
	for _, list := range models {
		list = slices.Clone(list)
		slices.SortStableFunc(list, func(a, b llm.Model) int {
			return cmp.Compare(a.Name(), b.Name())
		})
		result = append(result, list...)
	}
	return result, errors.Join(errs...)
}

// GetModel returns the model with the name, which is its canonical name,
// such as ollama:llama3.2:latest, which is its provider's name in the
// registry, and its name, or otherwise its name, or an alias, which the
// providers are asked for at once. Returns ErrNotFound when no provider has
// it, and ErrConflict when more than one does, so its canonical name is
// needed. When no provider has it, and a provider can't be asked, such as
// when it can't be reached, that provider's error is returned.
func (r *Registry) GetModel(ctx context.Context, name string) (_ llm.Model, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.GetModel",
		attribute.String("arg.name", name),
	)
	defer func() { endSpan(err) }()

	if name == "" {
		return nil, llm.ErrBadParameter.With("model name is required")
	}

	// A canonical name is its provider's name, and its name
	if provider, model, ok := strings.Cut(name, separator); ok {
		if p := r.Provider(provider); p != nil {
			return p.GetModel(ctx, model)
		}
	}

	// Otherwise ask every provider
	entries := r.entries()
	models := make([]llm.Model, len(entries))
	errs := make([]error, len(entries))
	each(entries, func(i int, e entry) {
		models[i], errs[i] = e.provider.GetModel(ctx, name)
		if errs[i] != nil && !errors.Is(errs[i], llm.ErrNotFound) {
			errs[i] = fmt.Errorf("%s: %w", e.name, errs[i])
		} else {
			errs[i] = nil
		}
	})

	// Return the one model which is found
	var found []string
	var result llm.Model
	for i, model := range models {
		if model != nil {
			found = append(found, entries[i].name+separator+model.Name())
			result = model
		}
	}
	switch {
	case len(found) == 1:
		return result, nil
	case len(found) > 1:
		return nil, llm.ErrConflict.Withf("model %q is %s", name, strings.Join(found, " and "))
	case errors.Join(errs...) != nil:
		return nil, errors.Join(errs...)
	default:
		return nil, llm.ErrNotFound.Withf("model %q", name)
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// add adds the provider, with the name, which must be an identifier, so it
// has no separator, and not added already, as mustn't the provider, so its models' canonical names are
// found by which provider they're of. The caller holds the lock.
func (r *Registry) add(name string, provider llm.Provider) error {
	if provider == nil {
		return llm.ErrBadParameter.With("provider is nil")
	} else if !reflect.TypeOf(provider).Comparable() {
		return llm.ErrBadParameter.Withf("provider %q isn't comparable, such as a pointer", name)
	} else if !types.IsIdentifier(name) {
		return llm.ErrBadParameter.Withf("invalid provider name %q, which must be an identifier", name)
	} else if _, exists := r.providers[name]; exists {
		return llm.ErrConflict.Withf("provider %q is added already", name)
	}
	for other, p := range r.providers {
		if p == provider {
			return llm.ErrConflict.Withf("provider %q is added already, as %q", name, other)
		}
	}
	r.providers[name] = provider
	return nil
}

// nameOf returns the name of the provider in the registry, and whether it's
// in the registry
func (r *Registry) nameOf(provider llm.Provider) (string, bool) {
	if !reflect.TypeOf(provider).Comparable() {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for name, p := range r.providers {
		if p == provider {
			return name, true
		}
	}
	return "", false
}

// entries returns the providers, and their names, in order of their names
func (r *Registry) entries() []entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]entry, 0, len(r.providers))
	for name, provider := range r.providers {
		result = append(result, entry{name: name, provider: provider})
	}
	slices.SortFunc(result, func(a, b entry) int {
		return cmp.Compare(a.name, b.name)
	})
	return result
}

// each calls the function with each of the providers, and its index, at
// once, and returns when they've returned. The providers are called without
// the registry's lock, as they can take a while, such as to make requests.
func each(entries []entry, fn func(int, entry)) {
	var wg sync.WaitGroup
	for i, e := range entries {
		wg.Go(func() {
			fn(i, e)
		})
	}
	wg.Wait()
}
