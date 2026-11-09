// Package file is the provider of credentials which are environment
// variables, or are kept in a file of JSON, which only its user can read,
// so credentials which are set, such as how a connector's server is signed
// in to, are kept once the process ends
package file

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	credentials "github.com/mutablelogic/go-llm/auth/credentials"
	env "github.com/mutablelogic/go-llm/auth/credentials/env"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// File is the provider of credentials which are environment variables, or
// are in its file. The file is read each time, and written to a temporary
// file, which is renamed to it, so it's never written partly. It's safe to
// use concurrently.
type File struct {
	mu   sync.Mutex
	path string
	env  credentials.Provider
}

var _ credentials.Provider = (*File)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the provider of credentials which are environment variables,
// or are in the file with the path, whose directory is created, when it
// doesn't exist
func New(path string) (*File, error) {
	if strings.TrimSpace(path) == "" {
		return nil, llm.ErrBadParameter.With("missing path")
	} else if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return &File{path: path, env: env.New()}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Get returns the environment variable with the key, when it's set, or
// else the credential in the file, or ErrNotFound when there's neither
func (f *File) Get(ctx context.Context, key string) (string, error) {
	if value, err := f.env.Get(ctx, key); err == nil {
		return value, nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	values, err := f.read()
	if err != nil {
		return "", err
	}
	if value, exists := values[key]; exists {
		return value, nil
	}
	return "", llm.ErrNotFound.Withf("credential %q isn't set", key)
}

// Set sets the credential with the key in the file, keeping the others
func (f *File) Set(_ context.Context, key, value string) error {
	if key == "" {
		return llm.ErrBadParameter.With("missing credential key")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	values, err := f.read()
	if err != nil {
		return err
	}
	values[key] = value
	return f.write(values)
}

// Delete deletes the credential with the key from the file, or does
// nothing when there's none. An environment variable isn't changed.
func (f *File) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	values, err := f.read()
	if err != nil {
		return err
	} else if _, exists := values[key]; !exists {
		return nil
	}
	delete(values, key)
	return f.write(values)
}

// Expand replaces each placeholder with the credential it names, which
// must exist, and returns ErrNotFound, with each which doesn't
func (f *File) Expand(ctx context.Context, value string) (string, error) {
	return credentials.Expand(ctx, f.Get, value)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// read returns the credentials in the file, or none when there's no file.
// The caller holds the lock.
func (f *File) read() (map[string]string, error) {
	values := make(map[string]string)
	data, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return values, nil
	} else if err != nil {
		return nil, err
	} else if err := json.Unmarshal(data, &values); err != nil {
		return nil, llm.ErrInternalServerError.Withf("%s: %v", f.path, err)
	}
	return values, nil
}

// write writes the credentials to a temporary file, which only the user
// can read, and renames it to the file. The caller holds the lock.
func (f *File) write(values map[string]string) error {
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.path), "."+filepath.Base(f.path)+"-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), f.path)
}
