package resource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/resource/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Store keeps the resources, such as in files, or a database, so they're
// kept when the server restarts: their content, once, by its hash, and
// each resource, by its hash and session, which refers to its content
type Store interface {
	// Write keeps the content, as it's read, and returns its hash, which is
	// its SHA-256, in hexadecimal, and its size. Content which is kept
	// already, by its hash, is kept once. Content which can't be read whole
	// isn't kept.
	Write(ctx context.Context, r io.Reader) (hash string, size int64, err error)

	// Open returns a reader of the content with the hash, which is closed
	// when it's read, or ErrNotFound when there's none
	Open(ctx context.Context, hash string) (io.ReadCloser, error)

	// Remove removes the content with the hash, or does nothing when there's
	// none
	Remove(ctx context.Context, hash string) error

	// Contents returns the hashes of the content which is kept, in any order
	Contents(ctx context.Context) ([]string, error)

	// Create adds the resource, or returns ErrConflict when there's one with
	// its key
	Create(context.Context, schema.Resource) error

	// Get returns the resource with the key, or ErrNotFound when there's none
	Get(context.Context, schema.ResourceKey) (schema.Resource, error)

	// List returns the resources, in any order
	List(context.Context) ([]schema.Resource, error)

	// Update calls the function with the resource with the key, which it
	// changes, and replaces it with what the function changed it to, apart
	// from its key, and returns it, or returns the function's error, without
	// changing it, or ErrNotFound when there's none
	Update(context.Context, schema.ResourceKey, func(*schema.Resource) error) (schema.Resource, error)

	// Delete deletes the resource with the key, and returns it, as it was,
	// or ErrNotFound when there's none. Its content is kept, until it's
	// removed.
	Delete(context.Context, schema.ResourceKey) (schema.Resource, error)
}

// store keeps the resources, and their content, in memory, so they aren't
// kept when the process ends
type store struct {
	mu        sync.Mutex
	contents  map[string][]byte
	resources map[storeKey]schema.Resource
}

// storeKey is a resource's key, as a map's key
type storeKey struct {
	hash, session string
}

var _ Store = (*store)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewStore returns a store which keeps the resources, and their content, in
// memory, so they aren't kept when the process ends, such as for tests, and
// examples
func NewStore() Store {
	return &store{contents: make(map[string][]byte), resources: make(map[storeKey]schema.Resource)}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (s *store) Write(ctx context.Context, r io.Reader) (string, int64, error) {
	var buf bytes.Buffer
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(&buf, hash), contextReader{ctx, r})
	if err != nil {
		return "", 0, err
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.contents[sum]; !exists {
		s.contents[sum] = buf.Bytes()
	}
	return sum, size, nil
}

func (s *store) Open(_ context.Context, hash string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, exists := s.contents[hash]
	if !exists {
		return nil, llm.ErrNotFound.Withf("resource content %q", hash)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *store) Remove(_ context.Context, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.contents, hash)
	return nil
}

func (s *store) Contents(context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, 0, len(s.contents))
	for hash := range s.contents {
		result = append(result, hash)
	}
	return result, nil
}

func (s *store) Create(_ context.Context, resource schema.Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := keyOf(resource.Key())
	if _, exists := s.resources[key]; exists {
		return llm.ErrConflict.Withf("resource %s exists", resource.Key())
	}
	s.resources[key] = Clone(resource)
	return nil
}

func (s *store) Get(_ context.Context, key schema.ResourceKey) (schema.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if resource, exists := s.resources[keyOf(key)]; exists {
		return Clone(resource), nil
	}
	return schema.Resource{}, llm.ErrNotFound.Withf("resource %s", key)
}

func (s *store) List(context.Context) ([]schema.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]schema.Resource, 0, len(s.resources))
	for _, resource := range s.resources {
		result = append(result, Clone(resource))
	}
	return result, nil
}

func (s *store) Update(_ context.Context, key schema.ResourceKey, fn func(*schema.Resource) error) (schema.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resource, exists := s.resources[keyOf(key)]
	if !exists {
		return schema.Resource{}, llm.ErrNotFound.Withf("resource %s", key)
	}
	resource = Clone(resource)
	if err := fn(&resource); err != nil {
		return schema.Resource{}, err
	}
	resource.Hash, resource.Session = key.Hash, key.Session
	s.resources[keyOf(key)] = Clone(resource)
	return resource, nil
}

func (s *store) Delete(_ context.Context, key schema.ResourceKey) (schema.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resource, exists := s.resources[keyOf(key)]
	if !exists {
		return schema.Resource{}, llm.ErrNotFound.Withf("resource %s", key)
	}
	delete(s.resources, keyOf(key))
	return resource, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// Clone returns a copy of the resource, whose session is a copy, so changing
// it doesn't change the resource
func Clone(resource schema.Resource) schema.Resource {
	if resource.Session != nil {
		session := *resource.Session
		resource.Session = &session
	}
	return resource
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE

// keyOf returns the resource's key, as a map's key
func keyOf(key schema.ResourceKey) storeKey {
	if key.Session == nil {
		return storeKey{hash: key.Hash}
	}
	return storeKey{hash: key.Hash, session: key.Session.String()}
}

// contextReader is a reader which fails once its context is done
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
