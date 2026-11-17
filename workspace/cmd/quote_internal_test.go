package cmd

import (
	"os/exec"
	"testing"

	// Packages
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
