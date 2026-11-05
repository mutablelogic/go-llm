//go:build !client

package main

import (
	"cmp"
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

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// agentStore is a store of the registry's agents in a directory, as a file
// of each agent, as JSON, named by its name, such as agent-segment.json, so
// they're kept when the server restarts. A file is written to a temporary
// file, which is renamed to it, so it's never written partly. An agent's
// serial is when it was stored, in nanoseconds, and more than the last one,
// so an agent which is deleted, and created again, doesn't have a serial it
// had, even when the server restarts. It's safe to use concurrently.
type agentStore struct {
	mu     sync.Mutex
	dir    string
	serial uint64
}

var _ registry.AgentStore = (*agentStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// agentPrefix and agentSuffix are of the name of an agent's file
	agentPrefix = "agent-"
	agentSuffix = ".json"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newAgentStore returns a store of the registry's agents in the directory,
// which is created, when it doesn't exist
func newAgentStore(dir string) (*agentStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, llm.ErrBadParameter.With("missing directory")
	} else if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &agentStore{dir: dir}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create writes an agent with the definition, and returns it, or returns
// ErrConflict when there's an agent with its name
func (s *agentStore) Create(_ context.Context, meta schema.AgentMeta) (schema.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !types.IsIdentifier(meta.Name) {
		return schema.Agent{}, llm.ErrBadParameter.Withf("invalid agent name %q", meta.Name)
	} else if _, err := os.Stat(s.path(meta.Name)); err == nil {
		return schema.Agent{}, llm.ErrConflict.Withf("agent %q exists", meta.Name)
	}
	now := time.Now().UTC()
	agent := schema.Agent{Serial: s.next(now), Created: now, Modified: now, AgentMeta: meta}
	if err := s.write(agent); err != nil {
		return schema.Agent{}, err
	}
	return s.read(meta.Name)
}

// Get returns the agent with the name. Returns ErrNotFound when there's
// none.
func (s *agentStore) Get(_ context.Context, name string) (schema.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(name)
}

// List returns the agents, in order of their names, which is this store's
// order, from the request's offset, and at most its limit of them, and how
// many there are. A file which can't be read as an agent is skipped.
func (s *agentStore) List(_ context.Context, req schema.AgentListRequest) (schema.AgentList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return schema.AgentList{}, err
	}
	var list []schema.Agent
	for _, entry := range entries {
		name, ok := agentName(entry.Name())
		if !ok || entry.IsDir() {
			continue
		}
		agent, err := s.read(name)
		if err != nil {
			continue
		}
		list = append(list, agent)
	}
	slices.SortFunc(list, func(a, b schema.Agent) int { return cmp.Compare(a.Name, b.Name) })
	count := uint(len(list))
	list = list[min(req.Offset, count):]
	if req.Limit != nil {
		list = list[:min(*req.Limit, uint(len(list)))]
	}
	return schema.AgentList{AgentListRequest: req, Count: count, Body: list}, nil
}

// Put replaces the agent with its name, when its serial is that agent's, and
// returns it, with its new serial, and when it was modified. Returns
// ErrNotFound when there's none, and ErrConflict when its serial isn't the
// agent's.
func (s *agentStore) Put(_ context.Context, agent schema.Agent) (schema.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, err := s.read(agent.Name)
	if err != nil {
		return schema.Agent{}, err
	} else if existing.Serial != agent.Serial {
		return schema.Agent{}, llm.ErrConflict.Withf("agent %q is at serial %d, not %d", agent.Name, existing.Serial, agent.Serial)
	}
	now := time.Now().UTC()
	agent.Serial = s.next(now)
	agent.Created = existing.Created
	agent.Modified = now
	if err := s.write(agent); err != nil {
		return schema.Agent{}, err
	}
	return s.read(agent.Name)
}

// Delete deletes the agent with the name, when the serial is its serial, or
// does nothing when there's none. Returns ErrConflict when the serial isn't
// its serial.
func (s *agentStore) Delete(_ context.Context, name string, serial uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, err := s.read(name)
	if errors.Is(err, llm.ErrNotFound) {
		return nil
	} else if err != nil {
		return err
	} else if existing.Serial != serial {
		return llm.ErrConflict.Withf("agent %q is at serial %d, not %d", name, existing.Serial, serial)
	}
	return os.Remove(s.path(name))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// next returns the next serial, which is the time, in nanoseconds, or one
// more than the last, when the time isn't after it. The caller holds the
// lock.
func (s *agentStore) next(now time.Time) uint64 {
	s.serial = max(uint64(now.UnixNano()), s.serial+1)
	return s.serial
}

// path returns the path of the file of the agent with the name
func (s *agentStore) path(name string) string {
	return filepath.Join(s.dir, agentPrefix+name+agentSuffix)
}

// read returns the agent with the name, from its file, or ErrNotFound when
// there's none. The caller holds the lock.
func (s *agentStore) read(name string) (schema.Agent, error) {
	if !types.IsIdentifier(name) {
		return schema.Agent{}, llm.ErrNotFound.Withf("agent %q", name)
	}
	data, err := os.ReadFile(s.path(name))
	if errors.Is(err, fs.ErrNotExist) {
		return schema.Agent{}, llm.ErrNotFound.Withf("agent %q", name)
	} else if err != nil {
		return schema.Agent{}, err
	}
	var agent schema.Agent
	if err := json.Unmarshal(data, &agent); err != nil {
		return schema.Agent{}, llm.ErrInternalServerError.Withf("agent %q: %v", name, err)
	}
	return agent, nil
}

// write writes the agent to its file, by writing a temporary file, which
// only the user can read, and renaming it to the agent's. The caller holds
// the lock.
func (s *agentStore) write(agent schema.Agent) error {
	data, err := json.MarshalIndent(agent, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.dir, "."+agentPrefix+"*.tmp")
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
	return os.Rename(f.Name(), s.path(agent.Name))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// agentName returns the name of the agent whose file has the name, and
// whether it is an agent's file
func agentName(file string) (string, bool) {
	name, ok := strings.CutPrefix(file, agentPrefix)
	if !ok {
		return "", false
	} else if name, ok = strings.CutSuffix(name, agentSuffix); !ok {
		return "", false
	}
	return name, types.IsIdentifier(name)
}
