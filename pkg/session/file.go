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
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
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

var _ schema.Store = (*FileStore)(nil)

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
func (f *FileStore) Create(_ context.Context, meta schema.SessionMeta) (*schema.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if meta.Model == "" {
		return nil, llm.ErrBadParameter.With("model name is required")
	}

	now := time.Now()
	s := &schema.Session{
		ID:          uuid.New().String(),
		SessionMeta: meta,
		Messages:    make(schema.Conversation, 0),
		Created:     now,
		Modified:    now,
	}
	if err := f.write(s); err != nil {
		return nil, err
	}
	return s, nil
}

// Get retrieves a session by ID from disk.
func (f *FileStore) Get(_ context.Context, id string) (*schema.Session, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.read(id)
}

// List returns sessions from disk, ordered by last modified time
// (most recent first), with pagination support.
func (f *FileStore) List(_ context.Context, req schema.ListSessionRequest) (*schema.ListSessionResponse, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return nil, llm.ErrInternalServerError.Withf("readdir: %v", err)
	}

	result := make([]*schema.Session, 0, len(entries))
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

	// Paginate
	total := uint(len(result))
	start := req.Offset
	if start > total {
		start = total
	}
	end := start + types.Value(req.Limit)
	if req.Limit == nil || end > total {
		end = total
	}

	return &schema.ListSessionResponse{
		Count:  total,
		Offset: req.Offset,
		Limit:  req.Limit,
		Body:   result[start:end],
	}, nil
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
func (f *FileStore) write(s *schema.Session) error {
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
func (f *FileStore) read(id string) (*schema.Session, error) {
	data, err := os.ReadFile(f.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, llm.ErrNotFound.Withf("session %q", id)
		}
		return nil, llm.ErrInternalServerError.Withf("read: %v", err)
	}
	var s schema.Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, llm.ErrInternalServerError.Withf("unmarshal: %v", err)
	}
	return &s, nil
}

// Write persists a session's current state to disk.
// This is called after mutations (e.g. Append) to keep the file in sync.
func (f *FileStore) Write(s *schema.Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.write(s)
}

// Update applies non-zero fields from meta to the session identified by id,
// persists the result to disk, and returns the updated session.
func (f *FileStore) Update(_ context.Context, id string, meta schema.SessionMeta) (*schema.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	s, err := f.read(id)
	if err != nil {
		return nil, err
	}

	if meta.Name != "" {
		s.Name = meta.Name
	}
	if meta.Model != "" {
		s.Model = meta.Model
	}
	if meta.Provider != "" {
		s.Provider = meta.Provider
	}
	if meta.SystemPrompt != "" {
		s.SystemPrompt = meta.SystemPrompt
	}
	if meta.Format != nil {
		s.Format = meta.Format
	}
	if meta.Thinking != nil {
		s.Thinking = meta.Thinking
	}
	if meta.ThinkingBudget > 0 {
		s.ThinkingBudget = meta.ThinkingBudget
	}
	s.Modified = time.Now()

	if err := f.write(s); err != nil {
		return nil, err
	}
	return s, nil
}
