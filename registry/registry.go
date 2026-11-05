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
	schema "github.com/mutablelogic/go-llm/registry/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
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
	providers map[string]*cached
	statuses  statuses
	agents    *agentNamespace

	// lifecycle is held to delete a session, and, shared, to change its
	// status, so a status isn't set as, or after, it's deleted
	lifecycle sync.RWMutex
}

// entry is a provider, its name in the registry, and its cache
type entry struct {
	name     string
	provider llm.Provider
	cache    *cached
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
	r := &Registry{providers: make(map[string]*cached)}
	if err := r.opt.apply(opts); err != nil {
		return nil, err
	}

	// With a toolkit, and agents, the agents are tools, in a namespace of
	// their own, which Run adds the stored agents to
	if r.toolkit != nil && r.agentStore != nil {
		r.agents = newAgentNamespace()
		if err := r.toolkit.Add(r.agents); err != nil {
			return nil, err
		}
	}
	return r, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Add adds the provider, with the name, which must be an identifier, such as
// my-ollama, which starts with a letter, followed by letters, numbers,
// underscores or hyphens, or with its own name, such as its type, when the
// name is empty, and the options, such as the patterns of the names of the
// models which are included, or excluded. Returns ErrConflict when a
// provider with the name, or the provider itself, is added already, and
// ErrBadParameter when a pattern isn't valid. Watchers are sent that it was
// created.
func (r *Registry) Add(name string, provider llm.Provider, opts ...ProviderOpt) error {
	if provider == nil {
		return llm.ErrBadParameter.With("provider is nil")
	} else if name == "" {
		name = provider.Name()
	}
	c, err := newCached(provider, opts...)
	if err != nil {
		return err
	}
	c.changed = func() { r.updated(name, c) }

	// It's published as it's added, under the lock, so a concurrent removal
	// can't be published before it
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.add(name, c); err != nil {
		return err
	}
	r.created(name)
	return nil
}

// Remove removes the provider with the name, and returns it, so it can be
// closed, or returns ErrNotFound when there's none. Calls which have started
// already, such as listing models, still use it. Watchers are sent that it
// was deleted.
func (r *Registry) Remove(name string) (llm.Provider, error) {
	c, err := r.remove(name)
	if err != nil {
		return nil, err
	}
	return c.provider, nil
}

// SessionStore returns the store of the registry's sessions, or nil when it
// has none, when it has no sessions
func (r *Registry) SessionStore() SessionStore {
	return r.sessionStore
}

// AgentStore returns the store of the registry's agents, or nil when it has
// none, when it has no agents
func (r *Registry) AgentStore() AgentStore {
	return r.agentStore
}

// Toolkit returns the toolkit of the tools which the registry's models can
// call, or nil when it has none
func (r *Registry) Toolkit() *toolkit.Toolkit {
	return r.toolkit
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
	if c := r.cached(name); c != nil {
		return c.provider
	}
	return nil
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

// Model returns the model, with the name of its provider in the registry,
// and its canonical name
func (r *Registry) Model(model llm.Model) *schema.Model {
	provider, _, _ := strings.Cut(r.Name(model), separator)
	return schema.NewModel(provider, r.Name(model), model)
}

// Ping pings every provider at once, and returns when they've replied, with
// the errors of those which failed joined, each saying which provider it
// is, or nil when every ping succeeded. The providers are pinged, whenever
// they were last.
func (r *Registry) Ping(ctx context.Context) (err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Ping")
	defer func() { endSpan(err) }()

	entries := r.entries()
	errs := make([]error, len(entries))
	each(entries, func(i int, e entry) {
		if err := e.cache.ping(ctx, 0); err != nil {
			errs[i] = fmt.Errorf("%s: %w", e.name, err)
		}
	})
	return errors.Join(errs...)
}

// ListModels returns the models of every provider, or of the providers with
// the names, when there are any, which must exist, which match their
// patterns, in order of their providers' names, and then their names. The
// models are cached, and are listed again, at once, when they're stale.
// When a provider's models can't be listed, such as when it can't be
// reached, and none are cached, the others' models are returned, with an
// error saying which providers' couldn't be.
func (r *Registry) ListModels(ctx context.Context, providers ...string) (_ []llm.Model, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.ListModels",
		attribute.StringSlice("arg.providers", providers),
	)
	defer func() { endSpan(err) }()

	// The providers, or those with the names
	entries := r.entries()
	if len(providers) > 0 {
		var selected []entry
		for _, name := range providers {
			i := slices.IndexFunc(entries, func(e entry) bool { return e.name == name })
			if i < 0 {
				return nil, llm.ErrNotFound.Withf("provider %q", name)
			}
			if !slices.ContainsFunc(selected, func(e entry) bool { return e.name == name }) {
				selected = append(selected, entries[i])
			}
		}
		slices.SortFunc(selected, func(a, b entry) int { return cmp.Compare(a.name, b.name) })
		entries = selected
	}
	models := make([][]llm.Model, len(entries))
	errs := make([]error, len(entries))
	each(entries, func(i int, e entry) {
		models[i], errs[i] = e.cache.listModels(ctx)
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
// registry, and its name, or otherwise its name, or an alias, which every
// provider's models are searched for at once. A model must match its
// provider's patterns. The models are cached, as ListModels does. Returns ErrNotFound when no provider has
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
		if c := r.cached(provider); c != nil {
			return c.getModel(ctx, model)
		}
	}

	// Otherwise ask every provider
	entries := r.entries()
	models := make([]llm.Model, len(entries))
	errs := make([]error, len(entries))
	each(entries, func(i int, e entry) {
		models[i], errs[i] = e.cache.getModel(ctx, name)
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
// has no separator, and not added already, as mustn't the provider, so its
// models' canonical names are found by which provider they're of. The
// caller holds the lock.
func (r *Registry) add(name string, c *cached) error {
	provider := c.provider
	if !reflect.TypeOf(provider).Comparable() {
		return llm.ErrBadParameter.Withf("provider %q isn't comparable, such as a pointer", name)
	} else if !types.IsIdentifier(name) {
		return llm.ErrBadParameter.Withf("invalid provider name %q, which must be an identifier", name)
	} else if _, exists := r.providers[name]; exists {
		return llm.ErrConflict.Withf("provider %q is added already", name)
	}
	for other, p := range r.providers {
		if p.provider == provider {
			return llm.ErrConflict.Withf("provider %q is added already, as %q", name, other)
		}
	}
	r.providers[name] = c
	return nil
}

// replace replaces the provider with the name, which is the existing one,
// with the new one, and publishes that it was updated. Returns ErrConflict
// when the provider with the name has changed, such as when it was removed,
// or replaced, since it was got.
func (r *Registry) replace(name string, existing, c *cached) error {
	c.changed = func() { r.updated(name, c) }
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.providers[name] != existing {
		return llm.ErrConflict.Withf("provider %q was changed as it was updated", name)
	}
	r.providers[name] = c
	r.replaced(name)
	return nil
}

// remove removes the provider with the name, and returns it, and its
// cache, or returns ErrNotFound when there's none, and sends watchers that
// it was deleted
func (r *Registry) remove(name string) (*cached, error) {
	// It's published as it's removed, under the lock, so a concurrent
	// replacement can't be published before it
	r.mu.Lock()
	defer r.mu.Unlock()
	c, exists := r.providers[name]
	if !exists {
		return nil, llm.ErrNotFound.Withf("provider %q", name)
	}
	delete(r.providers, name)
	r.deleted(name)
	return c, nil
}

// cached returns the provider with the name, and its cache, or nil when
// there's none
func (r *Registry) cached(name string) *cached {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.providers[name]
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
		if p.provider == provider {
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
	for name, c := range r.providers {
		result = append(result, entry{name: name, provider: c.provider, cache: c})
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
