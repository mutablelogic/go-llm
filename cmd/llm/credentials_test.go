//go:build !client

package main

import (
	"os"
	"path/filepath"
	"testing"

	// Packages
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_CredentialsKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "llm", credentialsKeyFile)

	// It's made, readable only by its owner, and the same once it's made
	first, err := credentialsKey(path)
	require.NoError(t, err)
	assert.Len(t, first, 43)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	second, err := credentialsKey(path)
	require.NoError(t, err)
	assert.Equal(t, first, second)

	// No temporary files are left
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	assert.Len(t, entries, 1)

	// One which others can read isn't used
	require.NoError(t, os.Chmod(path, 0o644))
	_, err = credentialsKey(path)
	assert.Error(t, err)
}
