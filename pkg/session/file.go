package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	jsonExt              = ".json"
	DirPerm  os.FileMode = 0o700 // Directory permission for session store
	FilePerm os.FileMode = 0o600 // File permission for session files
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// FileStore is a file-backed implementation of Store.
// Each session is stored as {id}.json in a directory.
// It is safe for concurrent use.
type FileStore struct {
	mu  sync.RWMutex
	dir string
}

var _ Store = (*FileStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewFileStore creates a new file-backed session store in the given directory.
// The directory is created if it does not exist.
func NewFileStore(dir string) (*FileStore, error) {
	if dir == "" {
		return nil, llm.ErrBadParameter.With("directory is required")
	}
	if err := os.MkdirAll(dir, DirPerm); err != nil {
		return nil, llm.ErrInternalServerError.Withf("mkdir: %v", err)
	}
	return &FileStore{dir: dir}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create creates a new session with a unique ID, writes it to disk,
// and returns it.
func (f *FileStore) Create(_ context.Context, name string, model schema.Model) (*Session, error) {
	if model.Name == "" {
		return nil, llm.ErrBadParameter.With("model name is required")
	}

	now := time.Now()
	s := &Session{
		ID:       uuid.New().String(),
		Name:     name,
		Model:    model,
		Messages: make(schema.Conversation, 0),
		Created:  now,
		Modified: now,
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.write(s); err != nil {
		return nil, err
	}
	return s, nil
}

// Get retrieves a session by ID from disk.
func (f *FileStore) Get(_ context.Context, id string) (*Session, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.read(id)
}

// List returns all sessions from disk, ordered by last modified time
// (most recent first).
func (f *FileStore) List(_ context.Context, opts ...opt.Opt) ([]*Session, error) {
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return nil, llm.ErrInternalServerError.Withf("readdir: %v", err)
	}

	result := make([]*Session, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), jsonExt) {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), jsonExt)
		s, err := f.read(id)
		if err != nil {
			continue // skip corrupt files
		}
		result = append(result, s)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Modified.After(result[j].Modified)
	})

	if limit := o.GetUint(opt.LimitKey); limit > 0 && int(limit) < len(result) {
		result = result[:limit]
	}

	return result, nil
}

// Delete removes a session file by ID.
func (f *FileStore) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := f.path(id)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return llm.ErrNotFound.Withf("session %q", id)
	}
	if err := os.Remove(path); err != nil {
		return llm.ErrInternalServerError.Withf("remove: %v", err)
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// path returns the file path for a session ID.
func (f *FileStore) path(id string) string {
	return filepath.Join(f.dir, id+jsonExt)
}

// write serialises a session to its JSON file.
func (f *FileStore) write(s *Session) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return llm.ErrInternalServerError.Withf("marshal: %v", err)
	}
	if err := os.WriteFile(f.path(s.ID), data, FilePerm); err != nil {
		return llm.ErrInternalServerError.Withf("write: %v", err)
	}
	return nil
}

// read deserialises a session from its JSON file.
func (f *FileStore) read(id string) (*Session, error) {
	data, err := os.ReadFile(f.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, llm.ErrNotFound.Withf("session %q", id)
		}
		return nil, llm.ErrInternalServerError.Withf("read: %v", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, llm.ErrInternalServerError.Withf("unmarshal: %v", err)
	}
	return &s, nil
}

// Write persists a session's current state to disk.
// This is called after mutations (e.g. Append) to keep the file in sync.
func (f *FileStore) Write(s *Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.write(s)
}
