package openai

import "strings"

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Audio struct {
	// Supported voices include ash, ballad, coral, sage, and verse
	Voice string `json:"voice"`

	// Supported formats: wav, mp3, flac, opus, or pcm16
	Format string `json:"format"`
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewAudio(voice, format string) *Audio {
	voice = strings.TrimSpace(strings.ToLower(voice))
	format = strings.TrimSpace(strings.ToLower(format))
	if voice == "" || format == "" {
		return nil
	}
	return &Audio{Voice: voice, Format: format}
}
