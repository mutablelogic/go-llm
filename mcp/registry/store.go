package registry

import (
	"cmp"
	"context"
	"slices"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Store has the definitions of connectors, by their names, each with its
// serial, which the store sets, and changes each time the connector does, so
// a change is made to the connector as it was last seen, rather than a newer
// one, such as one which another process made. A serial isn't used again
// for a name, even once its connector is deleted, and another created with
// the name, so a change to one isn't made to another.
type Store interface {
	// List returns the connectors, with their serials, in any order
	List(context.Context) ([]schema.Connector, error)

	// Put creates the connector, when its serial is zero, and the store has no
	// connector with its name, or replaces the connector with its name, when its
	// serial is that connector's, and returns it, with its new serial. Returns
	// ErrConflict when its serial isn't the connector's, as the connector was
	// changed, or created, since it was seen.
	Put(context.Context, schema.Connector) (schema.Connector, error)

	// Delete deletes the connector with the name, when the serial is its
	// serial, or does nothing when there's none. Returns ErrConflict when
	// the serial isn't its serial, as it was changed since it was seen.
	Delete(ctx context.Context, name string, serial uint64) error
}

// memoryCredentials is how each connector's server is authorized, in
// memory, which is kept when the credentials provider can't keep it, such
// as env's, so it isn't kept once the process ends
type memoryCredentials struct {
	mu          sync.Mutex
	credentials map[string]schema.Credential
}

// newMemoryCredentials returns credentials in memory
func newMemoryCredentials() *memoryCredentials {
	return &memoryCredentials{credentials: make(map[string]schema.Credential)}
}

func (s *memoryCredentials) get(name string) (*schema.Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	credential, exists := s.credentials[name]
	if !exists {
		return nil, llm.ErrNotFound.Withf("connector %q has no credential", name)
	}
	credential.RedirectURIs = slices.Clone(credential.RedirectURIs)
	credential.Scopes = slices.Clone(credential.Scopes)
	return &credential, nil
}

func (s *memoryCredentials) put(name string, credential schema.Credential) {
	s.mu.Lock()
	defer s.mu.Unlock()
	credential.RedirectURIs = slices.Clone(credential.RedirectURIs)
	credential.Scopes = slices.Clone(credential.Scopes)
	s.credentials[name] = credential
}

func (s *memoryCredentials) delete(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.credentials, name)
}

// memoryStore is a store of connectors in memory, which aren't kept once the
// process ends, which is the registry's, unless it's given another
type memoryStore struct {
	mu         sync.Mutex
	connectors map[string]schema.Connector
	serials    map[string]uint64 // The last serial of each name, deleted, or not
}

var _ Store = (*memoryStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewMemoryStore returns a store of connectors in memory, which aren't kept
// once the process ends
func NewMemoryStore() Store {
	return &memoryStore{connectors: make(map[string]schema.Connector), serials: make(map[string]uint64)}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (s *memoryStore) List(context.Context) ([]schema.Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]schema.Connector, 0, len(s.connectors))
	for _, connector := range s.connectors {
		result = append(result, clone(connector))
	}
	slices.SortFunc(result, func(a, b schema.Connector) int { return cmp.Compare(a.Name, b.Name) })
	return result, nil
}

func (s *memoryStore) Put(_ context.Context, connector schema.Connector) (schema.Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, exists := s.connectors[connector.Name]
	switch {
	case !exists && connector.Serial != 0:
		return schema.Connector{}, llm.ErrConflict.Withf("connector %q was deleted", connector.Name)
	case exists && connector.Serial != existing.Serial:
		return schema.Connector{}, llm.ErrConflict.Withf("connector %q was changed", connector.Name)
	}
	// Its serial is one more than the last of its name, so one which was
	// deleted isn't used again
	connector.Serial = s.serials[connector.Name] + 1
	s.serials[connector.Name] = connector.Serial
	s.connectors[connector.Name] = clone(connector)
	return clone(connector), nil
}

func (s *memoryStore) Delete(_ context.Context, name string, serial uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, exists := s.connectors[name]; !exists {
		return nil
	} else if existing.Serial != serial {
		return llm.ErrConflict.Withf("connector %q was changed", name)
	}
	delete(s.connectors, name)
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// clone returns the connector, with its own patterns, so a change to them
// isn't a change to the store's
func clone(connector schema.Connector) schema.Connector {
	connector.Include = slices.Clone(connector.Include)
	connector.Exclude = slices.Clone(connector.Exclude)
	return connector
}
