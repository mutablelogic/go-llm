package cmd

import (
	"os/exec"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ShellQuote(t *testing.T) {
	// A cursor is pasted into a shell as it is, whatever it has
	for _, cursor := range []string{"page2", "page$HOME", "it's", "`date`", `a\"b`, "a b", ""} {
		quoted := shellQuote(cursor)
		out, err := exec.Command("sh", "-c", "printf %s "+quoted).Output()
		require.NoError(t, err, quoted)
		assert.Equal(t, cursor, string(out), quoted)
	}
}

func Test_SplitName(t *testing.T) {
	// A workspace's name, and, after a slash, its folder, under which is
	// the request's own
	for arg, want := range map[string][2]string{
		"files":          {"files", ""},
		"files/":         {"files", ""},
		"files/TV":       {"files", "TV"},
		"files/TV/2026/": {"files", "TV/2026"},
	} {
		name, req := splitName(arg, schema.ListRequest{})
		assert.Equal(t, want[0], name, arg)
		assert.Equal(t, want[1], req.Path, arg)
	}
	name, req := splitName("files/TV", schema.ListRequest{Path: "2026", Recursive: true})
	assert.Equal(t, "files", name)
	assert.Equal(t, "TV/2026", req.Path, "with the request's folder, under it")
	assert.True(t, req.Recursive)
	_, req = splitName("files", schema.ListRequest{Path: "TV"})
	assert.Equal(t, "TV", req.Path, "the request's, as it is")
}

func Test_Indent(t *testing.T) {
	// A recursive listing is a tree, indented under each folder
	folder := func(uri string) schema.ResourceMeta { return schema.ResourceMeta{URI: uri, Type: schema.FolderType} }
	file := func(uri string) schema.ResourceMeta { return schema.ResourceMeta{URI: uri, Type: "video/x-matroska"} }
	rows := indent([]schema.ResourceMeta{
		folder("resource://files/file:///TV/MobLand/Season 1/"),
		file("resource://files/file:///TV/MobLand/Season 1/E01.mkv"),
		folder("resource://files/file:///TV/MobLand/Season 1/Extras/"),
		file("resource://files/file:///TV/MobLand/Season 1/Extras/a.mkv"),
		file("resource://files/file:///TV/MobLand/Season 1/E02.mkv"),
		folder("resource://files/file:///TV/MobLand/Season 2/"),
		file("resource://files/file:///TV/MobLand/Season 2/E01.mkv"),
		file("resource://files/file:///TV/MobLand/notes.md"),
	})
	var depths []int
	for _, r := range rows {
		depths = append(depths, r.depth)
	}
	assert.Equal(t, []int{0, 1, 1, 2, 1, 0, 1, 0}, depths)
}
