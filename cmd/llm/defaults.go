package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Defaults is a persistent key-value store backed by a JSON file on disk.
type Defaults struct {
	mu   sync.RWMutex
	path string
	data map[string]any
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewDefaults creates a Defaults store at the given file path.
// If the file exists, its contents are loaded; otherwise the store starts empty.
func NewDefaults(path string) (*Defaults, error) {
	d := &Defaults{
		path: path,
		data: make(map[string]any),
	}

	// Load existing file (ignore if it doesn't exist)
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		if err := json.NewDecoder(f).Decode(&d.data); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	return d, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Get retrieves a value by key. Returns nil if the key does not exist.
func (d *Defaults) Get(key string) any {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.data[key]
}

// GetString retrieves a string value by key. Returns empty string if the key
// does not exist or the value is not a string.
func (d *Defaults) GetString(key string) string {
	v, _ := d.Get(key).(string)
	return v
}

// Set stores a value by key and persists the store to disk.
// Pass nil to remove a key.
func (d *Defaults) Set(key string, value any) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if value == nil {
		delete(d.data, key)
	} else {
		d.data[key] = value
	}
	return d.save()
}

// Keys returns all keys in the store.
func (d *Defaults) Keys() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	keys := make([]string, 0, len(d.data))
	for k := range d.data {
		keys = append(keys, k)
	}
	return keys
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// save writes the store to disk as indented JSON, creating parent directories
// as needed.
func (d *Defaults) save() error {
	if err := os.MkdirAll(filepath.Dir(d.path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(d.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(d.path, data, 0600)
}
