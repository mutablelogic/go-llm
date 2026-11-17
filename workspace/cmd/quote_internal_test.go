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
