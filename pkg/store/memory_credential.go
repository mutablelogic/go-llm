package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	// Packages

	encrypt "github.com/mutablelogic/go-llm/pkg/encrypt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// MemoryCredentialStore is an in-memory implementation of CredentialStore.
// Credentials are encrypted at rest using AES-256-GCM with a per-entry salt.
// It is safe for concurrent use.
type MemoryCredentialStore struct {
	mu         sync.RWMutex
	passphrase string
	creds      map[string][]byte // keyed by server URL, value is encrypted blob
}

var _ schema.CredentialStore = (*MemoryCredentialStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewMemoryCredentialStore creates a new empty in-memory credential store.
// The passphrase is used to encrypt and decrypt credentials.
func NewMemoryCredentialStore(passphrase string) (*MemoryCredentialStore, error) {
	if err := encrypt.ValidatePassphrase(passphrase); err != nil {
		return nil, err
	}
	return &MemoryCredentialStore{
		passphrase: passphrase,
		creds:      make(map[string][]byte),
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// GetCredential retrieves the credential for the given server URL.
func (s *MemoryCredentialStore) GetCredential(_ context.Context, rawURL string) (*schema.OAuthCredentials, error) {
	canonicalURL, err := schema.CanonicalURL(rawURL)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	blob, ok := s.creds[canonicalURL]
	if !ok {
		return nil, schema.ErrNotFound.Withf("credential not found for %q", canonicalURL)
	}

	plaintext, err := encrypt.Decrypt[[]byte](s.passphrase, blob)
	if err != nil {
		return nil, fmt.Errorf("credential decrypt failed for %q: %w", canonicalURL, err)
	}

	var cred schema.OAuthCredentials
	if err := json.Unmarshal(plaintext, &cred); err != nil {
		return nil, fmt.Errorf("credential unmarshal failed for %q: %w", canonicalURL, err)
	}
	return &cred, nil
}

// SetCredential stores (or updates) the credential for the given server URL.
func (s *MemoryCredentialStore) SetCredential(_ context.Context, rawURL string, cred schema.OAuthCredentials) error {
	canonicalURL, err := schema.CanonicalURL(rawURL)
	if err != nil {
		return err
	}
	plaintext, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("credential marshal failed: %w", err)
	}

	blob, err := encrypt.Encrypt(s.passphrase, plaintext)
	if err != nil {
		return fmt.Errorf("credential encrypt failed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.creds[canonicalURL] = blob
	return nil
}

// DeleteCredential removes the credential for the given server URL.
func (s *MemoryCredentialStore) DeleteCredential(_ context.Context, rawURL string) error {
	canonicalURL, err := schema.CanonicalURL(rawURL)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.creds[canonicalURL]; !ok {
		return schema.ErrNotFound.Withf("credential not found for %q", canonicalURL)
	}
	delete(s.creds, canonicalURL)
	return nil
}
