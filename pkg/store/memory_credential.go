package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
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
func (s *MemoryCredentialStore) GetCredential(_ context.Context, url string) (*schema.OAuthCredentials, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	blob, ok := s.creds[url]
	if !ok {
		return nil, llm.ErrNotFound.Withf("credential not found for %q", url)
	}

	plaintext, err := encrypt.Decrypt[[]byte](s.passphrase, blob)
	if err != nil {
		return nil, fmt.Errorf("credential decrypt failed for %q: %w", url, err)
	}

	var cred schema.OAuthCredentials
	if err := json.Unmarshal(plaintext, &cred); err != nil {
		return nil, fmt.Errorf("credential unmarshal failed for %q: %w", url, err)
	}
	return &cred, nil
}

// SetCredential stores (or updates) the credential for the given server URL.
func (s *MemoryCredentialStore) SetCredential(_ context.Context, url string, cred schema.OAuthCredentials) error {
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
	s.creds[url] = blob
	return nil
}

// DeleteCredential removes the credential for the given server URL.
func (s *MemoryCredentialStore) DeleteCredential(_ context.Context, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.creds[url]; !ok {
		return llm.ErrNotFound.Withf("credential not found for %q", url)
	}
	delete(s.creds, url)
	return nil
}
