package fstool

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// readFile is a tool that reads the contents of a text file.
type readFile struct {
	root string
}

var _ tool.Tool = (*readFile)(nil)

///////////////////////////////////////////////////////////////////////////////
// REQUEST / RESPONSE TYPES

// ReadFileRequest defines the input for the read_file tool.
type ReadFileRequest struct {
	Path      string `json:"path" jsonschema:"Relative path to the file to read"`
	StartLine *int   `json:"start_line,omitempty" jsonschema:"First line to read (1-based, default: first line)"`
	EndLine   *int   `json:"end_line,omitempty" jsonschema:"Last line to read (1-based inclusive, default: last line)"`
}

// ReadFileResponse contains the file content and metadata.
type ReadFileResponse struct {
	Path       string `json:"path"`        // Relative path from the root
	Content    string `json:"content"`     // Text content (possibly truncated by line range)
	TotalLines int    `json:"total_lines"` // Total number of lines in the file
	StartLine  int    `json:"start_line"`  // First line returned (1-based)
	EndLine    int    `json:"end_line"`    // Last line returned (1-based)
}

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

const (
	// maxReadSize is the maximum file size we'll read (1MB).
	maxReadSize = 1 << 20
)

///////////////////////////////////////////////////////////////////////////////
// TOOL INTERFACE

func (*readFile) Name() string {
	return "fs_read_file"
}

func (t *readFile) Description() string {
	return fmt.Sprintf("Read the text contents of a file under the project root (%s). "+
		"Returns the content along with line metadata. "+
		"Use start_line and end_line (1-based, inclusive) to read a specific range of lines. "+
		"Binary files cannot be read.", t.root)
}

func (*readFile) Schema() (*jsonschema.Schema, error) {
	return jsonschema.For[ReadFileRequest](nil)
}

func (t *readFile) Run(_ context.Context, input json.RawMessage) (any, error) {
	var req ReadFileRequest

	// Unmarshal input
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}
	if req.Path == "" {
		return nil, llm.ErrBadParameter.With("path is required")
	}

	// Validate line numbers
	if req.StartLine != nil && *req.StartLine < 1 {
		return nil, llm.ErrBadParameter.With("start_line must be >= 1")
	}
	if req.EndLine != nil && *req.EndLine < 1 {
		return nil, llm.ErrBadParameter.With("end_line must be >= 1")
	}
	if req.StartLine != nil && req.EndLine != nil && *req.StartLine > *req.EndLine {
		return nil, llm.ErrBadParameter.With("start_line must be <= end_line")
	}

	// Resolve path
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
	if info.IsDir() {
		return nil, llm.ErrBadParameter.Withf("path %q is a directory, not a file", req.Path)
	}
	if info.Size() > maxReadSize {
		return nil, llm.ErrBadParameter.Withf("file is too large (%d bytes, max %d)", info.Size(), maxReadSize)
	}

	// Check for binary content
	if isBinary(abs) {
		return nil, llm.ErrBadParameter.Withf("file %q appears to be binary", req.Path)
	}

	// Read the file
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, llm.ErrInternalServerError.Withf("read: %v", err)
	}

	// Split into lines
	lines := splitLines(data)
	totalLines := len(lines)

	// Determine range
	startLine := 1
	endLine := totalLines
	if req.StartLine != nil {
		startLine = *req.StartLine
	}
	if req.EndLine != nil {
		endLine = *req.EndLine
	}

	// Clamp to valid range
	if startLine > totalLines {
		startLine = totalLines
	}
	if endLine > totalLines {
		endLine = totalLines
	}

	// Extract the requested range (convert to 0-based)
	selected := lines[startLine-1 : endLine]
	content := strings.Join(selected, "\n")

	return &ReadFileResponse{
		Path:       req.Path,
		Content:    content,
		TotalLines: totalLines,
		StartLine:  startLine,
		EndLine:    endLine,
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// resolvePath validates and returns the absolute path for a relative path,
// ensuring it stays within the root.
func (t *readFile) resolvePath(relPath string) (string, error) {
	clean := filepath.Clean(relPath)
	abs := filepath.Join(t.root, clean)

	// Prevent traversal
	if !strings.HasPrefix(abs, t.root) {
		return "", llm.ErrBadParameter.Withf("path %q escapes root directory", relPath)
	}

	return abs, nil
}

// isBinary returns true if the file appears to be binary by sniffing the first
// 512 bytes. A file is considered binary if the detected MIME type does not
// start with "text/" and is not a known text type.
func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return false
	}

	// Check for null bytes — a strong binary signal
	if bytes.ContainsRune(buf[:n], 0) {
		return true
	}

	ct := http.DetectContentType(buf[:n])
	mt := strings.SplitN(ct, ";", 2)[0]

	switch {
	case strings.HasPrefix(mt, "text/"):
		return false
	case mt == "application/json":
		return false
	case mt == "application/xml":
		return false
	default:
		return true
	}
}

// splitLines splits data into lines, handling \n and \r\n line endings.
func splitLines(data []byte) []string {
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	// Ensure at least one line for empty files
	if len(lines) == 0 {
		lines = []string{""}
	}

	// Preserve trailing newline as empty final line
	if len(data) > 0 && (data[len(data)-1] == '\n') {
		lines = append(lines, "")
	}

	return lines
}
