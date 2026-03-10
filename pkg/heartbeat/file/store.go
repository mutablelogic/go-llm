package file

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	llm "github.com/mutablelogic/go-llm"
	heartbeat "github.com/mutablelogic/go-llm/pkg/heartbeat"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

const (
	jsonExt              = ".json"
	dirPerm  os.FileMode = 0o700
	filePerm os.FileMode = 0o600
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Store is a file-backed, concurrency-safe store for Heartbeat records.
// Each heartbeat is persisted as {id}.json inside the configured directory.
type Store struct {
	mu    sync.RWMutex
	dir   string
	NowFn func() time.Time // if non-nil, overrides time.Now (for testing)
}

func (s *Store) now() time.Time {
	if s.NowFn != nil {
		return s.NowFn()
	}
	return time.Now()
}

var _ heartbeat.Store = (*Store)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewStore creates a new file-backed heartbeat store rooted at dir.
// The directory is created if it does not already exist.
func NewStore(dir string) (*Store, error) {
	if dir == "" {
		return nil, llm.ErrBadParameter.With("directory is required")
	}
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, llm.ErrInternalServerError.Withf("mkdir: %v", err)
	}
	return &Store{dir: dir}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create persists a new Heartbeat derived from the supplied fields.
// A unique ID and timestamps are assigned automatically.
func (s *Store) Create(ctx context.Context, meta heartbeat.HeartbeatMeta) (*heartbeat.Heartbeat, error) {
	if meta.Message == "" {
		return nil, llm.ErrBadParameter.With("message is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	h := &heartbeat.Heartbeat{
		ID:            uuid.New().String(),
		HeartbeatMeta: meta,
		Created:       now,
	}
	if err := s.write(h); err != nil {
		return nil, err
	}
	return h, nil
}

// Get retrieves a single Heartbeat by ID. Returns ErrNotFound if absent.
func (s *Store) Get(ctx context.Context, id string) (*heartbeat.Heartbeat, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.read(id)
}

// Delete removes the heartbeat file for the given ID.
// Returns ErrNotFound if no such heartbeat exists.
func (s *Store) Delete(ctx context.Context, id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.path(id)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return llm.ErrNotFound.Withf("heartbeat %q", id)
	}
	if err := os.Remove(path); err != nil {
		return llm.ErrInternalServerError.Withf("remove: %v", err)
	}
	return nil
}

// List returns all heartbeats in the store.
// When includeFired is false, already-fired heartbeats are excluded.
func (s *Store) List(ctx context.Context, includeFired bool) ([]*heartbeat.Heartbeat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids, err := s.listIDs()
	if err != nil {
		return nil, err
	}
	result := make([]*heartbeat.Heartbeat, 0, len(ids))
	for _, id := range ids {
		h, err := s.read(id)
		if err != nil {
			continue // skip corrupt files
		}
		if !includeFired && h.Fired {
			continue
		}
		result = append(result, h)
	}
	return result, nil
}

// Update applies non-zero fields from message and schedule to the heartbeat
// identified by id. A non-nil schedule replaces the existing one and resets
// the Fired flag; nil schedule leaves it unchanged.
func (s *Store) Update(ctx context.Context, id string, meta heartbeat.HeartbeatMeta) (*heartbeat.Heartbeat, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := s.read(id)
	if err != nil {
		return nil, err
	}
	if message := strings.TrimSpace(meta.Message); message != "" {
		h.Message = message
	}
	if !meta.Schedule.IsZero() {
		h.Schedule = meta.Schedule
		h.Fired = false // rescheduling reactivates the heartbeat
	}
	h.Modified = types.Ptr(time.Now())
	if err := s.write(h); err != nil {
		return nil, err
	}
	return h, nil
}

// Next returns all heartbeats that are due now and atomically marks them fired
// (under the write lock — the file store has no transaction support).
func (s *Store) Next(ctx context.Context) ([]*heartbeat.Heartbeat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids, err := s.listIDs()
	if err != nil {
		return nil, err
	}
	now := s.now()
	var result []*heartbeat.Heartbeat
	for _, id := range ids {
		h, err := s.read(id)
		if err != nil {
			continue
		}
		if h.Fired {
			continue
		}
		base := h.Created
		if h.LastFired != nil {
			base = h.LastFired.Add(time.Minute)
		}
		next := h.Schedule.Next(base)
		if !next.IsZero() && !next.After(now) {
			if h.Schedule.Year != nil {
				h.Fired = true
			} else {
				h.LastFired = &now
			}
			h.Modified = &now
			if err := s.write(h); err != nil {
				return result, err
			}
			result = append(result, h)
		}
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// validateID rejects any id that is not a valid UUID, preventing path-traversal
// attacks where a caller passes a value containing '/' or '..' components.
func validateID(id string) error {
	if !types.IsUUID(id) {
		return llm.ErrBadParameter.Withf("invalid heartbeat id %q", id)
	}
	return nil
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+jsonExt)
}

func (s *Store) write(h *heartbeat.Heartbeat) error {
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return llm.ErrInternalServerError.Withf("marshal: %v", err)
	}
	if err := os.WriteFile(s.path(h.ID), data, filePerm); err != nil {
		return llm.ErrInternalServerError.Withf("write: %v", err)
	}
	return nil
}

func (s *Store) read(id string) (*heartbeat.Heartbeat, error) {
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, llm.ErrNotFound.Withf("heartbeat %q", id)
		}
		return nil, llm.ErrInternalServerError.Withf("read: %v", err)
	}
	var h heartbeat.Heartbeat
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, llm.ErrInternalServerError.Withf("unmarshal %s: %v", id, err)
	}
	return &h, nil
}

func (s *Store) listIDs() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, llm.ErrInternalServerError.Withf("readdir: %v", err)
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), jsonExt) {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), jsonExt))
	}
	return ids, nil
}
