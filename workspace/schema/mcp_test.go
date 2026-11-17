package schema_test

import (
	"encoding/json"
	"testing"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// roundTrip encodes the value as JSON, and decodes it, as it's sent
func roundTrip[T any](t *testing.T, v *T) *T {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	var result T
	require.NoError(t, json.Unmarshal(data, &result))
	return &result
}

func Test_ListRequestMCP(t *testing.T) {
	// A request is sent with its folder, recursion, and limit, in _meta
	req := schema.ListRequest{Path: "notes/2026", Recursive: true, Cursor: "c1", Limit: 50}
	params := req.MCP()
	assert.Equal(t, "c1", params.Cursor)
	assert.Equal(t, req, schema.ListRequestOf(roundTrip(t, params)))

	// Without them, there's no _meta, as a server which isn't a workspace
	// expects
	assert.Nil(t, schema.ListRequest{}.MCP().Meta)
	assert.Equal(t, schema.ListRequest{}, schema.ListRequestOf(nil))
}

func Test_ResourceMetaMCP(t *testing.T) {
	// A resource is sent with its modified time, metadata, and artwork
	r := schema.ResourceMeta{
		URI:      "file:///notes/meeting.md",
		Name:     "meeting.md",
		Title:    "Meeting",
		Type:     "text/markdown",
		Size:     42,
		Modified: time.Date(2026, 11, 17, 10, 0, 0, 123456789, time.UTC),
		Meta:     []schema.Meta{{NS: "markdown", Key: "tags", Text: "work", Value: json.RawMessage(`["work"]`)}},
		Artwork:  []schema.Artwork{{Key: "thumbnail", Type: "image/jpeg", URI: "artwork:///1"}},
	}
	resource := r.MCP()
	assert.Equal(t, "2026-11-17T10:00:00.123456789Z", resource.Annotations.LastModified, "with its fraction of a second")
	assert.Equal(t, r, schema.ResourceMetaOf(roundTrip(t, resource)))

	// A resource of another server is as it is
	plain := &sdkmcp.Resource{URI: "file:///a.txt", Name: "a.txt", MIMEType: "text/plain"}
	assert.Equal(t, schema.ResourceMeta{URI: "file:///a.txt", Name: "a.txt", Type: "text/plain"}, schema.ResourceMetaOf(plain))
}

func Test_Document(t *testing.T) {
	// A part is a Markdown document, with YAML front matter
	next := uint64(16384)
	doc := schema.Markdown{
		URI:  "resource://github/ui://github/get-me",
		Type: "text/html",
		Body: "# Hello\n",
		Next: &next,
		Meta: map[string]string{"dc:title": "Profile", "dc:language": "en"},
	}.Document()
	assert.Equal(t, `---
uri: "resource://github/ui://github/get-me"
type: "text/html"
offset: 0
next: 16384
meta:
  "dc:language": "en"
  "dc:title": "Profile"
---

# Hello
`, doc)

	// Without text, it's only its front matter, with its note
	doc = schema.Markdown{URI: "file:///a.html", Note: "No text"}.Document()
	assert.Equal(t, "---\nuri: \"file:///a.html\"\noffset: 0\nnote: \"No text\"\n---\n", doc)
}
