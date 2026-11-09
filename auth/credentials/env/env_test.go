package env_test

import (
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	env "github.com/mutablelogic/go-llm/auth/credentials/env"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Env(t *testing.T) {
	ctx := t.Context()
	t.Setenv("GO_LLM_TEST_KEY", "secret")
	t.Setenv("GO_LLM_TEST_EMPTY", "")
	provider := env.New()

	// Placeholders are replaced with the variables they name, which can be
	// set to nothing, and text without one is as it is
	for value, want := range map[string]string{
		"${GO_LLM_TEST_KEY}":        "secret",
		"$GO_LLM_TEST_KEY":          "secret",
		"Bearer ${GO_LLM_TEST_KEY}": "Bearer secret",
		"${GO_LLM_TEST_EMPTY}":      "",
		"no placeholder":            "no placeholder",
	} {
		got, err := provider.Expand(ctx, value)
		require.NoError(t, err, value)
		assert.Equal(t, want, got, value)
	}

	// But one which isn't set is ErrNotFound, which names it, and each
	// which isn't
	_, err := provider.Expand(ctx, "${GO_LLM_TEST_MISSING} ${GO_LLM_TEST_GONE}")
	assert.ErrorIs(t, err, llm.ErrNotFound)
	assert.Contains(t, err.Error(), "GO_LLM_TEST_MISSING")
	assert.Contains(t, err.Error(), "GO_LLM_TEST_GONE")
}
