package httphandler_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// stubTranscriber is a stub OpenAI-compatible server, with a model which
// transcribes speech, which keeps the fields of the last form, and replies
// with a transcript, whose segments are those asked for, or with the status
// of audio which is a status, such as "status 500"
type stubTranscriber struct {
	sync.Mutex
	path   string
	fields map[string]string
	audio  string
}

func (s *stubTranscriber) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/v1/models":
		w.Write([]byte(`{"object":"list","data":[{"id":"whisper-1","object":"model"}]}`))
	case "/v1/audio/transcriptions", "/v1/audio/translations":
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fields := make(map[string]string)
		for key, values := range r.MultipartForm.Value {
			fields[key] = values[0]
		}
		var audio string
		if files := r.MultipartForm.File["file"]; len(files) > 0 {
			fields["filename"] = files[0].Filename
			if f, err := files[0].Open(); err == nil {
				var data [64]byte
				n, _ := f.Read(data[:])
				audio = string(data[:n])
				f.Close()
			}
		}
		if code, ok := strings.CutPrefix(audio, "status "); ok {
			status, _ := strconv.Atoi(code)
			w.WriteHeader(status)
			w.Write([]byte(`{"error":{"message":"upstream error"}}`))
			return
		}
		s.Lock()
		s.path, s.fields, s.audio = r.URL.Path, fields, audio
		s.Unlock()
		w.Write([]byte(`{"text":"Hello, world.","language":"english","segments":[{"start":0,"end":1.5,"text":" Hello,"},{"start":1.5,"end":2.25,"text":" world."}],"usage":{"type":"duration","seconds":3}}`))
	default:
		http.NotFound(w, r)
	}
}

// Last returns the path, and fields, of the last request
func (s *stubTranscriber) Last() (string, map[string]string) {
	s.Lock()
	defer s.Unlock()
	return s.path, s.fields
}

// newTranscribeServer returns a server of a registry with a stub
// OpenAI-compatible server, as speech, which transcribes speech, and eliza,
// which doesn't
func newTranscribeServer(t *testing.T) (*test.Server, *stubTranscriber) {
	t.Helper()
	stub := new(stubTranscriber)
	openai := httptest.NewServer(stub)
	t.Cleanup(openai.Close)
	r := test.NewRegistry(t, nil)
	_, err := r.CreateProvider(t.Context(), schema.Provider{Name: "speech", Type: llm.OpenAI, URL: openai.URL})
	require.NoError(t, err)
	_, err = r.CreateProvider(t.Context(), schema.Provider{Type: llm.Eliza})
	require.NoError(t, err)
	return test.NewServer(t, r), stub
}

func Test_Transcribe(t *testing.T) {
	server, stub := newTranscribeServer(t)
	wav := []byte("RIFF\x24\x00\x00\x00WAVEfmt ")
	audio := func(path, model string, data []byte, opt map[string]json.RawMessage) (int, schema.TranscribeResponse) {
		var response schema.TranscribeResponse
		status := do(t, server, http.MethodPost, "llm/"+path, schema.TranscribeRequest{Model: model, Type: "audio/wav", Data: data, Opt: opt}, &response)
		return status, response
	}

	t.Run("Transcribe", func(t *testing.T) {
		// The text, its language, as a code, with the model's canonical
		// name, and the seconds, and the audio is a file named for its type
		status, response := audio("transcribe", "whisper-1", wav, nil)
		require.Equal(t, http.StatusOK, status)
		assert.Equal(t, "speech:whisper-1", response.Model)
		assert.Equal(t, "Hello, world.", response.Text)
		assert.Equal(t, "en", response.Language)
		assert.Empty(t, response.Segments)
		assert.NotZero(t, response.Usage.Duration, "how long the model took")
		response.Usage.Duration = 0
		assert.Equal(t, llm.Usage{Seconds: 3}, response.Usage)
		path, fields := stub.Last()
		assert.Equal(t, "/v1/audio/transcriptions", path)
		assert.Equal(t, "whisper-1", fields["model"])
		assert.Equal(t, "audio.wav", fields["filename"])
		assert.Equal(t, "verbose_json", fields["response_format"])
	})

	t.Run("Segments", func(t *testing.T) {
		// The segments, when they're asked for, and the language set
		status, response := audio("transcribe", "whisper-1", wav, map[string]json.RawMessage{
			"segments": json.RawMessage(`true`),
			"language": json.RawMessage(`"fr"`),
		})
		require.Equal(t, http.StatusOK, status)
		assert.Equal(t, []llm.TranscriptSegment{
			{Start: 0, End: 1500 * time.Millisecond, Text: "Hello,"},
			{Start: 1500 * time.Millisecond, End: 2250 * time.Millisecond, Text: "world."},
		}, response.Segments)
		_, fields := stub.Last()
		assert.Equal(t, "segment", fields["timestamp_granularities[]"])
		assert.Equal(t, "fr", fields["language"])
	})

	t.Run("Stream", func(t *testing.T) {
		// With text/event-stream, the text, and the segments, which a model
		// which doesn't stream has once it's transcribed, and then the
		// transcript
		data, err := json.Marshal(schema.TranscribeRequest{Model: "whisper-1", Type: "audio/wav", Data: wav, Opt: map[string]json.RawMessage{"segments": json.RawMessage(`true`)}})
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+test.Prefix+"/llm/transcribe", bytes.NewReader(data))
		require.NoError(t, err)
		req.Header.Set("Content-Type", types.ContentTypeJSON)
		req.Header.Set("Accept", types.ContentTypeTextStream)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var name, text string
		var segments []llm.TranscriptSegment
		var response schema.TranscribeResponse
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if value, ok := strings.CutPrefix(line, "event: "); ok {
				name = value
			} else if value, ok := strings.CutPrefix(line, "data: "); ok {
				switch name {
				case schema.EventText:
					var event schema.StreamEvent
					require.NoError(t, json.Unmarshal([]byte(value), &event))
					text += event.Text
				case schema.EventSegment:
					var segment llm.TranscriptSegment
					require.NoError(t, json.Unmarshal([]byte(value), &segment))
					segments = append(segments, segment)
				case schema.EventResult:
					require.NoError(t, json.Unmarshal([]byte(value), &response))
				case schema.EventError:
					t.Fatalf("error event: %s", value)
				}
			}
		}
		assert.Equal(t, "Hello, world.", text)
		assert.Equal(t, response.Segments, segments)
		assert.Len(t, segments, 2)
		assert.Equal(t, "speech:whisper-1", response.Model)
	})

	t.Run("Translate", func(t *testing.T) {
		// The text, in English
		status, response := audio("translate", "whisper-1", wav, nil)
		require.Equal(t, http.StatusOK, status)
		assert.Equal(t, "Hello, world.", response.Text)
		assert.Equal(t, "en", response.Language)
		path, _ := stub.Last()
		assert.Equal(t, "/v1/audio/translations", path)
	})

	t.Run("Errors", func(t *testing.T) {
		// A request needs a model, a type, and data
		status, _ := audio("transcribe", "", wav, nil)
		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPost, "llm/transcribe", schema.TranscribeRequest{Model: "whisper-1", Data: wav}, nil))
		status, _ = audio("transcribe", "whisper-1", nil, nil)
		assert.Equal(t, http.StatusBadRequest, status)

		// And a model which exists, whose provider transcribes, with audio,
		// and the options, which for a translation have no language
		status, _ = audio("transcribe", "missing", wav, nil)
		assert.Equal(t, http.StatusNotFound, status)
		status, _ = audio("transcribe", "eliza", wav, nil)
		assert.Equal(t, http.StatusNotImplemented, status)
		status, _ = audio("translate", "eliza", wav, nil)
		assert.Equal(t, http.StatusNotImplemented, status)
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPost, "llm/transcribe", schema.TranscribeRequest{Model: "whisper-1", Type: "image/png", Data: wav}, nil))
		status, _ = audio("transcribe", "whisper-1", wav, map[string]json.RawMessage{"speakers": json.RawMessage(`true`)})
		assert.Equal(t, http.StatusBadRequest, status)
		status, _ = audio("translate", "whisper-1", wav, map[string]json.RawMessage{"language": json.RawMessage(`"fr"`)})
		assert.Equal(t, http.StatusBadRequest, status)

		// And a provider which rejects the request is a request which isn't
		// valid, but a provider which fails otherwise is unavailable
		for code, want := range map[int]int{
			http.StatusBadRequest:          http.StatusBadRequest,
			http.StatusInternalServerError: http.StatusServiceUnavailable,
		} {
			status, _ = audio("transcribe", "whisper-1", []byte("status "+strconv.Itoa(code)), nil)
			assert.Equal(t, want, status, code)
		}
	})
}
