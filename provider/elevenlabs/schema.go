package elevenlabs

import (
	"math"
	"strings"
	"time"
	"unicode"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// transcribeRequest is a request to transcribe the speech in audio, which is
// a multipart form, with the audio as a file, the language, which is
// detected when it isn't set, and whether speakers are distinguished
// Reference: https://elevenlabs.io/docs/api-reference/speech-to-text/convert
type transcribeRequest struct {
	Model       string     `json:"model_id"`
	File        types.File `json:"file"`
	Language    string     `json:"language_code,omitempty"`
	Temperature *float64   `json:"temperature,omitempty"`
	Diarize     bool       `json:"diarize,omitempty"`
	Timestamps  string     `json:"timestamps_granularity,omitempty"` // word, or character
}

// transcribeResponse is a transcript, in the language it was spoken in, as
// an ISO 639-3 code, such as "eng", its text, its words, with when each was
// spoken, and who spoke it, when speakers are distinguished, and how long
// the audio is, in seconds
type transcribeResponse struct {
	Language string           `json:"language_code"`
	Text     string           `json:"text"`
	Words    []transcribeWord `json:"words"`
	Duration float64          `json:"audio_duration_secs"`
}

// transcribeWord is a word of a transcript, the space between words, or an
// event in the audio, such as laughter, from when to when, in seconds, and
// who spoke it, when speakers are distinguished
type transcribeWord struct {
	Text    string  `json:"text"`
	Type    string  `json:"type"` // word, spacing, or audio_event
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Speaker *string `json:"speaker_id,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// segmentGap is the silence between words which starts a segment
const segmentGap = time.Second

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// language returns the language of the transcript, as an ISO 639-1 code,
// such as "en", or its ISO 639-3 code, when it hasn't one
func (r transcribeResponse) language() string {
	code := strings.ToLower(strings.TrimSpace(r.Language))
	if language, exists := languages[code]; exists {
		return language
	}
	return code
}

// usage returns the seconds of audio, rounded up
func (r transcribeResponse) usage() llm.Usage {
	return llm.Usage{Seconds: uint(math.Ceil(r.Duration))}
}

// segments returns the segments of the transcript, of its words, which
// start when the speaker changes, after the end of a sentence, or a
// silence, and an event in the audio, such as laughter, which is its own
// segment
func (r transcribeResponse) segments() []llm.TranscriptSegment {
	var result []llm.TranscriptSegment
	var segment *llm.TranscriptSegment
	var text strings.Builder
	end := func() {
		if segment != nil {
			if segment.Text = strings.TrimSpace(text.String()); segment.Text != "" {
				result = append(result, *segment)
			}
			segment = nil
			text.Reset()
		}
	}
	for _, word := range r.Words {
		start, finish := seconds(word.Start), seconds(word.End)
		if word.Type == "spacing" {
			if segment != nil {
				text.WriteString(word.Text)
			}
			continue
		}

		// A segment ends when the speaker changes, after a sentence, or a
		// silence, or an event, and an event is a segment
		speaker := types.Value(word.Speaker)
		if segment != nil && (speaker != segment.Speaker || start-segment.End >= segmentGap || word.Type == "audio_event" || endsSentence(text.String())) {
			end()
		}
		if segment == nil {
			segment = &llm.TranscriptSegment{Start: start, Speaker: speaker}
		}
		text.WriteString(word.Text)
		segment.End = finish
		if word.Type == "audio_event" {
			end()
		}
	}
	end()
	return result
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// endsSentence returns true when the text ends with the end of a sentence
func endsSentence(text string) bool {
	text = strings.TrimRightFunc(text, unicode.IsSpace)
	return strings.HasSuffix(text, ".") || strings.HasSuffix(text, "?") || strings.HasSuffix(text, "!")
}

// seconds returns the seconds as a duration
func seconds(value float64) time.Duration {
	return time.Duration(value * float64(time.Second))
}
