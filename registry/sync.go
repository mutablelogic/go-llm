package registry

import (
	"context"
	"errors"
	"fmt"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Store has the definitions of providers, with their names, and their API
// keys' placeholders, such as ${OPENAI_KEY}, rather than their credentials,
// by their names, each with its serial, which the store sets, and changes
// each time the provider does, so a change is made to the provider as it
// was last seen, rather than a newer one, such as one which another server
// made
type Store interface {
	// List returns the providers, with their serials, in any order
	List(context.Context) ([]schema.Provider, error)

	// Put creates the provider, when its serial is zero, and the store has
	// no provider with its name, or replaces the provider with its name,
	// when its serial is that provider's, and returns it, with its new
	// serial. Returns ErrConflict when its serial isn't the provider's, as
	// the provider was changed, or created, since it was seen.
	Put(context.Context, schema.Provider) (schema.Provider, error)

	// Delete deletes the provider with the name, when the serial is its
	// serial, or does nothing when there's none. Returns ErrConflict when
	// the serial isn't its serial, as it was changed since it was seen.
	Delete(ctx context.Context, name string, serial uint64) error

	// Watch sends the store's events to the channel, when its providers are
	// changed by others, such as other servers, from when it returns, until
	// the context is cancelled, as the registry's Watch does. It returns a
	// channel which is sent nil, or an error when the store stops, once the
	// watch has ended. A store which isn't changed by others, such as a
	// file, sends no events, as its providers are listed periodically.
	Watch(context.Context, chan<- providerschema.Event) (<-chan error, error)
}

// syncer keeps a registry's providers the same as those of a store. The
// providers which it knows the store has are kept, as they were when the
// store had them, with their serials, so a change to the registry which the
// store doesn't have yet is told from a change to the store which the
// registry doesn't have, and is made to the store's provider as it was seen.
// When the store's provider was changed since, the providers are synced
// again, so the store's change is kept.
type syncer struct {
	registry *Registry
	store    Store
	known    map[string]schema.Provider
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// syncEventBuffer is how many events of the registry, and of the store, are
// kept while they're synced, before they're missed, when they're synced when
// the providers are next listed
const syncEventBuffer = 64

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// sync makes the registry's providers the same as the store's, once the
// registry's changes which haven't been synced are, so they're in the
// store: a provider which the store has is in the registry, with its
// definition and serial; one which the store doesn't have now, but did, is
// removed from it; one which the registry doesn't have now, but did, is
// deleted from the store, unless the store changed it since; and one which
// the registry has, which the store hasn't had, is put in the store. It
// returns the errors syncing the providers, each of which is synced again
// later, joined, apart from those of providers which the store changed
// since they were seen, as the store's change is kept. When the store's
// providers can't be listed, nothing is changed, and that's the error.
func (s *syncer) sync(ctx context.Context, registryEvents <-chan providerschema.Event) error {
	var errs []error
	failed := func(err error) {
		if err != nil && !errors.Is(err, llm.ErrConflict) {
			errs = append(errs, err)
		}
	}

	// Sync the registry's changes first
	for drained := false; !drained; {
		select {
		case event := <-registryEvents:
			failed(s.local(ctx, event.Row))
		default:
			drained = true
		}
	}

	// The store's providers
	list, err := s.store.List(ctx)
	if err != nil {
		return errors.Join(append(errs, fmt.Errorf("listing the store's providers: %w", err))...)
	}
	stored := make(map[string]schema.Provider, len(list))
	for _, def := range list {
		stored[def.Name] = def
	}

	// The providers the registry doesn't have now, but did, as they were
	// deleted from it, are deleted from the store, unless the store changed
	// them since, when they're the store's
	for name, known := range s.known {
		if _, exists := s.registry.definition(name); exists {
			continue
		}
		delete(s.known, name)
		def, exists := stored[name]
		if !exists || def.Serial != known.Serial {
			continue
		}
		if err := s.store.Delete(ctx, name, known.Serial); errors.Is(err, llm.ErrConflict) {
			continue
		} else if err != nil {
			failed(fmt.Errorf("deleting %q from the store: %w", name, err))
			s.known[name] = known
			continue
		}
		delete(stored, name)
	}

	// The store's providers are in the registry, with their serials
	for name, def := range stored {
		if current, exists := s.registry.definition(name); exists && current.Equal(def) {
			s.registry.setSerial(name, def.Serial)
			s.known[name] = def
			continue
		}
		var err error
		if s.registry.Provider(name) == nil {
			_, err = s.registry.createProvider(ctx, def, false)
		} else {
			_, err = s.registry.updateProvider(ctx, name, def.ProviderMeta, false, def.Serial)
		}
		if err != nil {
			failed(fmt.Errorf("setting %q from the store: %w", name, err))
			continue
		}
		s.known[name] = def
	}

	// The providers the store doesn't have now, but did, are removed from
	// the registry, unless the registry changed them since
	for name, known := range s.known {
		if _, exists := stored[name]; exists {
			continue
		}
		delete(s.known, name)
		if current, exists := s.registry.definition(name); exists && current.Equal(known) {
			if _, err := s.registry.DeleteProvider(ctx, name); err != nil && !errors.Is(err, llm.ErrNotFound) {
				failed(fmt.Errorf("removing %q, which the store doesn't have: %w", name, err))
			}
		}
	}

	// The registry's providers which the store hasn't had are put in it
	for _, name := range s.registry.Names() {
		if _, exists := s.known[name]; !exists {
			failed(s.local(ctx, name))
		}
	}

	// Return the errors
	return errors.Join(errs...)
}

// local syncs a change of the registry's provider with the name to the
// store: when the registry has it, with a definition which the store
// doesn't have, it's put in the store, as the store's provider was seen,
// and has the store's serial, and when it doesn't, and the store did, it's
// deleted from the store. A provider without a definition, such as one added
// with Add, isn't. Returns ErrConflict when the store's provider was changed
// since it was seen, when the providers should be synced, so the store's
// change is kept.
func (s *syncer) local(ctx context.Context, name string) error {
	known, wasKnown := s.known[name]
	if def, exists := s.registry.definition(name); exists {
		if wasKnown && known.Equal(def) {
			return nil
		}
		def.Serial = known.Serial
		stored, err := s.store.Put(ctx, def)
		if err != nil {
			return fmt.Errorf("putting %q in the store: %w", name, err)
		}
		s.known[name] = stored
		s.registry.setSerial(name, stored.Serial)
	} else if wasKnown && s.registry.Provider(name) == nil {
		if err := s.store.Delete(ctx, name, known.Serial); err != nil {
			return fmt.Errorf("deleting %q from the store: %w", name, err)
		}
		delete(s.known, name)
	}
	return nil
}
