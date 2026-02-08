package fstool

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
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

// fileInfo is a tool that returns detailed information about a single file.
type fileInfo struct {
	root string
}

var _ tool.Tool = (*fileInfo)(nil)

///////////////////////////////////////////////////////////////////////////////
// REQUEST / RESPONSE TYPES

// FileInfoRequest defines the input for the file_info tool.
type FileInfoRequest struct {
	Path string `json:"path" jsonschema:"Relative path to the file or directory"`
}

// FileInfoResponse contains detailed metadata for a single file or directory.
type FileInfoResponse struct {
	Path     string    `json:"path"`                // Relative path from the root
	IsDir    bool      `json:"is_dir"`              // True if directory
	Size     int64     `json:"size"`                // Size in bytes
	Modified time.Time `json:"modified"`            // Last modification time
	MimeType string    `json:"mime_type,omitempty"` // Detected MIME type (files only)
}

///////////////////////////////////////////////////////////////////////////////
// TOOL INTERFACE

func (*fileInfo) Name() string {
	return "fs_file_info"
}

func (t *fileInfo) Description() string {
	return fmt.Sprintf("Get detailed information about a file or directory under the project root (%s). "+
		"Returns path, type, size, modification time, and MIME type (for files).", t.root)
}

func (*fileInfo) Schema() (*jsonschema.Schema, error) {
	return jsonschema.For[FileInfoRequest](nil)
}

func (t *fileInfo) Run(_ context.Context, input json.RawMessage) (any, error) {
	var req FileInfoRequest

	// Unmarshal input
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}
	if req.Path == "" {
		return nil, llm.ErrBadParameter.With("path is required")
	}

	// Resolve path (allows files, not just directories)
	abs, err := t.resolvePath(req.Path)
	if err != nil {
		return nil, err
	}

	// Stat the file
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, llm.ErrNotFound.Withf("path %q", req.Path)
		}
		return nil, llm.ErrInternalServerError.Withf("stat: %v", err)
	}

	resp := &FileInfoResponse{
		Path:     req.Path,
		IsDir:    info.IsDir(),
		Size:     info.Size(),
		Modified: info.ModTime(),
	}

	// Detect MIME type for files
	if !info.IsDir() {
		resp.MimeType = detectMimeType(abs)
	}

	return resp, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// resolvePath validates and returns the absolute path for a relative path,
// ensuring it stays within the root. Unlike resolve(), this allows files too.
func (t *fileInfo) resolvePath(relPath string) (string, error) {
	clean := filepath.Clean(relPath)
	abs := filepath.Join(t.root, clean)

	// Prevent traversal
	if !strings.HasPrefix(abs, t.root) {
		return "", llm.ErrBadParameter.Withf("path %q escapes root directory", relPath)
	}

	return abs, nil
}

// detectMimeType returns the MIME type for a file. It first tries extension-based
// detection, then falls back to content sniffing for common cases.
func detectMimeType(path string) string {
	// Try extension first — fast and reliable for known types
	ext := filepath.Ext(path)
	if ext != "" {
		if mt := mime.TypeByExtension(ext); mt != "" {
			return mt
		}
	}

	// Fall back to content sniffing (reads first 512 bytes)
	f, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return "application/octet-stream"
	}

	return http.DetectContentType(buf[:n])
}
