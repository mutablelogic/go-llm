//go:build !client

package main

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// defaults are the command's defaults, which are kept between runs, by key
type defaults interface {
	Get(string) any
	Set(string, any) error
}

// providerStore is a store of the registry's providers in the command's defaults,
// as a list of their definitions, with their serials, in order of their
// names, under one key, so they're kept when the server restarts. Only the
// server changes them, so it sends no events. It's safe to use concurrently.
type providerStore struct {
	mu       sync.Mutex
	defaults defaults
}

var _ registry.ProviderStore = (*providerStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// providersKey is the key which the providers are kept under
	providersKey = "llm.providers"

	// providersFile is the name of the file which the server keeps the
	// providers in, in the directory of the command's defaults
	providersFile = "providers.json"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newProviderStore returns a store of the registry's providers in the defaults
func newProviderStore(d defaults) (registry.ProviderStore, error) {
	if d == nil {
		return nil, llm.ErrBadParameter.With("missing defaults")
	}
	return &providerStore{defaults: d}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// migrateProviders moves the providers which the defaults have, as they were
// kept before they were kept in a file of their own, to the file, unless it
// exists, and removes them from the defaults
func migrateProviders(file *jsonFile, d defaults) error {
	value := d.Get(providersKey)
	if value == nil {
		return nil
	} else if !file.Exists() {
		if err := file.Set(providersKey, value); err != nil {
			return err
		}
	}
	return d.Set(providersKey, nil)
}

// List returns the providers in the defaults, in order of their names
func (s *providerStore) List(context.Context) ([]schema.Provider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

// Put puts the provider in the defaults, when its serial is zero, and there's
// no provider with its name, or replaces the provider with its name, when its
// serial is that provider's, and returns it, with its new serial, which is
// one more. Returns ErrBadParameter when it has no name, and ErrConflict when
// its serial isn't the provider's.
func (s *providerStore) Put(_ context.Context, def schema.Provider) (schema.Provider, error) {
	if def.Name == "" {
		return schema.Provider{}, llm.ErrBadParameter.With("missing provider name")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return schema.Provider{}, err
	}
	var serial uint64
	if i := slices.IndexFunc(list, func(other schema.Provider) bool { return other.Name == def.Name }); i >= 0 {
		serial = list[i].Serial
		list = slices.Delete(list, i, i+1)
	}
	if def.Serial != serial {
		return schema.Provider{}, llm.ErrConflict.Withf("provider %q is at serial %d, not %d", def.Name, serial, def.Serial)
	}
	def.Serial = serial + 1
	if err := s.write(append(list, def)); err != nil {
		return schema.Provider{}, err
	}
	return def, nil
}

// Delete deletes the provider with the name from the defaults, when the
// serial is its serial, or does nothing when there's none. Returns
// ErrConflict when the serial isn't its serial.
func (s *providerStore) Delete(_ context.Context, name string, serial uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(list, func(def schema.Provider) bool { return def.Name == name })
	if i < 0 {
		return nil
	} else if list[i].Serial != serial {
		return llm.ErrConflict.Withf("provider %q is at serial %d, not %d", name, list[i].Serial, serial)
	}
	return s.write(slices.Delete(list, i, i+1))
}

// Watch sends no events, as only the server changes the providers, until the
// context is cancelled, when the returned channel is sent nil
func (s *providerStore) Watch(ctx context.Context, ch chan<- eventschema.Event) (<-chan error, error) {
	if ch == nil {
		return nil, llm.ErrBadParameter.With("missing channel")
	}
	done := make(chan error, 1)
	go func() {
		<-ctx.Done()
		done <- nil
	}()
	return done, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// read returns the providers in the defaults, or none when there are none.
// The defaults have the providers as they were set, or as JSON was decoded
// into, when they were read from disk, so they're encoded as JSON, and
// decoded as providers. The caller holds the lock.
func (s *providerStore) read() ([]schema.Provider, error) {
	value := s.defaults.Get(providersKey)
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var list []schema.Provider
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, llm.ErrInternalServerError.Withf("%s: %v", providersKey, err)
	}
	return list, nil
}

// write sets the providers in the defaults, in order of their names, or
// removes them, when there are none. The caller holds the lock.
func (s *providerStore) write(list []schema.Provider) error {
	if len(list) == 0 {
		return s.defaults.Set(providersKey, nil)
	}
	slices.SortFunc(list, func(a, b schema.Provider) int { return cmp.Compare(a.Name, b.Name) })
	return s.defaults.Set(providersKey, list)
}
