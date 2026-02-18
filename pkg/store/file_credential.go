package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	encrypt "github.com/mutablelogic/go-llm/pkg/encrypt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// FileCredentialStore is a filesystem-backed implementation of CredentialStore.
// Each credential is stored as a separate binary file in a directory, keyed
// by a SHA-256 hash of the server URL. The file contains the raw encrypted
// blob (salt || nonce || ciphertext) with no wrapper or metadata.
// Credentials are encrypted at rest using AES-256-GCM with a per-entry salt.
// It is safe for concurrent use.
type FileCredentialStore struct {
	mu         sync.RWMutex
	passphrase string
	dir        string
}

var _ schema.CredentialStore = (*FileCredentialStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewFileCredentialStore creates a new file-backed credential store rooted at dir.
// The directory is created (with parents) if it does not already exist.
// The passphrase is used to encrypt and decrypt credentials.
func NewFileCredentialStore(passphrase, dir string) (*FileCredentialStore, error) {
	if err := encrypt.ValidatePassphrase(passphrase); err != nil {
		return nil, err
	}
	if err := ensureDir(dir); err != nil {
		return nil, err
	}
	return &FileCredentialStore{
		passphrase: passphrase,
		dir:        dir,
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// GetCredential retrieves the credential for the given server URL.
func (s *FileCredentialStore) GetCredential(_ context.Context, url string) (*schema.OAuthCredentials, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	blob, err := os.ReadFile(s.path(url))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, llm.ErrNotFound.Withf("credential not found for %q", url)
		}
		return nil, fmt.Errorf("credential read failed for %q: %w", url, err)
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
func (s *FileCredentialStore) SetCredential(_ context.Context, url string, cred schema.OAuthCredentials) error {
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

	if err := os.WriteFile(s.path(url), blob, FilePerm); err != nil {
		return fmt.Errorf("credential write failed for %q: %w", url, err)
	}
	return nil
}

// DeleteCredential removes the credential for the given server URL.
func (s *FileCredentialStore) DeleteCredential(_ context.Context, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path(url)); err != nil {
		if os.IsNotExist(err) {
			return llm.ErrNotFound.Withf("credential not found for %q", url)
		}
		return fmt.Errorf("credential delete failed for %q: %w", url, err)
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// path returns the filesystem path for a given server URL.
func (s *FileCredentialStore) path(url string) string {
	return hashPath(s.dir, url, credExt)
}
