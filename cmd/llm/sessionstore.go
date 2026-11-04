//go:build !client

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// sessionStore is a store of the registry's sessions in a directory, as a
// file of each session, with its messages, as JSON, named by its identifier,
// such as session-3b241101-e2bb-4255-8caf-4136c566a962.json, so they're kept
// when the server restarts, and a change to a session writes only its file.
// A file is written to a temporary file, which is renamed to it, so it's
// never written partly. It's safe to use concurrently.
type sessionStore struct {
	mu  sync.Mutex
	dir string
}

var _ registry.SessionStore = (*sessionStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// sessionPrefix and sessionSuffix are of the name of a session's file
	sessionPrefix = "session-"
	sessionSuffix = ".json"

	// sessionsKey is the key of the defaults which the sessions were kept in,
	// which they're moved from to files
	sessionsKey = "llm.sessions"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newSessionStore returns a store of the registry's sessions in the
// directory, which is created, when it doesn't exist
func newSessionStore(dir string) (*sessionStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, llm.ErrBadParameter.With("missing directory")
	} else if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &sessionStore{dir: dir}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create writes a session with the settings, and no messages, with a new
// identifier, which is a random UUID, and a serial of one, and returns it
func (s *sessionStore) Create(_ context.Context, meta schema.SessionMeta) (schema.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	session := schema.Session{ID: uuid.New(), Serial: 1, Created: now, Modified: now, SessionMeta: meta}
	if _, err := os.Stat(s.path(session.ID)); err == nil {
		return schema.Session{}, llm.ErrConflict.Withf("session %q exists", session.ID)
	}
	if err := s.write(session); err != nil {
		return schema.Session{}, err
	}
	return s.read(session.ID)
}

// Get returns the session with the identifier, with its messages. Returns
// ErrNotFound when there's none.
func (s *sessionStore) Get(_ context.Context, id uuid.UUID) (schema.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(id)
}

// List returns the sessions, without their messages, most recently modified
// first, from the request's offset, and at most its limit of them, and how
// many there are. A file which can't be read as a session is skipped.
func (s *sessionStore) List(_ context.Context, req schema.SessionListRequest) (schema.SessionList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return schema.SessionList{}, err
	}
	var list []schema.Session
	for _, entry := range entries {
		id, ok := sessionID(entry.Name())
		if !ok || entry.IsDir() {
			continue
		}
		session, err := s.read(id)
		if err != nil {
			continue
		}
		session.Messages = nil
		list = append(list, session)
	}
	slices.SortFunc(list, func(a, b schema.Session) int {
		if c := b.Modified.Compare(a.Modified); c != 0 {
			return c
		}
		return a.ID.Compare(b.ID)
	})
	count := uint(len(list))
	list = list[min(req.Offset, count):]
	if req.Limit != nil {
		list = list[:min(*req.Limit, uint(len(list)))]
	}
	return schema.SessionList{SessionListRequest: req, Count: count, Body: list}, nil
}

// Put replaces the session with its identifier, when its serial is that
// session's, and returns it, with its serial one more, and when it was
// modified, keeping when it was created. Returns ErrNotFound when there's
// none, and ErrConflict when its serial isn't the session's.
func (s *sessionStore) Put(_ context.Context, session schema.Session) (schema.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, err := s.read(session.ID)
	if err != nil {
		return schema.Session{}, err
	} else if existing.Serial != session.Serial {
		return schema.Session{}, llm.ErrConflict.Withf("session %q is at serial %d, not %d", session.ID, existing.Serial, session.Serial)
	}
	session.Serial++
	session.Created = existing.Created
	session.Modified = time.Now().UTC()
	if err := s.write(session); err != nil {
		return schema.Session{}, err
	}
	return s.read(session.ID)
}

// Delete deletes the session with the identifier, when the serial is its
// serial, or does nothing when there's none. Returns ErrConflict when the
// serial isn't its serial.
func (s *sessionStore) Delete(_ context.Context, id uuid.UUID, serial uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, err := s.read(id)
	if errors.Is(err, llm.ErrNotFound) {
		return nil
	} else if err != nil {
		return err
	} else if existing.Serial != serial {
		return llm.ErrConflict.Withf("session %q is at serial %d, not %d", id, existing.Serial, serial)
	}
	return os.Remove(s.path(id))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// path returns the path of the file of the session with the identifier
func (s *sessionStore) path(id uuid.UUID) string {
	return filepath.Join(s.dir, sessionPrefix+id.String()+sessionSuffix)
}

// read returns the session with the identifier, from its file, which is the
// caller's to change, or ErrNotFound when there's none. The caller holds the
// lock.
func (s *sessionStore) read(id uuid.UUID) (schema.Session, error) {
	data, err := os.ReadFile(s.path(id))
	if errors.Is(err, fs.ErrNotExist) {
		return schema.Session{}, llm.ErrNotFound.Withf("session %q", id)
	} else if err != nil {
		return schema.Session{}, err
	}
	var session schema.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return schema.Session{}, llm.ErrInternalServerError.Withf("session %q: %v", id, err)
	}
	return session, nil
}

// write writes the session to its file, by writing a temporary file, which
// only the user can read, and renaming it to the session's, so the file is
// never written partly. The caller holds the lock.
func (s *sessionStore) write(session schema.Session) error {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.dir, "."+sessionPrefix+"*.tmp")
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
	return os.Rename(f.Name(), s.path(session.ID))
}

// migrate moves the sessions which the defaults have, as they were kept
// before they were kept in files, to files, which aren't replaced, when
// they exist, and removes them from the defaults
func (s *sessionStore) migrate(d defaults) error {
	value := d.Get(sessionsKey)
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var list []schema.Session
	if err := json.Unmarshal(data, &list); err != nil {
		return llm.ErrInternalServerError.Withf("%s: %v", sessionsKey, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, session := range list {
		if _, err := os.Stat(s.path(session.ID)); err == nil {
			continue
		} else if err := s.write(session); err != nil {
			return err
		}
	}
	return d.Set(sessionsKey, nil)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// sessionID returns the identifier of the session whose file has the name,
// and whether it is a session's file
func sessionID(name string) (uuid.UUID, bool) {
	value, ok := strings.CutPrefix(name, sessionPrefix)
	if !ok {
		return uuid.Nil(), false
	} else if value, ok = strings.CutSuffix(value, sessionSuffix); !ok {
		return uuid.Nil(), false
	}
	id, err := uuid.Parse(value)
	return id, err == nil
}
