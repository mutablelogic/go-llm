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

// syncer keeps a registry's providers the same as those of a store. The
// providers which it knows the store has are kept, as they were when the
// store had them, with their serials, so a change to the registry which the
// store doesn't have yet is told from a change to the store which the
// registry doesn't have, and is made to the store's provider as it was seen.
// When the store's provider was changed since, the providers are synced
// again, so the store's change is kept.
type syncer struct {
	registry *Registry
	store    ProviderStore
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
// store. Each provider is synced by what the store had, when it was last
// seen, what it has now, and what the registry has now: a change to the
// store since it was seen is the registry's, including a deletion, and a
// change to the registry, which the store hasn't seen, when the store
// hasn't changed it, is the store's, including a deletion. A change which
// can't be made is made again later. It returns the errors syncing the
// providers, joined, apart from those of providers which the store changed
// since they were seen, which are synced when they're next listed. When the
// store's providers can't be listed, nothing is changed, and that's the
// error.
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

	// Each provider which the store has had, has, or the registry has
	names := make(map[string]struct{}, len(stored)+len(s.known))
	for name := range s.known {
		names[name] = struct{}{}
	}
	for name := range stored {
		names[name] = struct{}{}
	}
	for _, name := range s.registry.Names() {
		names[name] = struct{}{}
	}
	for name := range names {
		failed(s.provider(ctx, name, stored))
	}

	// Return the errors
	return errors.Join(errs...)
}

// provider syncs the provider with the name, by what the store had, when it
// was last seen, what it has now, which is stored, and what the registry
// has now
func (s *syncer) provider(ctx context.Context, name string, stored map[string]schema.Provider) error {
	known, wasKnown := s.known[name]
	def, inStore := stored[name]
	current, defined := s.registry.definition(name)
	added := !defined && s.registry.Provider(name) != nil
	unchanged := wasKnown && inStore && def.Serial == known.Serial

	switch {
	case added:
		// A provider which was added without a definition, such as with
		// Add, isn't synced
		return nil

	case wasKnown && !inStore:
		// The store deleted it since it was seen, so it's removed from the
		// registry, even when the registry changed it, as the store's
		// change is kept
		delete(s.known, name)
		if defined {
			if _, err := s.registry.DeleteProvider(ctx, name); err != nil && !errors.Is(err, llm.ErrNotFound) {
				return fmt.Errorf("removing %q, which the store deleted: %w", name, err)
			}
		}
		return nil

	case unchanged && !defined:
		// The registry deleted it, and the store hasn't changed it since, so
		// it's deleted from the store, or deleted again later, when it can't
		// be. When the store changed it as it was deleted, it's the store's,
		// when the providers are next listed.
		if err := s.store.Delete(ctx, name, known.Serial); err != nil {
			return fmt.Errorf("deleting %q from the store: %w", name, err)
		}
		delete(s.known, name)
		return nil

	case unchanged && !current.Equal(known):
		// The registry changed it, and the store hasn't since, so it's put
		// in the store, as it was seen, or put again later, when it can't be
		return s.put(ctx, current, known.Serial)

	case inStore:
		// The store has it, and it's changed since it was seen, or it
		// wasn't, so it's the store's, including its type, or has the
		// store's serial, when it's the same
		if err := s.registry.setProvider(ctx, def); err != nil {
			return fmt.Errorf("setting %q from the store: %w", name, err)
		}
		s.known[name] = def
		return nil

	case defined:
		// The registry created it, and the store hasn't had it, so it's
		// created in the store
		return s.put(ctx, current, 0)

	default:
		// Neither has it, so it's forgotten
		delete(s.known, name)
		return nil
	}
}

// put puts the registry's provider in the store, as it was seen, by its
// serial, or as a new provider, when it's zero, and keeps it, as the store
// has it, with the store's serial, which the registry's provider has
func (s *syncer) put(ctx context.Context, def schema.Provider, serial uint64) error {
	def.Serial = serial
	stored, err := s.store.Put(ctx, def)
	if err != nil {
		return fmt.Errorf("putting %q in the store: %w", def.Name, err)
	}
	s.known[def.Name] = stored
	s.registry.setSerial(def.Name, stored.Serial)
	return nil
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
		return s.put(ctx, def, known.Serial)
	} else if wasKnown && s.registry.Provider(name) == nil {
		if err := s.store.Delete(ctx, name, known.Serial); err != nil {
			return fmt.Errorf("deleting %q from the store: %w", name, err)
		}
		delete(s.known, name)
	}
	return nil
}
