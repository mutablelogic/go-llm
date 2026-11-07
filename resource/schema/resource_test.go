package schema_test

import (
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/resource/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_URI(t *testing.T) {
	// A resource's URI has its hash as its path, as it has no host, and its
	// URI as it was, with its hash as its host, is the same resource
	hash := strings.Repeat("a", 64)
	assert.Equal(t, "resource:///"+hash, schema.URI(hash))
	for _, uri := range []string{
		"resource:///" + hash,
		"resource://" + hash,
		" resource:///" + strings.ToUpper(hash) + " ",
	} {
		got, err := schema.ParseURI(uri)
		require.NoError(t, err, uri)
		assert.Equal(t, hash, got, uri)
	}

	// What isn't one, isn't
	for _, uri := range []string{
		"resource:////" + hash,
		"resource:///" + hash[:63],
		"file:///" + hash,
		hash,
		"resource:///",
	} {
		_, err := schema.ParseURI(uri)
		assert.ErrorIs(t, err, llm.ErrBadParameter, uri)
	}
}
