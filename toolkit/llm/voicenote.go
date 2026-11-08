package llm

import (
	"context"
	"mime"
	"strings"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// VoiceNoteConverter converts audio, such as speech a model spoke, to a
// voice note, which is Ogg Opus, such as with opusenc, of opus-tools
type VoiceNoteConverter interface {
	// Convert returns the audio as a voice note, which is Ogg Opus, or
	// ErrNotImplemented when it isn't audio it converts
	Convert(ctx context.Context, audio gollm.Resource) (gollm.Resource, error)
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// VoiceNote is the format of speech which is a voice note, such as a
	// messaging app's, which is Ogg Opus
	VoiceNote = "voice-note"

	// voiceNoteType is the type of a voice note
	voiceNoteType = "audio/ogg"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// isVoiceNote returns whether the audio of the type is a voice note, which
// is Ogg, or Opus
func isVoiceNote(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	switch strings.ToLower(mediaType) {
	case "audio/ogg", "audio/opus":
		return true
	}
	return false
}
