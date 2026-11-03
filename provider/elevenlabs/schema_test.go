package elevenlabs

import (
	"encoding/json"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Response(t *testing.T) {
	decode := func(data string) transcribeResponse {
		var r transcribeResponse
		require.NoError(t, json.Unmarshal([]byte(data), &r))
		return r
	}

	t.Run("Language", func(t *testing.T) {
		// An ISO 639-3 code is its ISO 639-1 code, when it has one
		assert.Equal(t, "en", decode(`{"language_code":"eng"}`).language())
		assert.Equal(t, "zh", decode(`{"language_code":"cmn"}`).language())
		assert.Equal(t, "yue", decode(`{"language_code":"yue"}`).language())
		assert.Equal(t, "en", decode(`{"language_code":"en"}`).language())
		assert.Empty(t, decode(`{}`).language())
	})

	t.Run("Usage", func(t *testing.T) {
		// The seconds of audio, rounded up
		assert.Equal(t, llm.Usage{Seconds: 7}, decode(`{"audio_duration_secs":6.43}`).usage())
	})

	t.Run("Segments", func(t *testing.T) {
		// Segments end after a sentence, a silence, or when the speaker
		// changes, and an event is a segment
		r := decode(`{"words":[
			{"text":"Hello,","type":"word","start":0.0,"end":0.5,"speaker_id":"speaker_0"},
			{"text":" ","type":"spacing","start":0.5,"end":0.8,"speaker_id":"speaker_0"},
			{"text":"world.","type":"word","start":0.8,"end":1.3,"speaker_id":"speaker_0"},
			{"text":" ","type":"spacing","start":1.3,"end":1.6,"speaker_id":"speaker_0"},
			{"text":"This","type":"word","start":1.6,"end":1.8,"speaker_id":"speaker_0"},
			{"text":" ","type":"spacing","start":1.8,"end":1.9,"speaker_id":"speaker_0"},
			{"text":"is","type":"word","start":1.9,"end":2.0,"speaker_id":"speaker_0"},
			{"text":" ","type":"spacing","start":2.0,"end":3.5,"speaker_id":"speaker_0"},
			{"text":"later","type":"word","start":3.5,"end":4.0,"speaker_id":"speaker_0"},
			{"text":"(laughter)","type":"audio_event","start":4.0,"end":4.5,"speaker_id":"speaker_0"},
			{"text":"Yes","type":"word","start":4.5,"end":5.0,"speaker_id":"speaker_1"}
		]}`)
		assert.Equal(t, []llm.TranscriptSegment{
			{Start: 0, End: 1300 * time.Millisecond, Text: "Hello, world.", Speaker: "speaker_0"},
			{Start: 1600 * time.Millisecond, End: 2 * time.Second, Text: "This is", Speaker: "speaker_0"},
			{Start: 3500 * time.Millisecond, End: 4 * time.Second, Text: "later", Speaker: "speaker_0"},
			{Start: 4 * time.Second, End: 4500 * time.Millisecond, Text: "(laughter)", Speaker: "speaker_0"},
			{Start: 4500 * time.Millisecond, End: 5 * time.Second, Text: "Yes", Speaker: "speaker_1"},
		}, r.segments())
		assert.Empty(t, decode(`{"words":[]}`).segments())
	})

	t.Run("Sentences", func(t *testing.T) {
		// Consecutive sentences, by a speaker, without a silence, are
		// segments, in any language, and before a closing quote
		r := decode(`{"words":[
			{"text":"你好。","type":"word","start":0.0,"end":0.5},
			{"text":"世界？","type":"word","start":0.5,"end":1.0},
			{"text":"مرحبا؟","type":"word","start":1.0,"end":1.5},
			{"text":"\"Sleep?\"","type":"word","start":1.5,"end":2.0},
			{"text":" ","type":"spacing","start":2.0,"end":2.1},
			{"text":"Yes","type":"word","start":2.1,"end":2.5}
		]}`)
		var texts []string
		for _, segment := range r.segments() {
			texts = append(texts, segment.Text)
		}
		assert.Equal(t, []string{"你好。", "世界？", "مرحبا؟", `"Sleep?"`, "Yes"}, texts)
	})
}

func Test_Apply(t *testing.T) {
	t.Run("Options", func(t *testing.T) {
		// The language, temperature, and speakers, which have word times
		var r transcribeRequest
		options, err := r.apply(llm.WithLanguage("fr"), llm.WithTemperature(0), llm.WithSpeakers(true))
		require.NoError(t, err)
		assert.Equal(t, "fr", r.Language)
		assert.Equal(t, types.Ptr(0.0), r.Temperature)
		assert.True(t, r.Diarize)
		assert.Equal(t, "word", r.Timestamps)
		assert.Equal(t, transcribeOptions{language: "fr", speakers: true}, options)
	})

	t.Run("Errors", func(t *testing.T) {
		for _, opt := range []llm.Opt{
			llm.WithOpt(llm.LanguageKey, json.RawMessage(`"English"`)),
			llm.WithDimensions(3),
		} {
			var r transcribeRequest
			_, err := r.apply(opt)
			assert.ErrorIs(t, err, llm.ErrBadParameter)
		}
	})
}
