package schema_test

import (
	"testing"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	assert "github.com/stretchr/testify/assert"
)

func Test_URI(t *testing.T) {
	// A workspace's resource's URI has the workspace, and its own URI, or
	// its path, under its root, which it's converted back to
	for _, root := range []string{"", "file:///"} {
		for _, original := range []string{"ui://github/get-me", "file:///notes/a.md", "repo://o/r/contents/README.md", "file:///", "artwork:///a.png"} {
			uri := schema.URIOf("github", root, original)
			workspace, rest, ok := schema.ParseURI(uri)
			assert.True(t, ok, uri)
			assert.Equal(t, "github", workspace)
			assert.Equal(t, original, schema.OriginalURI(root, rest), uri)
		}
	}
	assert.Equal(t, "resource://github/ui://github/get-me", schema.URIOf("github", "", "ui://github/get-me"))
	assert.Equal(t, "resource://files/notes/a.md", schema.URIOf("files", "file:///", "file:///notes/a.md"))
	assert.Equal(t, "resource://files/ui://x/y", schema.URIOf("files", "file:///", "ui://x/y"), "not under its root")

	// Its own URI, as it was, is still read, with a root
	_, rest, ok := schema.ParseURI("resource://files/file:///notes/a.md")
	assert.True(t, ok)
	assert.Equal(t, "file:///notes/a.md", schema.OriginalURI("file:///", rest))

	// A resource on its own, or another URI, isn't a workspace's
	for _, uri := range []string{"resource:///abc123", "file:///a.md", "resource://github", "resource://github/", ""} {
		_, _, ok := schema.ParseURI(uri)
		assert.False(t, ok, uri)
	}
}

func Test_RootOf(t *testing.T) {
	templates := func(uris ...string) []*sdkmcp.ResourceTemplate {
		var result []*sdkmcp.ResourceTemplate
		for _, uri := range uris {
			result = append(result, &sdkmcp.ResourceTemplate{URITemplate: uri})
		}
		return result
	}
	assert.Equal(t, "file:///", schema.RootOf(templates(schema.RootTemplate("file:///"))))
	assert.Equal(t, "memory:///", schema.RootOf(templates("memory:///{key}")))
	assert.Equal(t, "repo://o/", schema.RootOf(templates("repo://o/{+path}", "repo://{owner}/{repo}/issues/{n}")), "the one which is a root")
	assert.Empty(t, schema.RootOf(nil))
	assert.Empty(t, schema.RootOf(templates("a:///{+path}", "b:///{+path}")), "more than one")
	assert.Empty(t, schema.RootOf(templates("repo://{owner}/{repo}")), "more than one variable")
	assert.Empty(t, schema.RootOf(templates("file:///notes{+path}")), "a prefix which isn't a folder")
	assert.Empty(t, schema.RootOf(templates("file:///{+path}.md")), "a suffix")
}
