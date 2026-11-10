package openai

import (
	"context"
	"regexp"
	"slices"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// voices are the voices of OpenAI's speech models, which are fixed, in the
// order OpenAI lists them, of which marin and cedar are its best
// Reference: https://developers.openai.com/api/docs/guides/text-to-speech
var voices = []string{"alloy", "ash", "ballad", "coral", "echo", "fable", "nova", "onyx", "sage", "shimmer", "verse", "marin", "cedar"}

// bestVoices are the voices OpenAI recommends, for the best quality
var bestVoices = []string{"marin", "cedar"}

// reOlderSpeech matches OpenAI's older speech models, tts-1 and tts-1-hd,
// which speak in fewer voices, olderVoices
var (
	reOlderSpeech = regexp.MustCompile(`^tts-1(-hd)?\b`)
	olderVoices   = []string{"alloy", "ash", "coral", "echo", "fable", "onyx", "nova", "sage", "shimmer"}
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListVoices returns the voices the model speaks in, which are OpenAI's
// fixed voices, which it documents, or fewer, for tts-1 and tts-1-hd, or
// none, for a model which isn't known to speak, and for other
// OpenAI-compatible servers, which don't say. They're
// optimized for English. Returns ErrNotImplemented for a model which
// doesn't speak.
func (c *Client) ListVoices(_ context.Context, model llm.Model, opts ...llm.Opt) ([]llm.Voice, string, error) {
	m, ok := model.(*Model)
	if !ok || m.provider != c {
		return nil, "", llm.ErrBadParameter.With("not a model of this provider")
	}
	switch speaks := m.Capabilities().SpeechOutput; {
	case speaks != nil && !*speaks:
		return nil, "", llm.ErrNotImplemented.Withf("%s doesn't speak", m.Name())
	case speaks == nil || c.server() != serverOpenAI:
		return llm.Paged([]llm.Voice(nil), opts...)
	}
	names := voices
	if reOlderSpeech.MatchString(m.Name()) {
		names = olderVoices
	}
	result := make([]llm.Voice, 0, len(names))
	for _, name := range names {
		voice := llm.Voice{ID: name, Title: strings.ToUpper(name[:1]) + name[1:]}
		if slices.Contains(bestVoices, name) {
			voice.Description = "Recommended, for the best quality"
		}
		result = append(result, voice)
	}
	return llm.Paged(result, opts...)
}
