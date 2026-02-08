package fstool_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	// Packages
	fstool "github.com/mutablelogic/go-llm/pkg/fstool"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	assert "github.com/stretchr/testify/assert"
)

// boolPtr returns a pointer to a bool value.
func boolPtr(v bool) *bool { return &v }

// intPtr returns a pointer to an int value.
func intPtr(v int) *int { return &v }

// Helper to run a tool and decode the result
func runTool(t *testing.T, tools []tool.Tool, name string, input any) any {
	t.Helper()
	for _, tl := range tools {
		if tl.Name() == name {
			var raw json.RawMessage
			if input != nil {
				data, err := json.Marshal(input)
				assert.NoError(t, err)
				raw = data
			}
			result, err := tl.Run(context.TODO(), raw)
			assert.NoError(t, err)
			return result
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

// Helper to run a tool and expect an error
func runToolErr(t *testing.T, tools []tool.Tool, name string, input any) error {
	t.Helper()
	for _, tl := range tools {
		if tl.Name() == name {
			var raw json.RawMessage
			if input != nil {
				data, err := json.Marshal(input)
				assert.NoError(t, err)
				raw = data
			}
			_, err := tl.Run(context.TODO(), raw)
			return err
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE TESTS

// Test NewTools with a valid directory
func Test_fstool_001(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	tools, err := fstool.NewTools(dir)
	assert.NoError(err)
	assert.Len(tools, 3)
	assert.Equal("fs_list_files", tools[0].Name())
	assert.Equal("fs_file_info", tools[1].Name())
	assert.Equal("fs_read_file", tools[2].Name())
}

// Test NewTools with a non-existent directory
func Test_fstool_002(t *testing.T) {
	assert := assert.New(t)
	_, err := fstool.NewTools("/nonexistent/path/xyz")
	assert.Error(err)
}

// Test NewTools with a file (not a directory)
func Test_fstool_003(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	os.WriteFile(f, []byte("hello"), 0o600)
	_, err := fstool.NewTools(f)
	assert.Error(err)
}

// Test tool has a valid schema
func Test_fstool_004(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	tools, _ := fstool.NewTools(dir)
	s, err := tools[0].Schema()
	assert.NoError(err)
	assert.NotNil(s)
}

// Test tool description contains the root path
func Test_fstool_005(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	tools, _ := fstool.NewTools(dir)
	assert.Contains(tools[0].Description(), dir)
}

///////////////////////////////////////////////////////////////////////////////
// LIST (NON-RECURSIVE) TESTS

// Test listing an empty directory (non-recursive)
func Test_fstool_006(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_list_files", fstool.ListFilesRequest{Recursive: boolPtr(false)})
	entries := result.([]fstool.FileEntry)
	assert.Empty(entries)
}

// Test listing files and directories (non-recursive)
func Test_fstool_007(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa"), 0o600)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bb"), 0o600)
	os.Mkdir(filepath.Join(dir, "subdir"), 0o700)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_list_files", fstool.ListFilesRequest{Recursive: boolPtr(false)})
	entries := result.([]fstool.FileEntry)
	assert.Len(entries, 3)

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Path] = true
	}
	assert.True(names["a.txt"])
	assert.True(names["b.txt"])
	assert.True(names["subdir"])
}

// Test file entry fields (non-recursive)
func Test_fstool_008(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0o600)
	os.Mkdir(filepath.Join(dir, "mydir"), 0o700)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_list_files", fstool.ListFilesRequest{Recursive: boolPtr(false)})
	entries := result.([]fstool.FileEntry)

	for _, e := range entries {
		switch e.Path {
		case "hello.txt":
			assert.False(e.IsDir)
			assert.Equal(int64(5), e.Size)
			assert.False(e.Modified.IsZero())
		case "mydir":
			assert.True(e.IsDir)
		}
	}
}

// Test listing a subdirectory
func Test_fstool_009(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b"), 0o700)
	os.WriteFile(filepath.Join(dir, "a", "file.txt"), []byte("x"), 0o600)
	os.WriteFile(filepath.Join(dir, "a", "b", "deep.txt"), []byte("y"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_list_files", fstool.ListFilesRequest{Path: "a", Recursive: boolPtr(false)})
	entries := result.([]fstool.FileEntry)
	assert.Len(entries, 2) // file.txt and b/

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Path] = true
	}
	assert.True(names[filepath.Join("a", "file.txt")])
	assert.True(names[filepath.Join("a", "b")])
}

///////////////////////////////////////////////////////////////////////////////
// RECURSIVE TESTS

// Test recursive listing
func Test_fstool_010(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b"), 0o700)
	os.WriteFile(filepath.Join(dir, "root.txt"), []byte("r"), 0o600)
	os.WriteFile(filepath.Join(dir, "a", "mid.txt"), []byte("m"), 0o600)
	os.WriteFile(filepath.Join(dir, "a", "b", "deep.txt"), []byte("d"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_list_files", fstool.ListFilesRequest{Recursive: boolPtr(true)})
	entries := result.([]fstool.FileEntry)

	// Should contain: root.txt, a/, a/mid.txt, a/b/, a/b/deep.txt
	assert.Len(entries, 5)

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Path] = true
	}
	assert.True(names["root.txt"])
	assert.True(names["a"])
	assert.True(names[filepath.Join("a", "mid.txt")])
	assert.True(names[filepath.Join("a", "b")])
	assert.True(names[filepath.Join("a", "b", "deep.txt")])
}

// Test recursive listing from a subdirectory
func Test_fstool_011(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src", "pkg"), 0o700)
	os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("m"), 0o600)
	os.WriteFile(filepath.Join(dir, "src", "pkg", "lib.go"), []byte("l"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_list_files", fstool.ListFilesRequest{Path: "src", Recursive: boolPtr(true)})
	entries := result.([]fstool.FileEntry)

	// src/main.go, src/pkg/, src/pkg/lib.go
	assert.Len(entries, 3)
}

///////////////////////////////////////////////////////////////////////////////
// ERROR TESTS

// Test path traversal is rejected
func Test_fstool_012(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	tools, _ := fstool.NewTools(dir)

	data, _ := json.Marshal(fstool.ListFilesRequest{Path: "../../../etc"})
	_, err := tools[0].Run(context.TODO(), data)
	assert.Error(err)
}

// Test non-existent path
func Test_fstool_013(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	tools, _ := fstool.NewTools(dir)

	data, _ := json.Marshal(fstool.ListFilesRequest{Path: "nonexistent"})
	_, err := tools[0].Run(context.TODO(), data)
	assert.Error(err)
}

// Test path pointing to a file (not a directory)
func Test_fstool_014(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o600)
	tools, _ := fstool.NewTools(dir)

	data, _ := json.Marshal(fstool.ListFilesRequest{Path: "file.txt"})
	_, err := tools[0].Run(context.TODO(), data)
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// HIDDEN FILE TESTS

// Test hidden files are skipped in non-recursive listing
func Test_fstool_015(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("v"), 0o600)
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("h"), 0o600)
	os.Mkdir(filepath.Join(dir, ".git"), 0o700)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("c"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_list_files", fstool.ListFilesRequest{Recursive: boolPtr(false)})
	entries := result.([]fstool.FileEntry)
	assert.Len(entries, 1)
	assert.Equal("visible.txt", entries[0].Path)
}

// Test hidden files and directories are skipped in recursive listing
func Test_fstool_016(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0o700)
	os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0o700)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("r"), 0o600)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("g"), 0o600)
	os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("m"), 0o600)
	os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("h"), 0o600)
	os.WriteFile(filepath.Join(dir, ".git", "objects", "pack"), []byte("p"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_list_files", fstool.ListFilesRequest{Recursive: boolPtr(true)})
	entries := result.([]fstool.FileEntry)

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Path] = true
	}
	// Should see: readme.md, src/, src/main.go
	assert.Len(entries, 3)
	assert.True(names["readme.md"])
	assert.True(names["src"])
	assert.True(names[filepath.Join("src", "main.go")])
	// Should NOT see any hidden entries
	assert.False(names[".gitignore"])
	assert.False(names[".git"])
}

// Test default (nil input) is recursive
func Test_fstool_017(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a"), 0o700)
	os.WriteFile(filepath.Join(dir, "top.txt"), []byte("t"), 0o600)
	os.WriteFile(filepath.Join(dir, "a", "nested.txt"), []byte("n"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_list_files", nil)
	entries := result.([]fstool.FileEntry)

	// Default recursive: top.txt, a/, a/nested.txt
	assert.Len(entries, 3)
}

///////////////////////////////////////////////////////////////////////////////
// FILE INFO TESTS

// Test file info for a regular text file
func Test_fstool_018(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_file_info", fstool.FileInfoRequest{Path: "hello.txt"})
	info := result.(*fstool.FileInfoResponse)

	assert.Equal("hello.txt", info.Path)
	assert.False(info.IsDir)
	assert.Equal(int64(11), info.Size)
	assert.False(info.Modified.IsZero())
	assert.Contains(info.MimeType, "text/plain")
}

// Test file info for a directory
func Test_fstool_019(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "subdir"), 0o700)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_file_info", fstool.FileInfoRequest{Path: "subdir"})
	info := result.(*fstool.FileInfoResponse)

	assert.Equal("subdir", info.Path)
	assert.True(info.IsDir)
	assert.Empty(info.MimeType) // directories have no MIME type
}

// Test file info detects JSON MIME type by extension
func Test_fstool_020(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{"key":"value"}`), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_file_info", fstool.FileInfoRequest{Path: "data.json"})
	info := result.(*fstool.FileInfoResponse)

	assert.Contains(info.MimeType, "json")
}

// Test file info detects Go source by extension
func Test_fstool_021(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_file_info", fstool.FileInfoRequest{Path: "main.go"})
	info := result.(*fstool.FileInfoResponse)

	// Go files may not have a registered MIME type on all systems; ensure it's non-empty
	assert.NotEmpty(info.MimeType)
}

// Test file info with a binary file (sniff-based detection)
func Test_fstool_022(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	// PNG header magic bytes
	pngData := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	os.WriteFile(filepath.Join(dir, "image.png"), pngData, 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_file_info", fstool.FileInfoRequest{Path: "image.png"})
	info := result.(*fstool.FileInfoResponse)

	assert.Contains(info.MimeType, "image/png")
}

// Test file info for a file with no known extension (falls back to sniffing)
func Test_fstool_023(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "noext"), []byte("plain text content\n"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_file_info", fstool.FileInfoRequest{Path: "noext"})
	info := result.(*fstool.FileInfoResponse)

	assert.Contains(info.MimeType, "text/plain")
}

// Test file info rejects path traversal
func Test_fstool_024(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	tools, _ := fstool.NewTools(dir)

	err := runToolErr(t, tools, "fs_file_info", fstool.FileInfoRequest{Path: "../../../etc/passwd"})
	assert.Error(err)
}

// Test file info rejects non-existent path
func Test_fstool_025(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	tools, _ := fstool.NewTools(dir)

	err := runToolErr(t, tools, "fs_file_info", fstool.FileInfoRequest{Path: "nope.txt"})
	assert.Error(err)
}

// Test file info rejects empty path
func Test_fstool_026(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	tools, _ := fstool.NewTools(dir)

	err := runToolErr(t, tools, "fs_file_info", fstool.FileInfoRequest{})
	assert.Error(err)
}

// Test file info has valid schema and description
func Test_fstool_027(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	tools, _ := fstool.NewTools(dir)

	// Find file info tool
	var fi tool.Tool
	for _, tl := range tools {
		if tl.Name() == "fs_file_info" {
			fi = tl
		}
	}
	assert.NotNil(fi)
	s, err := fi.Schema()
	assert.NoError(err)
	assert.NotNil(s)
	assert.Contains(fi.Description(), dir)
}

///////////////////////////////////////////////////////////////////////////////
// READ FILE TESTS

// Test reading a whole text file
func Test_fstool_028(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("line1\nline2\nline3\n"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_read_file", fstool.ReadFileRequest{Path: "hello.txt"})
	resp := result.(*fstool.ReadFileResponse)

	assert.Equal("hello.txt", resp.Path)
	assert.Equal(4, resp.TotalLines) // 3 lines + trailing newline
	assert.Equal(1, resp.StartLine)
	assert.Equal(4, resp.EndLine)
	assert.Contains(resp.Content, "line1")
	assert.Contains(resp.Content, "line3")
}

// Test reading a line range
func Test_fstool_029(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "lines.txt"), []byte("aaa\nbbb\nccc\nddd\neee\n"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_read_file", fstool.ReadFileRequest{
		Path:      "lines.txt",
		StartLine: intPtr(2),
		EndLine:   intPtr(4),
	})
	resp := result.(*fstool.ReadFileResponse)

	assert.Equal(6, resp.TotalLines) // 5 lines + trailing newline
	assert.Equal(2, resp.StartLine)
	assert.Equal(4, resp.EndLine)
	assert.Equal("bbb\nccc\nddd", resp.Content)
}

// Test reading only start_line (to end of file)
func Test_fstool_030(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "data.txt"), []byte("one\ntwo\nthree\n"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_read_file", fstool.ReadFileRequest{
		Path:      "data.txt",
		StartLine: intPtr(2),
	})
	resp := result.(*fstool.ReadFileResponse)

	assert.Equal(2, resp.StartLine)
	assert.Equal(4, resp.EndLine) // includes trailing empty line
	assert.Contains(resp.Content, "two")
	assert.Contains(resp.Content, "three")
}

// Test reading only end_line (from start)
func Test_fstool_031(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "data.txt"), []byte("alpha\nbeta\ngamma\n"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_read_file", fstool.ReadFileRequest{
		Path:    "data.txt",
		EndLine: intPtr(2),
	})
	resp := result.(*fstool.ReadFileResponse)

	assert.Equal(1, resp.StartLine)
	assert.Equal(2, resp.EndLine)
	assert.Equal("alpha\nbeta", resp.Content)
}

// Test reading a single line
func Test_fstool_032(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "abc.txt"), []byte("aaa\nbbb\nccc\n"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_read_file", fstool.ReadFileRequest{
		Path:      "abc.txt",
		StartLine: intPtr(2),
		EndLine:   intPtr(2),
	})
	resp := result.(*fstool.ReadFileResponse)

	assert.Equal("bbb", resp.Content)
}

// Test binary file is rejected
func Test_fstool_033(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	// Write binary data with null bytes
	bin := make([]byte, 256)
	for i := range bin {
		bin[i] = byte(i)
	}
	os.WriteFile(filepath.Join(dir, "binary.dat"), bin, 0o600)

	tools, _ := fstool.NewTools(dir)
	err := runToolErr(t, tools, "fs_read_file", fstool.ReadFileRequest{Path: "binary.dat"})
	assert.Error(err)
}

// Test reading a directory is rejected
func Test_fstool_034(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "subdir"), 0o700)

	tools, _ := fstool.NewTools(dir)
	err := runToolErr(t, tools, "fs_read_file", fstool.ReadFileRequest{Path: "subdir"})
	assert.Error(err)
}

// Test read file rejects path traversal
func Test_fstool_035(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	tools, _ := fstool.NewTools(dir)

	err := runToolErr(t, tools, "fs_read_file", fstool.ReadFileRequest{Path: "../../etc/passwd"})
	assert.Error(err)
}

// Test read file rejects non-existent path
func Test_fstool_036(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	tools, _ := fstool.NewTools(dir)

	err := runToolErr(t, tools, "fs_read_file", fstool.ReadFileRequest{Path: "nope.txt"})
	assert.Error(err)
}

// Test read file rejects empty path
func Test_fstool_037(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	tools, _ := fstool.NewTools(dir)

	err := runToolErr(t, tools, "fs_read_file", fstool.ReadFileRequest{})
	assert.Error(err)
}

// Test start_line > end_line is rejected
func Test_fstool_038(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("a\nb\nc\n"), 0o600)
	tools, _ := fstool.NewTools(dir)

	err := runToolErr(t, tools, "fs_read_file", fstool.ReadFileRequest{
		Path:      "file.txt",
		StartLine: intPtr(5),
		EndLine:   intPtr(2),
	})
	assert.Error(err)
}

// Test start_line < 1 is rejected
func Test_fstool_039(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("a\n"), 0o600)
	tools, _ := fstool.NewTools(dir)

	err := runToolErr(t, tools, "fs_read_file", fstool.ReadFileRequest{
		Path:      "file.txt",
		StartLine: intPtr(0),
	})
	assert.Error(err)
}

// Test read file has valid schema and description
func Test_fstool_040(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	tools, _ := fstool.NewTools(dir)

	var rf tool.Tool
	for _, tl := range tools {
		if tl.Name() == "fs_read_file" {
			rf = tl
		}
	}
	assert.NotNil(rf)
	s, err := rf.Schema()
	assert.NoError(err)
	assert.NotNil(s)
	assert.Contains(rf.Description(), dir)
}

// Test reading an empty file
func Test_fstool_041(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "empty.txt"), []byte{}, 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_read_file", fstool.ReadFileRequest{Path: "empty.txt"})
	resp := result.(*fstool.ReadFileResponse)

	assert.Equal(1, resp.TotalLines)
	assert.Equal("", resp.Content)
}

// Test end_line beyond file length is clamped
func Test_fstool_042(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "short.txt"), []byte("only\ntwo\n"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_read_file", fstool.ReadFileRequest{
		Path:    "short.txt",
		EndLine: intPtr(100),
	})
	resp := result.(*fstool.ReadFileResponse)

	assert.Equal(3, resp.TotalLines) // "only", "two", "" (trailing newline)
	assert.Equal(3, resp.EndLine)    // clamped
	assert.Contains(resp.Content, "only")
	assert.Contains(resp.Content, "two")
}

// Test reading a file with no trailing newline
func Test_fstool_043(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "notail.txt"), []byte("aaa\nbbb"), 0o600)

	tools, _ := fstool.NewTools(dir)
	result := runTool(t, tools, "fs_read_file", fstool.ReadFileRequest{Path: "notail.txt"})
	resp := result.(*fstool.ReadFileResponse)

	assert.Equal(2, resp.TotalLines) // "aaa", "bbb" — no trailing empty line
	assert.Equal("aaa\nbbb", resp.Content)
}
