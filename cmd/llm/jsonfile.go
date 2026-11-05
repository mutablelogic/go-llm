//go:build !client

package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// jsonFile is values by key, in a file of JSON, which only the server writes,
// unlike the command's defaults, which a client writes too, as a whole, so
// it would replace what the server wrote since the client read them. A value
// is read from the file each time, and the file is written to a temporary
// file, which is renamed to it, so it's never written partly. It's safe to
// use concurrently.
type jsonFile struct {
	mu   sync.Mutex
	path string
}

var _ defaults = (*jsonFile)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newJSONFile returns the values in the file with the path, whose directory
// is created, when it doesn't exist
func newJSONFile(path string) (*jsonFile, error) {
	if strings.TrimSpace(path) == "" {
		return nil, llm.ErrBadParameter.With("missing path")
	} else if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return &jsonFile{path: path}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Get returns the value with the key, as JSON was decoded into, or nil when
// there's none, or the file can't be read
func (f *jsonFile) Get(key string) any {
	f.mu.Lock()
	defer f.mu.Unlock()
	values, err := f.read()
	if err != nil {
		return nil
	}
	return values[key]
}

// Set sets the value with the key, or removes it, when it's nil, keeping the
// others, and writes the file
func (f *jsonFile) Set(key string, value any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	values, err := f.read()
	if err != nil {
		return err
	}
	if value == nil {
		delete(values, key)
	} else {
		values[key] = value
	}
	return f.write(values)
}

// Exists returns whether the file exists
func (f *jsonFile) Exists() bool {
	_, err := os.Stat(f.path)
	return err == nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// read returns the values in the file, or none when there's no file. The
// caller holds the lock.
func (f *jsonFile) read() (map[string]any, error) {
	values := make(map[string]any)
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

// write writes the values to a temporary file, which only the user can
// read, and renames it to the file. The caller holds the lock.
func (f *jsonFile) write(values map[string]any) error {
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.path), "."+filepath.Base(f.path)+"-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	} else if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	} else if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), f.path)
}
