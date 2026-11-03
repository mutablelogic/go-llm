package schema_test

import (
	"mime"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ResourceType(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	wav := []byte("RIFF\x24\x00\x00\x00WAVEfmt ")

	// The type is detected from the data
	assert.Equal(t, "image/png", schema.ResourceType(png, ""))
	assert.Equal(t, "image/png", schema.ResourceType(png, "photo.jpg"))
	assert.Equal(t, "audio/wave", schema.ResourceType(wav, "clip"))

	// Or the name's extension, when the data isn't recognised, or is only
	// text, which the extension is more specific about. The extensions are
	// those Go knows of, whatever the system's types.
	assert.Equal(t, "image/webp", schema.ResourceType([]byte{0, 1, 2, 3}, "photo.WEBP"))
	assert.Equal(t, "application/json", schema.ResourceType([]byte(`{"a":1}`), "data.json"))
	assert.Contains(t, schema.ResourceType([]byte("hello"), ""), "text/plain")

	// Or it's binary
	assert.Equal(t, "application/octet-stream", schema.ResourceType([]byte{0, 1, 2, 3}, ""))

	// And Ogg, which is detected as a container, is its extension's type,
	// when it's audio or video, or audio
	ogg := []byte("OggS\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00")
	require.NoError(t, mime.AddExtensionType(".oga", "audio/ogg"))
	require.NoError(t, mime.AddExtensionType(".ogv", "video/ogg"))
	assert.Equal(t, "audio/ogg", schema.ResourceType(ogg, "clip.oga"))
	assert.Equal(t, "video/ogg", schema.ResourceType(ogg, "clip.ogv"))
	assert.Equal(t, "audio/ogg", schema.ResourceType(ogg, ""))
	assert.Equal(t, "audio/ogg", schema.ResourceType(ogg, "clip.json"))

	// And without data, it's the type of the extension, or none
	assert.Equal(t, "image/png", schema.ResourceType(nil, "photo.png"))
	assert.Empty(t, schema.ResourceType(nil, "photo"))
}

func Test_UnknownType(t *testing.T) {
	for _, contentType := range []string{"", " ", "application/octet-stream", "Application/Octet-Stream; charset=binary"} {
		assert.True(t, schema.UnknownType(contentType), contentType)
	}
	for _, contentType := range []string{"image/png", "audio/wave", "text/plain"} {
		assert.False(t, schema.UnknownType(contentType), contentType)
	}
}
