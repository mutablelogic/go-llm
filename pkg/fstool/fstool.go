package fstool

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// listFiles is a tool that lists files and directories under a rooted filesystem.
type listFiles struct {
	root string
}

var _ tool.Tool = (*listFiles)(nil)

///////////////////////////////////////////////////////////////////////////////
// REQUEST / RESPONSE TYPES

// ListFilesRequest defines the input for the list_files tool.
type ListFilesRequest struct {
	Path      string `json:"path,omitempty" jsonschema:"Relative path under the root directory to list (default: root)"`
	Recursive *bool  `json:"recursive,omitempty" jsonschema:"List files recursively (default: true)"`
}

// FileEntry represents a single file or directory in the listing.
type FileEntry struct {
	Path     string    `json:"path"`               // Relative path from the root
	IsDir    bool      `json:"is_dir"`             // True if directory
	Size     int64     `json:"size,omitempty"`     // File size in bytes (0 for directories)
	Modified time.Time `json:"modified,omitempty"` // Last modification time
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewTools returns filesystem tools rooted at the given directory.
// The root directory must exist.
func NewTools(root string) ([]tool.Tool, error) {
	// Resolve to absolute path
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid root path: %v", err)
	}

	// Check that the root exists and is a directory
	info, err := os.Stat(abs)
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("root path: %v", err)
	}
	if !info.IsDir() {
		return nil, llm.ErrBadParameter.Withf("root path is not a directory: %q", abs)
	}

	return []tool.Tool{
		&listFiles{root: abs},
		&fileInfo{root: abs},
		&readFile{root: abs},
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// TOOL INTERFACE

func (*listFiles) Name() string {
	return "fs_list_files"
}

func (t *listFiles) Description() string {
	return fmt.Sprintf("List files and directories under the project root (%s). "+
		"Returns name, type, size, and modification time for each entry. "+
		"Set recursive to true to list all nested contents.", t.root)
}

func (*listFiles) Schema() (*jsonschema.Schema, error) {
	return jsonschema.For[ListFilesRequest](nil)
}

func (t *listFiles) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req ListFilesRequest

	// Unmarshal input
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}

	// Resolve the target directory
	target, err := t.resolve(req.Path)
	if err != nil {
		return nil, err
	}

	// Collect entries (recursive by default)
	var entries []FileEntry
	recursive := req.Recursive == nil || *req.Recursive
	if recursive {
		entries, err = t.walkRecursive(ctx, target)
	} else {
		entries, err = t.listDir(target)
	}
	if err != nil {
		return nil, err
	}

	return entries, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// resolve validates and returns the absolute path for a relative path,
// ensuring it stays within the root.
func (t *listFiles) resolve(relPath string) (string, error) {
	// Clean and join
	clean := filepath.Clean(relPath)
	abs := filepath.Join(t.root, clean)

	// Ensure the resolved path is within the root (prevent traversal)
	if !strings.HasPrefix(abs, t.root) {
		return "", llm.ErrBadParameter.Withf("path %q escapes root directory", relPath)
	}

	// Check it exists and is a directory
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", llm.ErrNotFound.Withf("path %q", relPath)
		}
		return "", llm.ErrInternalServerError.Withf("stat: %v", err)
	}
	if !info.IsDir() {
		return "", llm.ErrBadParameter.Withf("path %q is not a directory", relPath)
	}

	return abs, nil
}

// isHidden returns true if the base name starts with a dot.
func isHidden(name string) bool {
	return strings.HasPrefix(filepath.Base(name), ".")
}

// listDir returns the immediate children of a directory, skipping hidden entries.
func (t *listFiles) listDir(dir string) ([]FileEntry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, llm.ErrInternalServerError.Withf("readdir: %v", err)
	}

	entries := make([]FileEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		if isHidden(de.Name()) {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue // skip entries we can't stat
		}
		rel, _ := filepath.Rel(t.root, filepath.Join(dir, de.Name()))
		entries = append(entries, FileEntry{
			Path:     rel,
			IsDir:    de.IsDir(),
			Size:     info.Size(),
			Modified: info.ModTime(),
		})
	}

	return entries, nil
}

// walkRecursive returns all entries under a directory, recursively,
// skipping hidden files and directories.
func (t *listFiles) walkRecursive(ctx context.Context, dir string) ([]FileEntry, error) {
	var entries []FileEntry

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries we can't access
		}

		// Check for context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Skip the root itself
		if path == dir {
			return nil
		}

		// Skip hidden files and directories
		if isHidden(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil // skip
		}

		rel, _ := filepath.Rel(t.root, path)
		entries = append(entries, FileEntry{
			Path:     rel,
			IsDir:    d.IsDir(),
			Size:     info.Size(),
			Modified: info.ModTime(),
		})

		return nil
	})

	if err != nil {
		return nil, llm.ErrInternalServerError.Withf("walk: %v", err)
	}

	return entries, nil
}
