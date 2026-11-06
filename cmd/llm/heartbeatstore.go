//go:build !client

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	heartbeat "github.com/mutablelogic/go-llm/heartbeat"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// heartbeatStore is a store of the heartbeats in a directory, as a file of
// each, as JSON, named by its identifier, such as
// heartbeat-3b241101-e2bb-4255-8caf-4136c566a962.json. Each change is made
// with a lock file, heartbeat.lock, locked, so processes on the same host
// which share the directory change a heartbeat one at a time, and only one
// of them fires it. A file is written to a temporary file, which is renamed
// to it, so it's never written partly. It's safe to use concurrently.
type heartbeatStore struct {
	mu  sync.Mutex
	dir string
}

var _ heartbeat.Store = (*heartbeatStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// heartbeatPrefix and heartbeatSuffix are of the name of a heartbeat's
	// file
	heartbeatPrefix = "heartbeat-"
	heartbeatSuffix = ".json"

	// heartbeatLock is the name of the file which is locked as the
	// heartbeats are changed
	heartbeatLock = "heartbeat.lock"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newHeartbeatStore returns a store of the heartbeats in the directory,
// which is created, when it doesn't exist
func newHeartbeatStore(dir string) (*heartbeatStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, llm.ErrBadParameter.With("missing directory")
	} else if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &heartbeatStore{dir: dir}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (s *heartbeatStore) Create(_ context.Context, h schema.Heartbeat) error {
	return s.locked(func() error {
		if _, err := os.Stat(s.path(h.ID)); err == nil {
			return llm.ErrConflict.Withf("heartbeat %q exists", h.ID)
		}
		h.Next = nil
		return s.write(h)
	})
}

func (s *heartbeatStore) Get(_ context.Context, id uuid.UUID) (result schema.Heartbeat, err error) {
	err = s.locked(func() error {
		result, err = s.read(id)
		return err
	})
	return result, err
}

// List returns the heartbeats, skipping a file which can't be read as one
func (s *heartbeatStore) List(context.Context) (result []schema.Heartbeat, err error) {
	err = s.locked(func() error {
		entries, err := os.ReadDir(s.dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			id, ok := heartbeatID(entry.Name())
			if !ok || entry.IsDir() {
				continue
			}
			if h, err := s.read(id); err == nil {
				result = append(result, h)
			}
		}
		return nil
	})
	return result, err
}

func (s *heartbeatStore) Update(_ context.Context, id uuid.UUID, fn func(*schema.Heartbeat) error) (result schema.Heartbeat, err error) {
	err = s.locked(func() error {
		h, err := s.read(id)
		if err != nil {
			return err
		} else if err := fn(&h); err != nil {
			return err
		}
		h.ID, h.Next = id, nil
		if err := s.write(h); err != nil {
			return err
		}
		result = h
		return nil
	})
	return result, err
}

func (s *heartbeatStore) Delete(_ context.Context, id uuid.UUID, check func(schema.Heartbeat) error) (result schema.Heartbeat, err error) {
	err = s.locked(func() error {
		h, err := s.read(id)
		if err != nil {
			return err
		} else if check != nil {
			if err := check(h); err != nil {
				return err
			}
		}
		if err := os.Remove(s.path(id)); err != nil {
			return err
		}
		result = h
		return nil
	})
	return result, err
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// locked calls the function, with the store's lock, and the lock file's,
// so no other process which shares the directory changes the heartbeats
// until it returns
func (s *heartbeatStore) locked(fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(filepath.Join(s.dir, heartbeatLock), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := lockFile(f); err != nil {
		return err
	}
	defer unlockFile(f)
	return fn()
}

// path returns the path of the file of the heartbeat with the identifier
func (s *heartbeatStore) path(id uuid.UUID) string {
	return filepath.Join(s.dir, heartbeatPrefix+id.String()+heartbeatSuffix)
}

// read returns the heartbeat with the identifier, from its file, or
// ErrNotFound when there's none. The caller holds the locks.
func (s *heartbeatStore) read(id uuid.UUID) (schema.Heartbeat, error) {
	data, err := os.ReadFile(s.path(id))
	if errors.Is(err, fs.ErrNotExist) {
		return schema.Heartbeat{}, llm.ErrNotFound.Withf("heartbeat %q", id)
	} else if err != nil {
		return schema.Heartbeat{}, err
	}
	var h schema.Heartbeat
	if err := json.Unmarshal(data, &h); err != nil {
		return schema.Heartbeat{}, llm.ErrInternalServerError.Withf("heartbeat %q: %v", id, err)
	}
	h.ID, h.Next = id, nil
	return h, nil
}

// write writes the heartbeat to its file, by writing a temporary file,
// which only the user can read, and renaming it to the heartbeat's. The
// caller holds the locks.
func (s *heartbeatStore) write(h schema.Heartbeat) error {
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.dir, "."+heartbeatPrefix+"*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	} else if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	} else if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.path(h.ID))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// heartbeatID returns the identifier of the heartbeat whose file has the
// name, and whether it is a heartbeat's file
func heartbeatID(name string) (uuid.UUID, bool) {
	value, ok := strings.CutPrefix(name, heartbeatPrefix)
	if !ok {
		return uuid.Nil(), false
	} else if value, ok = strings.CutSuffix(value, heartbeatSuffix); !ok {
		return uuid.Nil(), false
	}
	id, err := uuid.Parse(value)
	return id, err == nil
}
