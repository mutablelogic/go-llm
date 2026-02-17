package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	jsonExt              = ".json"
	DirPerm  os.FileMode = 0o700 // Directory permission for store directories
	FilePerm os.FileMode = 0o600 // File permission for store files
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - FILE UTILITIES

// ensureDir validates that dir is non-empty and creates it if needed.
func ensureDir(dir string) error {
	if dir == "" {
		return llm.ErrBadParameter.With("directory is required")
	}
	if err := os.MkdirAll(dir, DirPerm); err != nil {
		return llm.ErrInternalServerError.Withf("mkdir: %v", err)
	}
	return nil
}

// writeJSON serialises v to a JSON file at the given path.
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return llm.ErrInternalServerError.Withf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, FilePerm); err != nil {
		return llm.ErrInternalServerError.Withf("write: %v", err)
	}
	return nil
}

// readJSON deserialises a JSON file into v. Returns ErrNotFound when the
// file does not exist, using label to identify the missing resource.
func readJSON(path string, label string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return llm.ErrNotFound.Withf("%s", label)
		}
		return llm.ErrInternalServerError.Withf("read: %v", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return llm.ErrInternalServerError.Withf("unmarshal: %v", err)
	}
	return nil
}

// readJSONDir returns the IDs (filenames without .json extension) of all
// JSON files in dir, skipping subdirectories and non-JSON files.
func readJSONDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, llm.ErrInternalServerError.Withf("readdir: %v", err)
	}
	var ids []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), jsonExt) {
			continue
		}
		ids = append(ids, strings.TrimSuffix(entry.Name(), jsonExt))
	}
	return ids, nil
}

// jsonPath returns the file path for an ID in the given directory.
func jsonPath(dir, id string) string {
	return filepath.Join(dir, id+jsonExt)
}
