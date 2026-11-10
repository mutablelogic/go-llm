package elevenlabs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	elevenlabs "github.com/mutablelogic/go-llm/provider/elevenlabs"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// stubServer is a stub of ElevenLabs' API, which checks the API key, and
// keeps the fields of the last transcription
type stubServer struct {
	sync.Mutex
	fields map[string]string
	speech map[string]any // The last request to speak, with its voice and format
}

func (s *stubServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("xi-api-key") != "secret" {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail":{"status":"invalid_api_key"}}`))
		return
	}
	switch {
	case r.URL.Path == "/v1/models":
		w.Write([]byte(`[{"model_id":"eleven_v3","name":"Eleven v3","description":"Our most expressive model.","can_do_text_to_speech":true,"maximum_text_length_per_request":20,"languages":[{"language_id":"en","name":"English"},{"language_id":"de","name":"German"}]},
			{"model_id":"eleven_english_sts_v2","name":"Eleven English v2","can_do_text_to_speech":false,"can_do_voice_conversion":true}]`))
	case r.URL.Path == "/v2/voices":
		w.Write([]byte(`{"voices":[{"voice_id":"JBFqnCBsd6RMkjVDRZzb","name":"George - Warm, Captivating Storyteller","category":"premade"},{"voice_id":"hpp4J3VqNfWAUOO0d1Us","name":"Bella - Professional, Bright, Warm","category":"premade"},{"voice_id":"Nar0000000000000000a","name":"ProfessionalNarrator","category":"cloned"}],"has_more":false}`))
	case strings.HasPrefix(r.URL.Path, "/v1/text-to-speech/"):
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		body["voice"] = strings.TrimPrefix(r.URL.Path, "/v1/text-to-speech/")
		body["output_format"] = r.URL.Query().Get("output_format")
		s.Lock()
		s.speech = body
		s.Unlock()
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("character-cost", "3")
		w.Write([]byte("audio"))
	case r.URL.Path == "/v1/speech-to-text":
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.Lock()
		s.fields = map[string]string{}
		for key, values := range r.MultipartForm.Value {
			s.fields[key] = values[0]
		}
		if files := r.MultipartForm.File["file"]; len(files) > 0 {
			s.fields["file"] = files[0].Filename
		}
		s.Unlock()
		w.Write([]byte(`{"language_code":"fra","text":"Bonjour. Salut.","audio_duration_secs":2.2,"words":[
			{"text":"Bonjour.","type":"word","start":0,"end":1,"speaker_id":"speaker_0"},
			{"text":" ","type":"spacing","start":1,"end":1.2,"speaker_id":"speaker_0"},
			{"text":"Salut.","type":"word","start":1.2,"end":2,"speaker_id":"speaker_1"}]}`))
	default:
		http.NotFound(w, r)
	}
}

func newStub(t *testing.T) (*elevenlabs.Client, *stubServer) {
	t.Helper()
	stub := new(stubServer)
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	c, err := elevenlabs.New(server.URL+"/v1", "secret")
	require.NoError(t, err)
	return c, stub
}

func Test_Client(t *testing.T) {
	ctx := context.Background()
	c, stub := newStub(t)

	t.Run("New", func(t *testing.T) {
		// ElevenLabs' endpoint by default, with an API key, which it needs
		c, err := elevenlabs.New("", "secret")
		require.NoError(t, err)
		assert.Equal(t, elevenlabs.Endpoint, c.URL().String())
		assert.Equal(t, llm.ElevenLabs, c.Name())
		_, err = elevenlabs.New("", "")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = elevenlabs.New("localhost", "secret")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Ping", func(t *testing.T) {
		// With the API key, which is wrong for another client
		require.NoError(t, c.Ping(ctx))
		other, err := elevenlabs.New(c.URL().String(), "wrong")
		require.NoError(t, err)
		assert.Error(t, other.Ping(ctx))
	})

	t.Run("Models", func(t *testing.T) {
		// The Scribe models, which transcribe, but don't translate
		models, err := c.ListModels(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, models)
		assert.Equal(t, "scribe_v2", models[0].Name())
		assert.True(t, *models[0].Capabilities().Transcription)
		assert.False(t, *models[0].Capabilities().Translation)
		model, err := c.GetModel(ctx, "scribe_v1")
		require.NoError(t, err)
		assert.Equal(t, "scribe_v1", model.Name())

		// and the models which speak, which ElevenLabs lists, but not those
		// which convert voices
		require.Len(t, models, 5)
		model, err = c.GetModel(ctx, "eleven_v3")
		require.NoError(t, err)
		assert.Equal(t, "speech_output", model.Capabilities().String())
		assert.Equal(t, "Our most expressive model.", model.Description())
		assert.Contains(t, model.Meta(), "languages")
		_, err = c.GetModel(ctx, "eleven_english_sts_v2")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Speak", func(t *testing.T) {
		// In George's voice, unless another is given, as MP3, with the
		// characters it cost
		model, err := c.GetModel(ctx, "eleven_v3")
		require.NoError(t, err)
		audio, usage, err := c.Speak(ctx, model, "Hello")
		require.NoError(t, err)
		assert.Equal(t, "audio/mpeg", audio.Type())
		assert.Equal(t, uint(3), usage.Characters)
		stub.Lock()
		assert.Equal(t, map[string]any{"text": "Hello", "model_id": "eleven_v3", "voice": "JBFqnCBsd6RMkjVDRZzb", "output_format": "mp3_44100_128"}, stub.speech)
		stub.Unlock()

		// In a voice, by its ID, or its title, as Opus, for a voice note,
		// at a speed
		audio, _, err = c.Speak(ctx, model, "Hello", llm.WithOpt(llm.VoiceKey, "bella"), llm.WithOpt(llm.AudioFormatKey, "opus"), llm.WithOpt(llm.SpeedKey, 1.1))
		require.NoError(t, err)
		assert.Equal(t, "audio/ogg; codecs=opus", audio.Type())
		stub.Lock()
		assert.Equal(t, "hpp4J3VqNfWAUOO0d1Us", stub.speech["voice"])
		assert.Equal(t, "opus_48000_64", stub.speech["output_format"])
		assert.Equal(t, map[string]any{"speed": 1.1}, stub.speech["voice_settings"])
		stub.Unlock()
		_, _, err = c.Speak(ctx, model, "Hello", llm.WithOpt(llm.VoiceKey, "hpp4J3VqNfWAUOO0d1Us"))
		require.NoError(t, err)

		// A title which is shaped like an ID is a title, and an ID which
		// isn't the account's, such as a library voice's, is passed on
		for voice, want := range map[string]string{"ProfessionalNarrator": "Nar0000000000000000a", "LibraryVoice00000000": "LibraryVoice00000000"} {
			_, _, err = c.Speak(ctx, model, "Hello", llm.WithOpt(llm.VoiceKey, voice))
			require.NoError(t, err)
			stub.Lock()
			assert.Equal(t, want, stub.speech["voice"], voice)
			stub.Unlock()
		}

		// PCM, raw, which is audio/L16, or in WAV
		audio, _, err = c.Speak(ctx, model, "Hello", llm.WithOpt(llm.AudioFormatKey, "wav"))
		require.NoError(t, err)
		assert.Equal(t, "audio/wav", audio.Type())
		audio, _, err = c.Speak(ctx, model, "Hello", llm.WithOpt(llm.AudioFormatKey, "pcm"))
		require.NoError(t, err)
		assert.Equal(t, "audio/l16; channels=1; rate=24000", audio.Type())

		// A voice which isn't the account's, a format, or an option, which
		// isn't supported, text which is longer than the model speaks, and
		// a model which doesn't speak
		for _, opt := range []llm.Opt{llm.WithOpt(llm.VoiceKey, "Nobody"), llm.WithOpt(llm.AudioFormatKey, "flac"), llm.WithOpt(llm.InstructionsKey, "calmly")} {
			_, _, err = c.Speak(ctx, model, "Hello", opt)
			assert.ErrorIs(t, err, llm.ErrBadParameter)
		}
		_, _, err = c.Speak(ctx, model, strings.Repeat("a", 21))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		scribe, err := c.GetModel(ctx, "scribe_v2")
		require.NoError(t, err)
		_, _, err = c.Speak(ctx, scribe, "Hello")
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
		_, err = c.ListVoices(ctx, scribe)
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
	})

	t.Run("Transcribe", func(t *testing.T) {
		// The text, its language, as an ISO 639-1 code, the segments, by
		// speaker, which are streamed, once it's transcribed, and the
		// seconds, and the audio is a file named for its type
		model, err := c.GetModel(ctx, "scribe_v2")
		require.NoError(t, err)
		audio := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "audio/mpeg", Data: []byte("ID3")}}
		var texts []string
		var segments []llm.TranscriptSegment
		transcript, usage, err := c.Transcribe(ctx, model, audio, func(text string, segment *llm.TranscriptSegment) {
			if segment != nil {
				segments = append(segments, *segment)
			} else {
				texts = append(texts, text)
			}
		}, llm.WithSpeakers(true))
		require.NoError(t, err)
		assert.Equal(t, "Bonjour. Salut.", transcript.Text)
		assert.Equal(t, "fr", transcript.Language)
		if assert.Len(t, transcript.Segments, 2) {
			assert.Equal(t, "speaker_1", transcript.Segments[1].Speaker)
		}
		assert.Equal(t, []string{"Bonjour. Salut."}, texts)
		assert.Equal(t, transcript.Segments, segments)
		assert.Equal(t, llm.Usage{Seconds: 3}, usage)
		stub.Lock()
		defer stub.Unlock()
		assert.Equal(t, "scribe_v2", stub.fields["model_id"])
		assert.Equal(t, "audio.mp3", stub.fields["file"])
		assert.Equal(t, "true", stub.fields["diarize"])
		assert.Equal(t, "word", stub.fields["timestamps_granularity"])
	})

	t.Run("Errors", func(t *testing.T) {
		// ElevenLabs doesn't translate, and the audio must be audio, for a
		// model of ElevenLabs
		model, err := c.GetModel(ctx, "scribe_v2")
		require.NoError(t, err)
		audio := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "audio/mpeg", Data: []byte("ID3")}}
		_, _, err = c.Translate(ctx, model, audio, nil)
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
		image := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/png", Data: []byte("png")}}
		_, _, err = c.Transcribe(ctx, model, image, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, _, err = c.Transcribe(ctx, nil, audio, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

func Test_Live(t *testing.T) {
	// The speech, with ElevenLabs' API, and the key in ELEVENLABS_API_KEY,
	// which is skipped with -short
	key := os.Getenv("ELEVENLABS_API_KEY")
	if key == "" || testing.Short() {
		t.Skip("ELEVENLABS_API_KEY isn't set, or -short")
	}
	c, err := elevenlabs.New("", key)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, c.Ping(ctx))
	model, err := c.GetModel(ctx, "scribe_v2")
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "speech-en.m4a"))
	require.NoError(t, err)
	audio := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: schema.ResourceType(data, "speech-en.m4a"), Data: data}}
	transcript, usage, err := c.Transcribe(ctx, model, audio, nil, llm.WithSpeakers(true))
	require.NoError(t, err)
	t.Logf("%+v, %+v", transcript, usage)
	assert.Contains(t, strings.ToLower(transcript.Text), "quick brown fox")
	assert.Equal(t, "en", transcript.Language)
	if assert.NotEmpty(t, transcript.Segments) {
		assert.NotEmpty(t, transcript.Segments[0].Speaker)
	}
	assert.Positive(t, usage.Seconds)
}

func Test_LiveVoices(t *testing.T) {
	// The account's voices, with the key in ELEVENLABS_API_KEY, which isn't
	// billed, and is skipped with -short
	key := os.Getenv("ELEVENLABS_API_KEY")
	if key == "" || testing.Short() {
		t.Skip("ELEVENLABS_API_KEY isn't set, or -short")
	}
	c, err := elevenlabs.New("", key)
	require.NoError(t, err)
	ctx := context.Background()
	model, err := c.GetModel(ctx, "eleven_multilingual_v2")
	require.NoError(t, err)
	list, err := c.ListVoices(ctx, model)
	require.NoError(t, err)
	assert.Empty(t, list.Next)
	voices := list.Body
	require.NotEmpty(t, voices, "ElevenLabs' own voices, at least")
	for _, voice := range voices {
		assert.NotEmpty(t, voice.ID)
		assert.NotEmpty(t, voice.Title)
		t.Logf("%s %s (%s): %s, %s, %s, %v", voice.ID, voice.Title, voice.Category, voice.Gender, voice.Age, voice.Accent, voice.Languages)
	}

	// A page at a time
	page, err := c.ListVoices(ctx, model, llm.WithLimit(5))
	require.NoError(t, err)
	assert.Len(t, page.Body, min(5, len(voices)))
	assert.Equal(t, uint(len(voices)), page.Count, "how many there are")
	next := page.Next
	if len(voices) > 5 {
		assert.NotEmpty(t, next)
		page, err = c.ListVoices(ctx, model, llm.WithLimit(5), llm.WithPageToken(next))
		require.NoError(t, err)
		assert.Equal(t, voices[5].ID, page.Body[0].ID)
	}
}

func Test_LiveSpeak(t *testing.T) {
	// A word, spoken, as a voice note, with the key in ELEVENLABS_API_KEY,
	// which is billed, a few characters, so it runs only with
	// ELEVENLABS_SPEAK set
	key := os.Getenv("ELEVENLABS_API_KEY")
	if key == "" || os.Getenv("ELEVENLABS_SPEAK") == "" || testing.Short() {
		t.Skip("ELEVENLABS_API_KEY or ELEVENLABS_SPEAK isn't set, or -short")
	}
	c, err := elevenlabs.New("", key)
	require.NoError(t, err)
	ctx := context.Background()
	model, err := c.GetModel(ctx, "eleven_flash_v2_5")
	require.NoError(t, err)
	audio, usage, err := c.Speak(ctx, model, "Hello.", llm.WithOpt(llm.VoiceKey, "George"), llm.WithOpt(llm.AudioFormatKey, "opus"))
	require.NoError(t, err)
	data, err := llm.ReadAll(ctx, audio)
	require.NoError(t, err)
	assert.Equal(t, "audio/ogg; codecs=opus", audio.Type())
	assert.True(t, strings.HasPrefix(string(data), "OggS"), "Ogg")
	assert.NotZero(t, usage.Characters)
	t.Logf("%d bytes, %d characters", len(data), usage.Characters)
}
