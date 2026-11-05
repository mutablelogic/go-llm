//go:build !client

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	memory "github.com/mutablelogic/go-llm/memory"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	yaml "go.yaml.in/yaml/v3"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// memoryStore keeps the memory's details in a file of JSON, under the
// details key, and its memories in files of Markdown, one each, in the
// directory, named by their keys, such as memory-kitchen_lights.md, whose
// front matter is their description, and when they were remembered, and
// changed, so they can be read, and edited, by hand. A memory is written to
// a temporary file, which is renamed to it, so it's never written partly.
type memoryStore struct {
	file *jsonFile
	mu   sync.Mutex
	dir  string
}

// memoryFrontMatter is the front matter of a memory's file
type memoryFrontMatter struct {
	Description string    `yaml:"description"`
	Created     time.Time `yaml:"created"`
	Modified    time.Time `yaml:"modified,omitempty"`
}

var _ memory.Store = (*memoryStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// memoryFile is the name of the file which the server keeps the
	// memory in, in the directory of the command's defaults
	memoryFile = "memory.json"

	// detailsKey is the key of the details, in the file
	detailsKey = "details"

	// memoryPrefix and memorySuffix are the start and end of the name of a
	// memory's file, either side of its key
	memoryPrefix = "memory-"
	memorySuffix = ".md"
)

// frontMatter is the line before, and after, a memory's front matter
var frontMatter = []byte("---\n")

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newMemoryStore returns the store of the memory, with the details in the
// file, and the memories in files in the directory
func newMemoryStore(file *jsonFile, dir string) *memoryStore {
	return &memoryStore{file: file, dir: dir}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Details returns the details in the file, which are all not known, when
// there's no file, or no details in it. Returns an error when the file
// can't be read, or its details aren't valid.
func (s *memoryStore) Details(context.Context) (schema.Details, error) {
	var details schema.Details
	value, err := s.file.Lookup(detailsKey)
	if err != nil || value == nil {
		return details, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return details, err
	} else if err := json.Unmarshal(data, &details); err != nil {
		return details, err
	}
	return details, details.Validate()
}

// PutDetails writes the details to the file, keeping what else is in it
func (s *memoryStore) PutDetails(_ context.Context, details schema.Details) error {
	if err := details.Validate(); err != nil {
		return err
	}
	return s.file.Set(detailsKey, details)
}

// ListMemories returns the memories which match the query, the most
// recently changed first. A file which isn't a memory, such as one edited by
// hand, which isn't valid, is left out.
func (s *memoryStore) ListMemories(_ context.Context, q string) ([]schema.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var result []schema.Memory
	for _, entry := range entries {
		key, ok := memoryKey(entry.Name())
		if !ok || entry.IsDir() {
			continue
		}
		if memory, err := s.read(key); err == nil && memory.Matches(q) {
			result = append(result, memory)
		}
	}
	memory.SortMemories(result)
	return result, nil
}

// GetMemory returns the memory with the key, from its file, or ErrNotFound
// when there's none
func (s *memoryStore) GetMemory(_ context.Context, key string) (schema.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(key)
}

// PutMemory writes the memory to its file
func (s *memoryStore) PutMemory(_ context.Context, m schema.Memory) error {
	meta, err := m.MemoryMeta.Validate()
	if err != nil {
		return err
	}
	m.MemoryMeta = meta
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(m)
}

// DeleteMemory deletes the memory's file, or returns ErrNotFound when
// there's none
func (s *memoryStore) DeleteMemory(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !types.IsIdentifier(key) {
		return llm.ErrNotFound.Withf("memory %q", key)
	}
	err := os.Remove(s.path(key))
	if errors.Is(err, fs.ErrNotExist) {
		return llm.ErrNotFound.Withf("memory %q", key)
	}
	return err
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// path returns the path of the file of the memory with the key
func (s *memoryStore) path(key string) string {
	return filepath.Join(s.dir, memoryPrefix+key+memorySuffix)
}

// read returns the memory with the key, from its file, or ErrNotFound when
// there's none. The caller holds the lock.
func (s *memoryStore) read(key string) (schema.Memory, error) {
	if !types.IsIdentifier(key) {
		return schema.Memory{}, llm.ErrNotFound.Withf("memory %q", key)
	}
	data, err := os.ReadFile(s.path(key))
	if errors.Is(err, fs.ErrNotExist) {
		return schema.Memory{}, llm.ErrNotFound.Withf("memory %q", key)
	} else if err != nil {
		return schema.Memory{}, err
	}
	m, err := parseMemory(key, data)
	if err != nil {
		return schema.Memory{}, llm.ErrInternalServerError.Withf("memory %q: %v", key, err)
	}
	return m, nil
}

// write writes the memory to its file, by writing a temporary file, which
// only the user can read, and renaming it to the memory's. The caller holds
// the lock.
func (s *memoryStore) write(m schema.Memory) error {
	data, err := formatMemory(m)
	if err != nil {
		return err
	} else if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(s.dir, "."+memoryPrefix+"*.tmp")
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
	return os.Rename(f.Name(), s.path(m.Key))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// memoryKey returns the key of the memory whose file has the name, and
// whether it is a memory's file
func memoryKey(file string) (string, bool) {
	key, ok := strings.CutPrefix(file, memoryPrefix)
	if !ok {
		return "", false
	} else if key, ok = strings.CutSuffix(key, memorySuffix); !ok {
		return "", false
	}
	return key, types.IsIdentifier(key)
}

// formatMemory returns the memory as Markdown, with its front matter
func formatMemory(m schema.Memory) ([]byte, error) {
	front, err := yaml.Marshal(memoryFrontMatter{Description: m.Description, Created: m.Created, Modified: m.Modified})
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.Write(frontMatter)
	b.Write(front)
	b.Write(frontMatter)
	b.WriteString("\n" + m.Body + "\n")
	return b.Bytes(), nil
}

// parseMemory returns the memory with the key, from its Markdown, with its
// front matter, or an error when it isn't valid
func parseMemory(key string, data []byte) (schema.Memory, error) {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	rest, ok := bytes.CutPrefix(data, frontMatter)
	if !ok {
		return schema.Memory{}, errors.New("missing front matter")
	}
	front, body, ok := bytes.Cut(rest, append([]byte("\n"), frontMatter...))
	if !ok {
		return schema.Memory{}, errors.New("front matter isn't ended")
	}
	var meta memoryFrontMatter
	if err := yaml.Unmarshal(front, &meta); err != nil {
		return schema.Memory{}, err
	}
	m := schema.Memory{MemoryMeta: schema.MemoryMeta{Key: key, Description: meta.Description, Body: string(body)}, Created: meta.Created, Modified: meta.Modified}
	valid, err := m.MemoryMeta.Validate()
	if err != nil {
		return schema.Memory{}, err
	}
	m.MemoryMeta = valid
	return m, nil
}
