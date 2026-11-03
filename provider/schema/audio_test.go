package schema_test

import (
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
)

func Test_AudioName(t *testing.T) {
	// The name, with an audio extension, or "audio" with the type's
	assert.Equal(t, "talk.mp3", schema.AudioName("talk.mp3", "audio/mpeg"))
	assert.Equal(t, "audio.m4a", schema.AudioName("", "audio/mp4"))
	assert.Equal(t, "audio.wav", schema.AudioName("talk", "audio/x-wav"))
	assert.Equal(t, "audio.wav", schema.AudioName("", "audio/wave"))
	assert.Equal(t, "audio.mp3", schema.AudioName("talk.txt", "audio/mpeg"))
	assert.Equal(t, "audio", schema.AudioName("", "audio/x-unknown"))
}
