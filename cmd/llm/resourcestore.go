//go:build !client

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	resource "github.com/mutablelogic/go-llm/resource"
	schema "github.com/mutablelogic/go-llm/resource/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// resourceStore is a store of the resources in a directory: their content,
// in a directory of its own, as a file of each, named by its hash, and each
// resource, as a file of JSON, named by its hash and session, such as
// resource-<hash>-3b241101-e2bb-4255-8caf-4136c566a962.json, or
// resource-<hash>.json, for one on its own. Content is written to a
// temporary file, which is renamed to its hash, once it's written whole, so
// it's never kept partly, and the temporary files of content which wasn't,
// such as when the server stopped, are removed when the store is opened. A
// resource's file is written the same way, and its temporary files are
// removed too. It's safe to use concurrently.
type resourceStore struct {
	mu  sync.Mutex
	dir string
}

var _ resource.Store = (*resourceStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// resourcePrefix and resourceSuffix are of the name of a resource's file
	resourcePrefix = "resource-"
	resourceSuffix = ".json"

	// contentDir is the directory of the content, in the store's directory,
	// and contentTemp the prefix of a temporary file of content which is
	// written
	contentDir  = "content"
	contentTemp = ".upload-"

	// resourceTemp is the prefix of a temporary file of a resource which is
	// written
	resourceTemp = ".resource-"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newResourceStore returns a store of the resources in the directory, which
// is created, when it doesn't exist, without the temporary files of content
// which wasn't written whole
func newResourceStore(dir string) (*resourceStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, llm.ErrBadParameter.With("missing directory")
	} else if err := os.MkdirAll(filepath.Join(dir, contentDir), 0o700); err != nil {
		return nil, err
	}
	for _, pattern := range []string{filepath.Join(dir, contentDir, contentTemp+"*"), filepath.Join(dir, resourceTemp+"*")} {
		temps, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, temp := range temps {
			_ = os.Remove(temp)
		}
	}
	return &resourceStore{dir: dir}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - CONTENT

func (s *resourceStore) Write(ctx context.Context, r io.Reader) (_ string, _ int64, err error) {
	// Write the content to a temporary file, as it's read, with its hash
	temp, err := os.CreateTemp(filepath.Join(s.dir, contentDir), contentTemp+"*")
	if err != nil {
		return "", 0, err
	}
	defer func() {
		if err != nil {
			_ = temp.Close()
			_ = os.Remove(temp.Name())
		}
	}()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(temp, hash), contextReader{ctx, r})
	if err != nil {
		return "", 0, err
	} else if err := temp.Sync(); err != nil {
		return "", 0, err
	} else if err := temp.Close(); err != nil {
		return "", 0, err
	}

	// Rename it to its hash, or remove it, when the content is kept already
	sum := hex.EncodeToString(hash.Sum(nil))
	if _, err := os.Stat(s.contentPath(sum)); err == nil {
		_ = os.Remove(temp.Name())
		return sum, size, nil
	}
	if err := os.Rename(temp.Name(), s.contentPath(sum)); err != nil {
		return "", 0, err
	}
	return sum, size, nil
}

func (s *resourceStore) Open(_ context.Context, hash string) (io.ReadCloser, error) {
	if _, err := schema.ParseHash(hash); err != nil {
		return nil, err
	}
	f, err := os.Open(s.contentPath(hash))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, llm.ErrNotFound.Withf("resource content %q", hash)
	}
	return f, err
}

func (s *resourceStore) Remove(_ context.Context, hash string) error {
	if _, err := schema.ParseHash(hash); err != nil {
		return err
	}
	if err := os.Remove(s.contentPath(hash)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (s *resourceStore) Contents(context.Context) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.dir, contentDir))
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if hash, err := schema.ParseHash(entry.Name()); err == nil && entry.Type().IsRegular() {
			result = append(result, hash)
		}
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - RESOURCES

func (s *resourceStore) Create(_ context.Context, r schema.Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.path(r.Key())); err == nil {
		return llm.ErrConflict.Withf("resource %s exists", r.Key())
	}
	return s.write(r)
}

func (s *resourceStore) Get(_ context.Context, key schema.ResourceKey) (schema.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(key)
}

func (s *resourceStore) List(context.Context) ([]schema.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	paths, err := filepath.Glob(filepath.Join(s.dir, resourcePrefix+"*"+resourceSuffix))
	if err != nil {
		return nil, err
	}
	result := make([]schema.Resource, 0, len(paths))
	for _, path := range paths {
		r, err := readResource(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

func (s *resourceStore) Update(_ context.Context, key schema.ResourceKey, fn func(*schema.Resource) error) (schema.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(key)
	if err != nil {
		return schema.Resource{}, err
	} else if err := fn(&r); err != nil {
		return schema.Resource{}, err
	}
	r.Hash, r.Session = key.Hash, key.Session
	if err := s.write(r); err != nil {
		return schema.Resource{}, err
	}
	return r, nil
}

func (s *resourceStore) Delete(_ context.Context, key schema.ResourceKey, check func(schema.Resource) error) (schema.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(key)
	if err != nil {
		return schema.Resource{}, err
	} else if check != nil {
		if err := check(r); err != nil {
			return schema.Resource{}, err
		}
	}
	if err := os.Remove(s.path(key)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return schema.Resource{}, err
	}
	return r, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// path returns the path of the file of the resource with the key
func (s *resourceStore) path(key schema.ResourceKey) string {
	name := resourcePrefix + key.Hash
	if key.Session != nil {
		name += "-" + key.Session.String()
	}
	return filepath.Join(s.dir, name+resourceSuffix)
}

// contentPath returns the path of the file of the content with the hash
func (s *resourceStore) contentPath(hash string) string {
	return filepath.Join(s.dir, contentDir, hash)
}

// read returns the resource with the key, or ErrNotFound when there's none
func (s *resourceStore) read(key schema.ResourceKey) (schema.Resource, error) {
	if _, err := schema.ParseHash(key.Hash); err != nil {
		return schema.Resource{}, err
	}
	r, err := readResource(s.path(key))
	if errors.Is(err, fs.ErrNotExist) {
		return schema.Resource{}, llm.ErrNotFound.Withf("resource %s", key)
	}
	return r, err
}

// write writes the resource to its file, through a temporary file, which is
// renamed to it, so it's never written partly
func (s *resourceStore) write(r schema.Resource) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	path := s.path(r.Key())
	temp, err := os.CreateTemp(s.dir, resourceTemp+"*")
	if err != nil {
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
		return err
	} else if err := temp.Close(); err != nil {
		_ = os.Remove(temp.Name())
		return err
	}
	return os.Rename(temp.Name(), path)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// readResource returns the resource in the file
func readResource(path string) (schema.Resource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return schema.Resource{}, err
	}
	var r schema.Resource
	if err := json.Unmarshal(data, &r); err != nil {
		return schema.Resource{}, err
	}
	return r, nil
}

// contextReader is a reader which fails once its context is done
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
