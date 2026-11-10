package elevenlabs_test

import (
	"context"
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
}

func (s *stubServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("xi-api-key") != "secret" {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail":{"status":"invalid_api_key"}}`))
		return
	}
	switch r.URL.Path {
	case "/v1/models":
		w.Write([]byte(`[]`))
	case "/v1/speech-to-text":
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
		_, err = c.GetModel(ctx, "eleven_v3")
		assert.ErrorIs(t, err, llm.ErrNotFound)
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
	voices, err := c.ListVoices(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, voices, "ElevenLabs' own voices, at least")
	for _, voice := range voices {
		assert.NotEmpty(t, voice.Name)
		assert.NotEmpty(t, voice.ID)
		t.Logf("%s (%s): %s, %s, %s, %v", voice.Name, voice.Category, voice.Gender, voice.Age, voice.Accent, voice.Languages)
	}
}
