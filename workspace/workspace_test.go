package workspace_test

import (
	"encoding/json"
	"image"
	"testing"

	// Packages
	workspace "github.com/mutablelogic/go-llm/workspace"
	metadata "github.com/mutablelogic/go-metadata"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// meta is go-metadata's metadata, with a value
type meta struct {
	ns, key, text string
	value         any
}

func (m meta) NS() string         { return m.ns }
func (m meta) Key() string        { return m.key }
func (m meta) Value() string      { return m.text }
func (m meta) Bytes() []byte      { return []byte(m.text) }
func (m meta) Image() image.Image { return nil }
func (m meta) Any() any           { return m.value }

func Test_Entry(t *testing.T) {
	// An entry is a folder, when its type says so, and is listed as JSON,
	// with its metadata as text, and as JSON, and its artwork by URI
	folder := workspace.Entry{URI: "file:///notes/2026/", Name: "2026", Type: workspace.FolderType}
	assert.True(t, folder.IsFolder())
	note := workspace.Entry{
		URI:     "file:///notes/2026/meeting.md",
		Name:    "meeting.md",
		Type:    "text/markdown",
		Size:    42,
		Meta:    []workspace.Meta{{NS: "markdown", Key: "tags", Text: "work,planning", Value: json.RawMessage(`["work","planning"]`)}},
		Artwork: []workspace.Artwork{{Key: "thumbnail", Type: "image/jpeg", URI: "artwork:///abc"}},
	}
	assert.False(t, note.IsFolder())
	data, err := json.Marshal(note)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"uri": "file:///notes/2026/meeting.md",
		"name": "meeting.md",
		"type": "text/markdown",
		"size": 42,
		"meta": [{"ns": "markdown", "key": "tags", "text": "work,planning", "value": ["work","planning"]}],
		"artwork": [{"key": "thumbnail", "type": "image/jpeg", "uri": "artwork:///abc"}]
	}`, string(data))
}

func Test_MetaOf(t *testing.T) {
	// go-metadata's metadata is an entry's, as text, and as JSON, apart from
	// its artwork, and a value which can't be JSON is its text
	md := []metadata.Metadata{
		meta{ns: "exif", key: "ISOSpeedRatings", text: "400", value: 400},
		meta{ns: metadata.ArtworkNamespace, key: "cover", text: "image/jpeg", value: []byte{0xff}},
		meta{ns: "id3", key: "Artist", text: "Someone", value: func() {}},
		nil,
	}
	assert.Equal(t, []workspace.Meta{
		{NS: "exif", Key: "ISOSpeedRatings", Text: "400", Value: json.RawMessage(`400`)},
		{NS: "id3", Key: "Artist", Text: "Someone", Value: json.RawMessage(`"Someone"`)},
	}, workspace.MetaOf(md))
}
