package gowhisper_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	gowhisper "github.com/mutablelogic/go-llm/provider/gowhisper"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// stubServer is a stub go-whisper server, with a model of its own, and one
// of OpenAI's, which it forwards to, which downloads models, streaming its
// progress, when it's asked to, and deletes them
type stubServer struct {
	sync.Mutex
	models   map[string]map[string]any
	deleted  []string
	download string
	fields   map[string]string // The fields of the last transcription
}

func newStub(t *testing.T) (*gowhisper.Client, *stubServer) {
	t.Helper()
	stub := &stubServer{models: map[string]map[string]any{
		"ggml-medium-q5_0": {"id": "ggml-medium-q5_0", "object": "model", "path": "ggml-medium-q5_0.bin", "created": 1793634996, "owned_by": "whisper"},
		"whisper-1":        {"id": "whisper-1", "object": "model", "owned_by": "openai"},
	}}
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	c, err := gowhisper.New(server.URL)
	require.NoError(t, err)
	return c, stub
}

func (s *stubServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Lock()
	defer s.Unlock()
	w.Header().Set("Content-Type", "application/json")
	id, _ := strings.CutPrefix(r.URL.Path, "/api/whisper/model/")
	switch {
	case r.URL.Path == "/api/whisper/model" && r.Method == http.MethodGet:
		models := make([]map[string]any, 0, len(s.models))
		for _, model := range s.models {
			models = append(models, model)
		}
		json.NewEncoder(w).Encode(models)
	case r.URL.Path == "/api/whisper/model" && r.Method == http.MethodPost:
		var req struct {
			Model string `json:"model"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		s.download = req.Model
		switch {
		case req.Model == "ggml-broken.bin":
			w.Header().Set("Content-Type", types.ContentTypeTextStream)
			fmt.Fprint(w, "event: download.error\ndata: \"disk full\"\n\n")
			return
		case req.Model == "large-v3.bin" && r.Header.Get("Accept") == types.ContentTypeTextStream:
			w.Header().Set("Content-Type", types.ContentTypeTextStream)
			fmt.Fprint(w, "event: download.error\ndata: \"Not Found: 404 Not Found\"\n\n")
			return
		case req.Model == "large-v3.bin":
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"code":404,"reason":"Not Found: 404 Not Found"}`)
			return
		}
		id := strings.TrimSuffix(req.Model, ".bin")
		model := map[string]any{"id": id, "object": "model", "path": req.Model, "created": 1793640000, "owned_by": "whisper"}
		s.models[id] = model
		if r.Header.Get("Accept") == types.ContentTypeTextStream {
			w.Header().Set("Content-Type", types.ContentTypeTextStream)
			fmt.Fprint(w, "event: download.progress\ndata: {\"current\":50,\"total\":200}\n\n")
			fmt.Fprint(w, "event: download.progress\ndata: {\"current\":200,\"total\":200}\n\n")
			data, _ := json.Marshal(model)
			fmt.Fprintf(w, "event: download.done\ndata: %s\n\n", data)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(model)
	case r.URL.Path == "/api/whisper/transcribe" || r.URL.Path == "/api/whisper/translate":
		// The fields, and the audio's name, and a transcript of two
		// segments, which is streamed, when it's asked to be, or an error,
		// for audio which is an error
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.fields = map[string]string{}
		for key, values := range r.MultipartForm.Value {
			s.fields[key] = values[0]
		}
		var audio string
		if files := r.MultipartForm.File["audio"]; len(files) > 0 {
			s.fields["audio"] = files[0].Filename
			if f, err := files[0].Open(); err == nil {
				data := make([]byte, 64)
				n, _ := f.Read(data)
				audio = string(data[:n])
				f.Close()
			}
		}
		task, language := "transcribe", "french"
		if strings.HasSuffix(r.URL.Path, "translate") {
			task = "translate"
		}
		segments := []string{
			`{"id":0,"start":0,"end":1.5,"text":" Bonjour,","tokens":["[_BEG_]"]}`,
			`{"id":1,"start":1.5,"end":3.25,"text":" tout le monde."}`,
		}
		if r.Header.Get("Accept") == types.ContentTypeTextStream {
			w.Header().Set("Content-Type", types.ContentTypeTextStream)
			if audio == "error" {
				fmt.Fprint(w, "event: transcript.text.error\ndata: \"cannot decode audio\"\n\n")
				return
			}
			for _, segment := range segments {
				fmt.Fprintf(w, "event: transcript.text.delta\ndata: %s\n\n", segment)
			}
			fmt.Fprintf(w, "event: transcript.text.done\ndata: {\"task\":%q,\"language\":%q,\"duration\":3.4}\n\n", task, language)
			return
		}
		fmt.Fprintf(w, `{"task":%q,"language":%q,"duration":3.4,"text":"Bonjour, tout le monde.","segments":[%s]}`, task, language, strings.Join(segments, ","))
	case id != "" && r.Method == http.MethodGet:
		if model, exists := s.models[id]; exists {
			json.NewEncoder(w).Encode(model)
		} else {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"code":404,"reason":"Not Found: %s"}`, id)
		}
	case id != "" && r.Method == http.MethodDelete:
		delete(s.models, id)
		s.deleted = append(s.deleted, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func Test_New(t *testing.T) {
	// The endpoint is go-whisper's by default, and a host without a scheme
	// has its port, and an endpoint without a path has its path
	for endpoint, want := range map[string]string{
		"":                       "http://localhost:8081/api/whisper",
		"nx1.lan":                "http://nx1.lan:8081/api/whisper",
		"nx1.lan:9000":           "http://nx1.lan:9000/api/whisper",
		"http://nx1.lan:8081":    "http://nx1.lan:8081/api/whisper",
		"https://whisper.lan/v1": "https://whisper.lan/v1",
	} {
		c, err := gowhisper.New(endpoint)
		require.NoError(t, err, endpoint)
		assert.Equal(t, want, c.URL().String(), endpoint)
		assert.Equal(t, llm.GoWhisper, c.Name())
		assert.Equal(t, c, c.Self())
	}
}

func Test_Models(t *testing.T) {
	c, _ := newStub(t)
	ctx := context.Background()

	t.Run("List", func(t *testing.T) {
		// Only go-whisper's own models, not those it forwards to
		require.NoError(t, c.Ping(ctx))
		models, err := c.ListModels(ctx)
		require.NoError(t, err)
		require.Len(t, models, 1)
		model := models[0]
		assert.Equal(t, "ggml-medium-q5_0", model.Name())
		assert.Equal(t, []string{"ggml-medium-q5_0.bin"}, model.Aliases())
		assert.Equal(t, time.Unix(1793634996, 0), model.Created())
		assert.Equal(t, json.RawMessage(`"whisper"`), model.Meta()["owned_by"])
		assert.Equal(t, c, model.Provider())
	})

	t.Run("Get", func(t *testing.T) {
		// By its id, or its file
		for _, name := range []string{"ggml-medium-q5_0", "ggml-medium-q5_0.bin"} {
			model, err := c.GetModel(ctx, name)
			require.NoError(t, err, name)
			assert.Equal(t, "ggml-medium-q5_0", model.Name())
		}

		// But not a model which doesn't exist, or one it forwards to
		_, err := c.GetModel(ctx, "missing")
		assert.Error(t, err)
		_, err = c.GetModel(ctx, "whisper-1")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = c.GetModel(ctx, " ")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Capabilities", func(t *testing.T) {
		// A model transcribes, and translates, unless it's English-only
		model, err := c.GetModel(ctx, "ggml-medium-q5_0")
		require.NoError(t, err)
		assert.Equal(t, types.Ptr(true), model.Capabilities().Transcription)
		assert.Equal(t, types.Ptr(true), model.Capabilities().Translation)
		assert.Equal(t, types.Ptr(false), model.Capabilities().Chat)
		english, err := c.LoadModel(ctx, "ggml-tiny.en", nil)
		require.NoError(t, err)
		assert.Equal(t, types.Ptr(false), english.Capabilities().Translation)
	})
}

func Test_Download(t *testing.T) {
	c, stub := newStub(t)
	ctx := context.Background()

	t.Run("Downloaded", func(t *testing.T) {
		// A model which is downloaded isn't downloaded again
		model, err := c.LoadModel(ctx, "ggml-medium-q5_0", nil)
		require.NoError(t, err)
		assert.Equal(t, "ggml-medium-q5_0", model.Name())
		assert.Empty(t, stub.download)
	})

	t.Run("Progress", func(t *testing.T) {
		// A model is downloaded by its file, with its progress
		var percents []float64
		model, err := c.LoadModel(ctx, "ggml-base", func(status string, percent float64) {
			assert.Equal(t, "downloading", status)
			percents = append(percents, percent)
		})
		require.NoError(t, err)
		assert.Equal(t, "ggml-base", model.Name())
		assert.Equal(t, "ggml-base.bin", stub.download)
		assert.Equal(t, []float64{25, 100}, percents)
	})

	t.Run("Wait", func(t *testing.T) {
		// Or without progress, as the model, once it's downloaded
		model, err := c.LoadModel(ctx, "ggml-small.bin", nil)
		require.NoError(t, err)
		assert.Equal(t, "ggml-small", model.Name())
		assert.Equal(t, "ggml-small.bin", stub.download)
	})

	t.Run("Errors", func(t *testing.T) {
		// A download which fails, with its message
		_, err := c.LoadModel(ctx, "ggml-broken", func(string, float64) {})
		assert.EqualError(t, err, "disk full")

		// And a model which can't be downloaded isn't found, as it's
		// streamed, or not
		_, err = c.LoadModel(ctx, "large-v3", func(string, float64) {})
		assert.ErrorIs(t, err, llm.ErrNotFound)
		assert.ErrorContains(t, err, `"large-v3.bin"`)
		_, err = c.LoadModel(ctx, "large-v3", nil)
		assert.ErrorIs(t, err, llm.ErrNotFound)

		// And a model without a name
		_, err = c.LoadModel(ctx, "", nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Delete", func(t *testing.T) {
		// A model is deleted by its id, and must be go-whisper's
		model, err := c.GetModel(ctx, "ggml-small")
		require.NoError(t, err)
		require.NoError(t, c.DeleteModel(ctx, model))
		assert.Equal(t, []string{"ggml-small"}, stub.deleted)
		_, err = c.GetModel(ctx, "ggml-small")
		assert.Error(t, err)
		assert.ErrorIs(t, c.DeleteModel(ctx, nil), llm.ErrBadParameter)
	})

	t.Run("Unload", func(t *testing.T) {
		// A model isn't kept loaded, so it can't be unloaded
		model, err := c.GetModel(ctx, "ggml-medium-q5_0")
		require.NoError(t, err)
		assert.ErrorIs(t, c.UnloadModel(ctx, model), llm.ErrNotImplemented)
	})
}

func Test_Live(t *testing.T) {
	// The models of a go-whisper server, at GOWHISPER_ADDR, such as
	// nx1.lan:8081, which isn't changed
	addr := os.Getenv("GOWHISPER_ADDR")
	if addr == "" {
		t.Skip("GOWHISPER_ADDR isn't set")
	}
	c, err := gowhisper.New(addr)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, c.Ping(ctx))
	models, err := c.ListModels(ctx)
	require.NoError(t, err)
	for _, model := range models {
		t.Logf("%s %v %s", model.Name(), model.Aliases(), model.Capabilities())
		got, err := c.GetModel(ctx, model.Name())
		require.NoError(t, err)
		assert.Equal(t, model.Name(), got.Name())
	}
	_, err = c.GetModel(ctx, "ggml-missing")
	assert.Error(t, err)
}

func Test_Transcribe(t *testing.T) {
	c, stub := newStub(t)
	ctx := context.Background()
	model, err := c.GetModel(ctx, "ggml-medium-q5_0")
	require.NoError(t, err)
	audio := &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/wave", Data: []byte("RIFF")}}
	fields := func() map[string]string {
		stub.Lock()
		defer stub.Unlock()
		return stub.fields
	}

	t.Run("Transcribe", func(t *testing.T) {
		// The text, its language, as a code, and the seconds, rounded up,
		// without segments, unless they're asked for, and the audio is a
		// file named for its type, with the model's id
		transcript, usage, err := c.Transcribe(ctx, model, audio, nil)
		require.NoError(t, err)
		assert.Equal(t, llm.Transcript{Text: "Bonjour, tout le monde.", Language: "fr"}, transcript)
		assert.Equal(t, llm.Usage{Seconds: 4}, usage)
		assert.Equal(t, "ggml-medium-q5_0", fields()["model"])
		assert.Equal(t, "audio.wav", fields()["audio"])
		assert.Equal(t, "audio.wav", fields()["filename"])
	})

	t.Run("Segments", func(t *testing.T) {
		// The segments, and the language set, which is the language the
		// server has
		transcript, _, err := c.Transcribe(ctx, model, audio, nil, llm.WithSegments(true), llm.WithLanguage("de"))
		require.NoError(t, err)
		assert.Equal(t, "fr", transcript.Language)
		assert.Equal(t, []llm.TranscriptSegment{
			{Start: 0, End: 1500 * time.Millisecond, Text: "Bonjour,"},
			{Start: 1500 * time.Millisecond, End: 3250 * time.Millisecond, Text: "tout le monde."},
		}, transcript.Segments)
		assert.Equal(t, "de", fields()["language"])
	})

	t.Run("Strict", func(t *testing.T) {
		// A client which is strict accepts a stream, of a transcript, and of
		// a download
		strict, err := gowhisper.New(c.URL().String(), client.OptStrict())
		require.NoError(t, err)
		model, err := strict.GetModel(ctx, "ggml-medium-q5_0")
		require.NoError(t, err)
		transcript, _, err := strict.Transcribe(ctx, model, audio, func(string, *llm.TranscriptSegment) {})
		require.NoError(t, err)
		assert.Equal(t, "Bonjour, tout le monde.", transcript.Text)
		_, err = strict.LoadModel(ctx, "ggml-strict", func(string, float64) {})
		require.NoError(t, err)
	})

	t.Run("Stream", func(t *testing.T) {
		// Each segment's text, separated, and the segments, as they're
		// transcribed, and the text is theirs
		var texts []string
		var segments []llm.TranscriptSegment
		transcript, usage, err := c.Transcribe(ctx, model, audio, func(text string, segment *llm.TranscriptSegment) {
			if segment != nil {
				segments = append(segments, *segment)
			} else {
				texts = append(texts, text)
			}
		}, llm.WithSegments(true))
		require.NoError(t, err)
		assert.Equal(t, []string{"Bonjour,", " tout le monde."}, texts)
		assert.Equal(t, transcript.Segments, segments)
		assert.Equal(t, "Bonjour, tout le monde.", transcript.Text)
		assert.Equal(t, "fr", transcript.Language)
		assert.Equal(t, uint(4), usage.Seconds)
	})

	t.Run("Translate", func(t *testing.T) {
		// The text, in English, which an English-only model doesn't
		// translate
		transcript, _, err := c.Translate(ctx, model, audio, nil)
		require.NoError(t, err)
		assert.Equal(t, "en", transcript.Language)
		assert.NotContains(t, fields(), "language")

		// And the language it's in, which it's translated from, is sent,
		// and the translation is in English
		transcript, _, err = c.Translate(ctx, model, audio, nil, llm.WithLanguage("fr"))
		require.NoError(t, err)
		assert.Equal(t, "en", transcript.Language)
		assert.Equal(t, "fr", fields()["language"])
		english, err := c.LoadModel(ctx, "ggml-tiny.en", nil)
		require.NoError(t, err)
		_, _, err = c.Translate(ctx, english, audio, nil)
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
	})

	t.Run("Errors", func(t *testing.T) {
		// An error streamed, audio which isn't audio, or is empty, and a
		// model which isn't go-whisper's
		broken := &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/wave", Data: []byte("error")}}
		_, _, err := c.Transcribe(ctx, model, broken, func(string, *llm.TranscriptSegment) {})
		assert.EqualError(t, err, "cannot decode audio")
		image := &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "image/png", Data: []byte("png")}}
		_, _, err = c.Transcribe(ctx, model, image, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		empty := &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/wave"}}
		_, _, err = c.Transcribe(ctx, model, empty, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, _, err = c.Transcribe(ctx, nil, audio, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

func Test_LiveTranscribe(t *testing.T) {
	// The speech, with the first of the models of a go-whisper server, at
	// GOWHISPER_ADDR, which is slow, so it's skipped with -short
	addr := os.Getenv("GOWHISPER_ADDR")
	if addr == "" || testing.Short() {
		t.Skip("GOWHISPER_ADDR isn't set, or -short")
	}
	c, err := gowhisper.New(addr)
	require.NoError(t, err)
	ctx := context.Background()
	models, err := c.ListModels(ctx)
	require.NoError(t, err)
	if len(models) == 0 {
		t.Skip("no models")
	}
	model := models[0]
	read := func(name string) llm.Resource {
		data, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
		require.NoError(t, err)
		return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: providerschema.ResourceType(data, name), Data: data}}
	}

	// Streamed, with its segments
	var pieces []string
	transcript, usage, err := c.Transcribe(ctx, model, read("speech-en.m4a"), func(text string, segment *llm.TranscriptSegment) {
		if segment == nil {
			pieces = append(pieces, text)
		}
	}, llm.WithSegments(true))
	require.NoError(t, err)
	t.Logf("%+v, %+v", transcript, usage)
	assert.Contains(t, strings.ToLower(transcript.Text), "quick brown fox")
	assert.Equal(t, transcript.Text, strings.Join(pieces, ""))
	assert.Equal(t, "en", transcript.Language)
	assert.NotEmpty(t, transcript.Segments)
	assert.Positive(t, usage.Seconds)

	// And translated
	if *model.Capabilities().Translation {
		transcript, _, err = c.Translate(ctx, model, read("speech-fr.wav"), nil)
		require.NoError(t, err)
		t.Logf("%+v", transcript)
		assert.Contains(t, strings.ToLower(transcript.Text), "hello")
	}
}
