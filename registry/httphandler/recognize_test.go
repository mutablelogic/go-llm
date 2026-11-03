package httphandler_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// stubRecognizer is a stub Ollama server, with a model which recognizes
// text, and one which doesn't see images, which keeps the last image, and
// replies with its text, or with the status of an image which is a status,
// such as "status 500"
type stubRecognizer struct {
	sync.Mutex
	image   []byte
	options map[string]any
}

func (s *stubRecognizer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string         `json:"name"`
		Options  map[string]any `json:"options"`
		Messages []struct {
			Images [][]byte `json:"images"`
		} `json:"messages"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/api/version":
		w.Write([]byte(`{"version":"0.0.1"}`))
	case "/api/tags":
		w.Write([]byte(`{"models":[{"name":"deepseek-ocr:latest","details":{}},{"name":"chat:latest","details":{}}]}`))
	case "/api/show":
		if req.Name == "deepseek-ocr:latest" {
			w.Write([]byte(`{"details":{},"capabilities":["completion","vision"]}`))
		} else {
			w.Write([]byte(`{"details":{},"capabilities":["completion"]}`))
		}
	case "/api/chat":
		var image []byte
		if len(req.Messages) > 0 && len(req.Messages[0].Images) > 0 {
			image = req.Messages[0].Images[0]
		}
		if code, ok := strings.CutPrefix(string(image), "status "); ok {
			status, _ := strconv.Atoi(code)
			w.WriteHeader(status)
			w.Write([]byte(`{"error":"upstream error"}`))
			return
		}
		s.Lock()
		s.image, s.options = image, req.Options
		s.Unlock()
		w.Write([]byte(`{"message":{"role":"assistant","content":"# Hello"},"done":true,"prompt_eval_count":900,"eval_count":3}`))
	default:
		http.NotFound(w, r)
	}
}

// Image returns the last image recognized
func (s *stubRecognizer) Image() []byte {
	s.Lock()
	defer s.Unlock()
	return s.image
}

// newRecognizeServer returns a server of a registry with a stub Ollama
// server, as home, which recognizes text, and eliza, which doesn't
func newRecognizeServer(t *testing.T) (*test.Server, *stubRecognizer) {
	t.Helper()
	stub := new(stubRecognizer)
	ollama := httptest.NewServer(stub)
	t.Cleanup(ollama.Close)
	r := test.NewRegistry(t, nil)
	_, err := r.CreateProvider(t.Context(), schema.Provider{Name: "home", Type: llm.Ollama, URL: ollama.URL})
	require.NoError(t, err)
	_, err = r.CreateProvider(t.Context(), schema.Provider{Type: llm.Eliza})
	require.NoError(t, err)
	return test.NewServer(t, r), stub
}

func Test_Recognize(t *testing.T) {
	server, stub := newRecognizeServer(t)
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

	t.Run("JSON", func(t *testing.T) {
		// The text of the image, as a page, with the model's canonical
		// name, and the page and tokens used
		var response schema.RecognizeResponse
		require.Equal(t, http.StatusOK, do(t, server, http.MethodPost, "llm/recognize", schema.RecognizeRequest{
			Model: "deepseek-ocr",
			Type:  "image/png",
			Data:  png,
			Opt:   map[string]json.RawMessage{"pages": json.RawMessage(`"0"`)},
		}, &response))
		assert.Equal(t, "home:deepseek-ocr:latest", response.Model)
		assert.Equal(t, "assistant", response.Message.Role())
		assert.Equal(t, "# Hello", response.Message.Text())
		assert.Equal(t, llm.Usage{InputTokens: 900, OutputTokens: 3, Pages: 1}, response.Usage)
		assert.Equal(t, png, stub.Image())
	})

	t.Run("Form", func(t *testing.T) {
		// A form has the file, whose type is detected when it's generic,
		// and the options as JSON
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		require.NoError(t, form.WriteField("model", "deepseek-ocr"))
		require.NoError(t, form.WriteField("opt", `{"table_format":"markdown"}`))
		file, err := form.CreateFormFile("file", "page")
		require.NoError(t, err)
		_, err = file.Write(png)
		require.NoError(t, err)
		require.NoError(t, form.Close())
		status, data := server.DoWithContentType(t, http.MethodPost, "llm/recognize", form.FormDataContentType(), body.Bytes())
		require.Equal(t, http.StatusOK, status, string(data))
		var response schema.RecognizeResponse
		require.NoError(t, json.Unmarshal(data, &response))
		assert.Equal(t, "# Hello", response.Message.Text())
		assert.Equal(t, png, stub.Image())
	})

	t.Run("FormErrors", func(t *testing.T) {
		// A form needs a file, and only one
		post := func(files ...string) int {
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			require.NoError(t, form.WriteField("model", "deepseek-ocr"))
			for _, name := range files {
				file, err := form.CreateFormFile("file", name)
				require.NoError(t, err)
				file.Write(png)
			}
			require.NoError(t, form.Close())
			status, _ := server.DoWithContentType(t, http.MethodPost, "llm/recognize", form.FormDataContentType(), body.Bytes())
			return status
		}
		assert.Equal(t, http.StatusBadRequest, post())
		assert.Equal(t, http.StatusBadRequest, post("a.png", "b.png"))
	})

	t.Run("Errors", func(t *testing.T) {
		post := func(req schema.RecognizeRequest) int {
			return do(t, server, http.MethodPost, "llm/recognize", req, nil)
		}
		image := func(model string, data []byte, opt map[string]json.RawMessage) schema.RecognizeRequest {
			return schema.RecognizeRequest{Model: model, Type: "image/png", Data: data, Opt: opt}
		}

		// A request needs a model, a type, and data
		assert.Equal(t, http.StatusBadRequest, post(image("", png, nil)))
		assert.Equal(t, http.StatusBadRequest, post(schema.RecognizeRequest{Model: "deepseek-ocr", Data: png}))
		assert.Equal(t, http.StatusBadRequest, post(image("deepseek-ocr", nil, nil)))

		// And a model which exists, and recognizes text, whose provider
		// does, and can read the document, with the options
		assert.Equal(t, http.StatusNotFound, post(image("missing", png, nil)))
		assert.Equal(t, http.StatusNotImplemented, post(image("home:chat:latest", png, nil)))
		assert.Equal(t, http.StatusNotImplemented, post(image("eliza", png, nil)))
		assert.Equal(t, http.StatusBadRequest, post(schema.RecognizeRequest{Model: "deepseek-ocr", Type: "application/pdf", Data: []byte("%PDF")}))
		assert.Equal(t, http.StatusBadRequest, post(image("deepseek-ocr", png, map[string]json.RawMessage{"pages": json.RawMessage(`[1]`)})))
		assert.Equal(t, http.StatusBadRequest, post(image("deepseek-ocr", png, map[string]json.RawMessage{"table_format": json.RawMessage(`"html"`)})))

		// And a provider which rejects the request is a request which isn't
		// valid, but a provider which fails otherwise is unavailable
		for status, want := range map[int]int{
			http.StatusBadRequest:          http.StatusBadRequest,
			http.StatusInternalServerError: http.StatusServiceUnavailable,
		} {
			assert.Equal(t, want, post(image("deepseek-ocr", []byte("status "+strconv.Itoa(status)), nil)), status)
		}
	})
}
