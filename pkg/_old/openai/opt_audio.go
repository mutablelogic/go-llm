package openai

import (
	"strings"

	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Audio struct {
	// Supported voices include ash, ballad, coral, sage, and verse
	Voice string `json:"voice"`

	// Supported formats: wav, mp3, flac, opus, or pcm16
	Format string `json:"format"`

	// Return the speed
	Speed float64 `json:"speed,omitempty"`
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

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func optVoice(opts *llm.Opts) string {
	if audio := optAudio(opts); audio != nil {
		return audio.Voice
	} else {
		return ""
	}
}

func optSpeed(opts *llm.Opts) float64 {
	if audio := optAudio(opts); audio != nil {
		return audio.Speed
	} else {
		return 1.0
	}
}

func optAudioFormat(opts *llm.Opts) string {
	if audio := optAudio(opts); audio != nil {
		return audio.Format
	} else {
		return "mp3"
	}
}
