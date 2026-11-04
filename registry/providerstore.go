package registry

import (
	"context"

	// Packages
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ProviderStore has the definitions of providers, with their names, and their API
// keys' placeholders, such as ${OPENAI_KEY}, rather than their credentials,
// by their names, each with its serial, which the store sets, and changes
// each time the provider does, so a change is made to the provider as it
// was last seen, rather than a newer one, such as one which another server
// made
type ProviderStore interface {
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
