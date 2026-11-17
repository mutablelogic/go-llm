package schema_test

import (
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	assert "github.com/stretchr/testify/assert"
)

func Test_URI(t *testing.T) {
	// A workspace's resource's URI has the workspace, and its own URI, which
	// it's converted back to
	for _, original := range []string{"ui://github/get-me", "file:///notes/a.md", "repo://o/r/contents/README.md"} {
		uri := schema.URIOf("github", original)
		workspace, back, ok := schema.ParseURI(uri)
		assert.True(t, ok, uri)
		assert.Equal(t, "github", workspace)
		assert.Equal(t, original, back)
	}
	assert.Equal(t, "resource://github/ui://github/get-me", schema.URIOf("github", "ui://github/get-me"))

	// A resource on its own, or another URI, isn't a workspace's
	for _, uri := range []string{"resource:///abc123", "file:///a.md", "resource://github", "resource://github/", ""} {
		_, _, ok := schema.ParseURI(uri)
		assert.False(t, ok, uri)
	}
}
