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
// one, such as one which another process made
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

// memoryStore is a store of connectors in memory, which aren't kept once the
// process ends, which is the registry's, unless it's given another
type memoryStore struct {
	mu         sync.Mutex
	connectors map[string]schema.Connector
}

var _ Store = (*memoryStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewMemoryStore returns a store of connectors in memory, which aren't kept
// once the process ends
func NewMemoryStore() Store {
	return &memoryStore{connectors: make(map[string]schema.Connector)}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (s *memoryStore) List(context.Context) ([]schema.Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]schema.Connector, 0, len(s.connectors))
	for _, connector := range s.connectors {
		result = append(result, connector)
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
	connector.Serial++
	s.connectors[connector.Name] = connector
	return connector, nil
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
