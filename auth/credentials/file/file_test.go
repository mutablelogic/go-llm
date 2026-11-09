package file_test

import (
	"os"
	"path/filepath"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	file "github.com/mutablelogic/go-llm/auth/credentials/file"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_File(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "dir", "credentials.json")
	provider, err := file.New(path)
	require.NoError(t, err)

	// There's no credential, until it's set
	_, err = provider.Get(ctx, "token")
	assert.ErrorIs(t, err, llm.ErrNotFound)
	require.NoError(t, provider.Set(ctx, "token", "secret"))
	value, err := provider.Get(ctx, "token")
	require.NoError(t, err)
	assert.Equal(t, "secret", value)

	// The file is only the user's
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Another provider, of the same file, has it, as it's kept
	other, err := file.New(path)
	require.NoError(t, err)
	value, err = other.Get(ctx, "token")
	require.NoError(t, err)
	assert.Equal(t, "secret", value)
	expanded, err := other.Expand(ctx, "Bearer ${token}")
	require.NoError(t, err)
	assert.Equal(t, "Bearer secret", expanded)

	// It's deleted, once or twice
	require.NoError(t, other.Delete(ctx, "token"))
	require.NoError(t, other.Delete(ctx, "token"))
	_, err = provider.Get(ctx, "token")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

func Test_FileEnv(t *testing.T) {
	ctx := t.Context()
	provider, err := file.New(filepath.Join(t.TempDir(), "credentials.json"))
	require.NoError(t, err)

	// An environment variable comes before the file
	t.Setenv("GO_LLM_TEST_KEY", "from-env")
	require.NoError(t, provider.Set(ctx, "GO_LLM_TEST_KEY", "from-file"))
	value, err := provider.Get(ctx, "GO_LLM_TEST_KEY")
	require.NoError(t, err)
	assert.Equal(t, "from-env", value)

	// And one which isn't set is ErrNotFound, when expanded
	_, err = provider.Expand(ctx, "${GO_LLM_TEST_MISSING}")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}
