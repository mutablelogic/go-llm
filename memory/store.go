package memory

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/memory/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// store keeps the details, and the memories, in memory, so they aren't
// kept when the process ends
type store struct {
	mu       sync.Mutex
	details  schema.Details
	memories map[string]schema.Memory
}

var _ Store = (*store)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewStore returns a store which keeps the details, and the memories, in
// memory, so they aren't kept when the process ends, such as for tests, and
// examples
func NewStore() Store {
	return &store{memories: make(map[string]schema.Memory)}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (s *store) Details(context.Context) (schema.Details, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clone(s.details), nil
}

func (s *store) PutDetails(_ context.Context, details schema.Details) error {
	if err := details.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.details = clone(details)
	return nil
}

func (s *store) ListMemories(_ context.Context, q string) ([]schema.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []schema.Memory
	for _, memory := range s.memories {
		if memory.Matches(q) {
			result = append(result, memory)
		}
	}
	SortMemories(result)
	return result, nil
}

func (s *store) GetMemory(_ context.Context, key string) (schema.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if memory, ok := s.memories[key]; ok {
		return memory, nil
	}
	return schema.Memory{}, llm.ErrNotFound.Withf("memory %q", key)
}

func (s *store) PutMemory(_ context.Context, memory schema.Memory) error {
	meta, err := memory.MemoryMeta.Validate()
	if err != nil {
		return err
	}
	memory.MemoryMeta = meta
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memories[memory.Key] = memory
	return nil
}

func (s *store) DeleteMemory(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.memories[key]; !ok {
		return llm.ErrNotFound.Withf("memory %q", key)
	}
	delete(s.memories, key)
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// SortMemories sorts the memories, the most recently changed first, and
// then by their keys, as a store lists them
func SortMemories(memories []schema.Memory) {
	slices.SortFunc(memories, func(a, b schema.Memory) int {
		if c := updated(b).Compare(updated(a)); c != 0 {
			return c
		}
		return strings.Compare(a.Key, b.Key)
	})
}

// updated returns when the memory was last changed
func updated(memory schema.Memory) time.Time {
	if memory.Modified.After(memory.Created) {
		return memory.Modified
	}
	return memory.Created
}
