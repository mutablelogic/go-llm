package openai

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestTranscribeOther(t *testing.T) {
	// DeepSeek doesn't transcribe, and Mistral doesn't translate, so
	// there's no request
	audio := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "audio/mpeg", Data: []byte("mp3")}}
	deepseek, err := New(DeepSeekEndpoint, "sk-test")
	require.NoError(t, err)
	_, _, err = deepseek.Transcribe(context.Background(), &Model{provider: deepseek, data: apiModel{ID: "deepseek-chat"}}, audio, nil)
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
	mistral, err := New(MistralEndpoint, "sk-test")
	require.NoError(t, err)
	_, _, err = mistral.Translate(context.Background(), &Model{provider: mistral, data: apiModel{ID: "voxtral-mini-latest"}}, audio, nil)
	assert.ErrorIs(t, err, llm.ErrNotImplemented)

	// Nor does a model of OpenAI's which is known not to translate
	openai, err := New("", "sk-test")
	require.NoError(t, err)
	_, _, err = openai.Translate(context.Background(), &Model{provider: openai, data: apiModel{ID: "gpt-4o-mini-transcribe"}}, audio, nil)
	assert.ErrorIs(t, err, llm.ErrNotImplemented)

	// And the audio must be audio
	image := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/png", Data: []byte("png")}}
	_, _, err = openai.Transcribe(context.Background(), &Model{provider: openai, data: apiModel{ID: "whisper-1"}}, image, nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func TestTranscribeRequestApply(t *testing.T) {
	apply := func(server server, model string, translate bool, opts ...llm.Opt) (transcribeRequest, transcribeOptions, error) {
		var r transcribeRequest
		options, err := r.apply(transcribeFamilyOf(server, model), model, translate, opts...)
		return r, options, err
	}

	t.Run("Whisper", func(t *testing.T) {
		// Whisper returns its language, and segments, which it's asked for
		r, options, err := apply(serverOpenAI, "whisper-1", false, llm.WithLanguage("fr"), llm.WithTemperature(0), llm.WithSegments(true))
		require.NoError(t, err)
		assert.Equal(t, "verbose_json", r.ResponseFormat)
		assert.Equal(t, "segment", r.TimestampGranularities)
		assert.Equal(t, "fr", r.Language)
		assert.Equal(t, types.Ptr(0.0), r.Temperature)
		assert.Equal(t, transcribeOptions{language: "fr", segments: true}, options)

		// But not speakers
		_, _, err = apply(serverOpenAI, "whisper-1", false, llm.WithSpeakers(true))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Diarize", func(t *testing.T) {
		// A diarizing model returns segments, with their speakers
		r, _, err := apply(serverOpenAI, "gpt-4o-transcribe-diarize", false, llm.WithSpeakers(true))
		require.NoError(t, err)
		assert.Equal(t, "diarized_json", r.ResponseFormat)
		assert.Equal(t, "auto", r.ChunkingStrategy)
	})

	t.Run("Other", func(t *testing.T) {
		// OpenAI's other models return text, without segments
		r, _, err := apply(serverOpenAI, "gpt-4o-transcribe", false)
		require.NoError(t, err)
		assert.Equal(t, "json", r.ResponseFormat)
		_, _, err = apply(serverOpenAI, "gpt-4o-transcribe", false, llm.WithSegments(true))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Mistral", func(t *testing.T) {
		// Mistral returns segments, and their speakers, which it's asked for
		r, _, err := apply(serverMistral, "voxtral-mini-latest", false)
		require.NoError(t, err)
		assert.Empty(t, r.ResponseFormat)
		assert.Empty(t, r.TimestampGranularity)
		r, _, err = apply(serverMistral, "voxtral-mini-latest", false, llm.WithSpeakers(true))
		require.NoError(t, err)
		assert.True(t, r.Diarize)
		assert.Equal(t, "segment", r.TimestampGranularity)
		assert.Empty(t, r.TimestampGranularities)
	})

	t.Run("Server", func(t *testing.T) {
		// OpenAI's models are the same on another server, and other models
		// are like whisper
		r, _, err := apply(serverOther, "gpt-4o-mini-transcribe", false)
		require.NoError(t, err)
		assert.Equal(t, "json", r.ResponseFormat)
		r, _, err = apply(serverOther, "gpt-4o-transcribe-diarize", false)
		require.NoError(t, err)
		assert.Equal(t, "diarized_json", r.ResponseFormat)
		r, _, err = apply(serverOther, "large-v3", false)
		require.NoError(t, err)
		assert.Equal(t, "verbose_json", r.ResponseFormat)
		assert.Equal(t, transcribeWhisper, transcribeFamilyOf(serverOpenAI, "whisper-1"))
		assert.Equal(t, transcribeText, transcribeFamilyOf(serverOpenAI, "gpt-transcribe"))
		assert.Equal(t, transcribeMistral, transcribeFamilyOf(serverMistral, "voxtral-mini-latest"))
	})

	t.Run("Errors", func(t *testing.T) {
		// A translation has no language, and options which aren't for
		// transcription aren't valid
		_, _, err := apply(serverOpenAI, "whisper-1", true, llm.WithLanguage("fr"))
		assert.ErrorIs(t, err, llm.ErrBadParameter)

		// And a language is an ISO 639-1 code, even when it's set with
		// WithOpt, such as from a request
		for _, language := range []string{"FRENCH", "FR", "french", ""} {
			_, _, err = apply(serverOpenAI, "whisper-1", false, llm.WithOpt(llm.LanguageKey, json.RawMessage(`"`+language+`"`)))
			assert.ErrorIs(t, err, llm.ErrBadParameter, language)
		}
		_, _, err = apply(serverOpenAI, "whisper-1", false, llm.WithDimensions(3))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, _, err = apply(serverOpenAI, "whisper-1", false, llm.WithOpt(llm.SegmentsKey, "yes"))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

func TestTranscribeResponse(t *testing.T) {
	decode := func(data string) transcribeResponse {
		var r transcribeResponse
		require.NoError(t, json.Unmarshal([]byte(data), &r))
		return r
	}

	t.Run("Language", func(t *testing.T) {
		// Whisper's names are codes, as are gpt-transcribe's codes, and
		// Mistral's null isn't known
		assert.Equal(t, "fr", decode(`{"language":"french"}`).language())
		assert.Equal(t, "haw", decode(`{"language":"HAW"}`).language())
		assert.Equal(t, "en", decode(`{"languages":[{"code":"en"}]}`).language())
		assert.Empty(t, decode(`{"language":null}`).language())
	})

	t.Run("Usage", func(t *testing.T) {
		// OpenAI's seconds, or tokens, and Mistral's seconds and tokens,
		// whose input tokens are the audio's
		assert.Equal(t, llm.Usage{Seconds: 7}, decode(`{"usage":{"type":"duration","seconds":6.6}}`).usage())
		assert.Equal(t, llm.Usage{InputTokens: 64, OutputTokens: 23}, decode(`{"usage":{"type":"tokens","input_tokens":64,"output_tokens":23,"total_tokens":87}}`).usage())
		assert.Equal(t, llm.Usage{InputTokens: 378, OutputTokens: 22, Seconds: 6}, decode(`{"usage":{"prompt_audio_seconds":6,"prompt_tokens":3,"total_tokens":400,"completion_tokens":22}}`).usage())

		// And whisper's duration, rounded up, without usage
		assert.Equal(t, llm.Usage{Seconds: 4}, decode(`{"task":"translate","duration":3.42}`).usage())
		assert.Equal(t, llm.Usage{Seconds: 7}, decode(`{"duration":6.42,"usage":{"type":"duration","seconds":7}}`).usage())
	})

	t.Run("Speaker", func(t *testing.T) {
		// OpenAI's speaker, or Mistral's speaker id
		r := decode(`{"segments":[{"speaker":"A","start":0.5,"end":1.25},{"speaker_id":"speaker_1"},{"speaker_id":null}]}`)
		require.Len(t, r.Segments, 3)
		assert.Equal(t, "A", r.Segments[0].speaker())
		assert.Equal(t, "speaker_1", r.Segments[1].speaker())
		assert.Empty(t, r.Segments[2].speaker())
		assert.Equal(t, 1250*time.Millisecond, seconds(r.Segments[0].End))
	})
}

func TestTranscribeStream(t *testing.T) {
	// streamed returns the pieces of text, and segments, streamed, and the
	// response, of the events, which are their data
	streamed := func(segments bool, events ...string) ([]string, []llm.TranscriptSegment, transcribeStream) {
		var texts []string
		var streamed []llm.TranscriptSegment
		s := transcribeStream{segments: segments, stream: func(text string, segment *llm.TranscriptSegment) {
			if segment != nil {
				assert.Empty(t, text)
				streamed = append(streamed, *segment)
			} else {
				texts = append(texts, text)
			}
		}}
		for _, data := range events {
			require.NoError(t, s.add(client.TextStreamEvent{Data: data}))
		}
		return texts, streamed, s
	}

	t.Run("OpenAI", func(t *testing.T) {
		// Pieces of text, and then the transcript, with its usage
		texts, segments, s := streamed(false,
			`{"type":"transcript.text.delta","delta":"Hello"}`,
			`{"type":"transcript.text.delta","delta":", world."}`,
			`{"type":"transcript.text.done","text":"Hello, world.","usage":{"type":"tokens","input_tokens":64,"output_tokens":5}}`,
		)
		assert.Equal(t, []string{"Hello", ", world."}, texts)
		assert.Empty(t, segments)
		assert.True(t, s.done)
		assert.Equal(t, "Hello, world.", s.response.Text)
		assert.Equal(t, llm.Usage{InputTokens: 64, OutputTokens: 5}, s.response.usage())
	})

	t.Run("Separators", func(t *testing.T) {
		// Segments whose text doesn't start with a space are separated by
		// one
		texts, _, _ := streamed(false,
			`{"type":"transcript.text.segment","text":"Hello.","start":0,"end":0.5}`,
			`{"type":"transcript.text.segment","text":"How are you?","start":0.5,"end":1}`,
			`{"type":"transcript.text.segment","text":" Fine.","start":1,"end":1.5}`,
		)
		assert.Equal(t, "Hello. How are you? Fine.", strings.Join(texts, ""))
	})

	t.Run("Language", func(t *testing.T) {
		// Mistral's language, when it's streamed, and the transcript has
		// none
		_, _, s := streamed(false,
			`{"type":"transcription.language","audio_language":"fr"}`,
			`{"type":"transcription.done","text":"Bonjour.","language":null}`,
		)
		assert.Equal(t, "fr", s.response.language())
	})

	t.Run("Diarize", func(t *testing.T) {
		// Segments, without text, whose text is streamed as text, without
		// the space at its start
		texts, segments, s := streamed(true,
			`{"type":"transcript.text.segment","text":" Hello,","speaker":"A","start":0,"end":0.5}`,
			`{"type":"transcript.text.segment","text":" world.","speaker":"B","start":0.5,"end":1}`,
			`{"type":"transcript.text.done","text":"Hello, world."}`,
		)
		assert.Equal(t, []string{"Hello,", " world."}, texts)
		assert.Equal(t, []llm.TranscriptSegment{
			{Start: 0, End: 500 * time.Millisecond, Text: "Hello,", Speaker: "A"},
			{Start: 500 * time.Millisecond, End: time.Second, Text: "world.", Speaker: "B"},
		}, segments)
		assert.Len(t, s.response.Segments, 2)
	})

	t.Run("Mistral", func(t *testing.T) {
		// Pieces of text, and segments, which aren't streamed unless
		// they're asked for, and then the transcript, with its segments
		texts, segments, s := streamed(false,
			`{"type":"transcription.text.delta","text":"Hello,"}`,
			`{"type":"transcription.segment","text":"Hello,","start":0,"end":0.5,"speaker_id":"speaker_1"}`,
			`{"type":"transcription.text.delta","text":" world."}`,
			`{"type":"transcription.done","text":"Hello, world.","segments":[{"text":"Hello, world.","start":0,"end":1}],"usage":{"prompt_audio_seconds":1,"total_tokens":10,"completion_tokens":4}}`,
		)
		assert.Equal(t, []string{"Hello,", " world."}, texts)
		assert.Empty(t, segments)
		assert.Len(t, s.response.Segments, 1)
		assert.Equal(t, llm.Usage{InputTokens: 6, OutputTokens: 4, Seconds: 1}, s.response.usage())
	})

	t.Run("Errors", func(t *testing.T) {
		// An event with an error fails the transcript, and the stream ends
		s := transcribeStream{stream: func(string, *llm.TranscriptSegment) {}}
		assert.ErrorContains(t, s.add(client.TextStreamEvent{Data: `{"type":"error","error":{"message":"bad audio"}}`}), "bad audio")
		assert.ErrorIs(t, s.add(client.TextStreamEvent{Data: "[DONE]"}), io.EOF)
	})
}

func TestAudioName(t *testing.T) {
	// The name, with an audio extension, or "audio" with the type's
	assert.Equal(t, "talk.mp3", audioName("talk.mp3", "audio/mpeg"))
	assert.Equal(t, "audio.m4a", audioName("", "audio/mp4"))
	assert.Equal(t, "audio.wav", audioName("talk", "audio/x-wav"))
	assert.Equal(t, "audio.mp3", audioName("talk.txt", "audio/mpeg"))
}
