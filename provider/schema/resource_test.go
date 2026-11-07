package schema_test

import (
	"encoding/binary"
	"encoding/json"
	"mime"
	"net/url"
	"slices"
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

	// And MP4, which is video, unless it has the brand of audio, M4A or M4B,
	// as its major brand, or a compatible one, even without the brand of
	// video, or it's named as audio
	mp4 := func(brands string) []byte {
		return append([]byte{0, 0, 0, byte(8 + len(brands))}, "ftyp"+brands...)
	}
	assert.Equal(t, "video/mp4", schema.ResourceType(mp4("isom\x00\x00\x00\x00isomiso2mp41mp42"), ""))
	assert.Equal(t, "audio/mp4", schema.ResourceType(mp4("M4A \x00\x00\x00\x00isomiso2mp41mp42"), ""))
	assert.Equal(t, "audio/mp4", schema.ResourceType(mp4("M4A \x00\x00\x00\x00M4A isom"), ""))
	assert.Equal(t, "audio/mp4", schema.ResourceType(mp4("isom\x00\x00\x00\x00isomM4B "), ""))
	assert.Equal(t, "audio/mp4", schema.ResourceType(mp4("isom\x00\x00\x00\x00isomiso2mp41mp42"), "talk.M4A"))
	assert.Equal(t, "audio/mp4", schema.ResourceType(mp4("M4B \x00\x00\x00\x00isomiso2mp41mp42"), "book.m4b"))

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

func Test_ResourceJSON(t *testing.T) {
	// A resource's URL is a string, and is read as one, or as its parts, as
	// it was once written, and its data is base64
	u, err := url.Parse("resource://abc")
	require.NoError(t, err)
	r := schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/png", URL: u, Data: []byte("x")}}
	data, err := json.Marshal(r)
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"image/png","data":"eA==","url":"resource://abc"}`, string(data))
	var read schema.Resource
	require.NoError(t, json.Unmarshal(data, &read))
	assert.Equal(t, r, read)
	parts, err := json.Marshal(struct {
		Type string   `json:"type"`
		URL  *url.URL `json:"url"`
	}{"image/png", u})
	require.NoError(t, err)
	read = schema.Resource{}
	require.NoError(t, json.Unmarshal(parts, &read))
	assert.Equal(t, "resource://abc", read.URL.String())

	// Without a URL, it has none, and one which isn't is an error
	read = schema.Resource{}
	require.NoError(t, json.Unmarshal([]byte(`{"type":"image/png","data":"eA=="}`), &read))
	assert.Nil(t, read.URL)
	assert.Error(t, json.Unmarshal([]byte(`{"type":"image/png","url":"%zz"}`), &read))
}

func Test_PCM16(t *testing.T) {
	// Little-endian samples are big-endian, as audio/L16 is, with the rate
	// and channels, and a trailing odd byte is left out
	r := schema.PCM16([]byte{0x01, 0x02, 0x03, 0x04, 0x05}, 24000, 1)
	assert.Equal(t, []byte{0x02, 0x01, 0x04, 0x03}, r.Data)
	mediaType, params, err := mime.ParseMediaType(r.Type())
	require.NoError(t, err)
	assert.Equal(t, "audio/l16", mediaType, "MIME types are in lower case, which is the same type")
	assert.Equal(t, map[string]string{"rate": "24000", "channels": "1"}, params)
}

func Test_WAV16(t *testing.T) {
	// Little-endian samples are a WAV file, with the rate and channels,
	// whose samples are read back, and a trailing odd byte is left out
	r := schema.WAV16([]byte{0x01, 0x02, 0x03, 0x04, 0x05}, 24000, 2)
	assert.Equal(t, "audio/wav", r.Type())
	assert.Len(t, r.Data, 44+4)
	samples, rate, channels, ok := schema.WAVSamples(r.Data)
	require.True(t, ok)
	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, samples)
	assert.Equal(t, 24000, rate)
	assert.Equal(t, 2, channels)

	// What isn't a WAV file, or is cut short, isn't
	_, _, _, ok = schema.WAVSamples([]byte("RIFF"))
	assert.False(t, ok)
	_, _, _, ok = schema.WAVSamples(r.Data[:30])
	assert.False(t, ok)

	// A WAV file which is cut short, within its data, isn't one, as its
	// data chunk is larger than what's left
	_, _, _, ok = schema.WAVSamples(r.Data[:len(r.Data)-1])
	assert.False(t, ok)

	// Samples which aren't whole frames aren't
	odd := slices.Clone(r.Data)
	binary.LittleEndian.PutUint32(odd[40:44], 3)
	_, _, _, ok = schema.WAVSamples(odd)
	assert.False(t, ok)

	// A data chunk of a size which isn't known, as a stream's is, is the
	// rest of the file, and chunks after it, such as C2PA's, are left out
	streamed := slices.Clone(r.Data)
	binary.LittleEndian.PutUint32(streamed[40:44], 0xFFFFFFFF)
	samples, _, _, ok = schema.WAVSamples(streamed)
	require.True(t, ok)
	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, samples)
	signed := append(slices.Clone(r.Data), []byte("C2PA\x02\x00\x00\x00ab")...)
	samples, _, _, ok = schema.WAVSamples(signed)
	require.True(t, ok)
	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, samples)
}
