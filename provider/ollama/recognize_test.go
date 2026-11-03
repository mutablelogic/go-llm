package ollama_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// recognizeServer is a stub Ollama server, with a model which recognizes
// text, one which sees images, and one which doesn't, which keeps the last
// request to chat, and replies with the text of the image
type recognizeServer struct {
	sync.Mutex
	request struct {
		Model     string         `json:"model"`
		KeepAlive string         `json:"keep_alive"`
		Options   map[string]any `json:"options"`
		Messages  []struct {
			Content string   `json:"content"`
			Images  [][]byte `json:"images"`
		} `json:"messages"`
	}
}

func (s *recognizeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/api/show":
		var req struct {
			Name string `json:"name"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		switch req.Name {
		case "deepseek-ocr:latest", "vision:latest":
			w.Write([]byte(`{"details":{},"capabilities":["completion","vision"]}`))
		default:
			w.Write([]byte(`{"details":{},"capabilities":["completion"]}`))
		}
	case "/api/chat":
		s.Lock()
		defer s.Unlock()
		json.NewDecoder(r.Body).Decode(&s.request)
		w.Write([]byte(`{"message":{"role":"assistant","content":"\n## Hello, World\n"},"done":true,"prompt_eval_count":900,"eval_count":12}`))
	default:
		http.NotFound(w, r)
	}
}

func newRecognizeClient(t *testing.T) (*ollama.Client, *recognizeServer) {
	t.Helper()
	stub := new(recognizeServer)
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	c, err := ollama.New(server.URL)
	require.NoError(t, err)
	return c, stub
}

func Test_ImplementsRecognizer(t *testing.T) {
	assert.Implements(t, (*llm.Recognizer)(nil), &ollama.Client{})
}

func Test_Recognize(t *testing.T) {
	c, stub := newRecognizeClient(t)
	ctx := context.Background()
	image := media{contentType: "image/png", data: []byte("\x89PNG")}
	model := func(name string) llm.Model {
		model, err := c.GetModel(ctx, name)
		require.NoError(t, err)
		return model
	}

	t.Run("Capabilities", func(t *testing.T) {
		// A model which recognizes text does, by its name, a model which
		// doesn't see images doesn't, and a model which sees images may
		assert.Equal(t, types.Ptr(true), model("deepseek-ocr:latest").Capabilities().OCR)
		assert.Equal(t, types.Ptr(false), model("chat:latest").Capabilities().OCR)
		assert.Nil(t, model("vision:latest").Capabilities().OCR)
	})

	t.Run("Image", func(t *testing.T) {
		// The image, with the model's own prompt, and its text as one page
		message, usage, err := c.Recognize(ctx, model("deepseek-ocr:latest"), image, llm.WithPages(0), llm.WithTableFormat("markdown"), llm.WithKeepAlive(-1))
		require.NoError(t, err)
		assert.Equal(t, "assistant", message.Role())
		assert.Equal(t, "## Hello, World", message.Text())
		assert.Equal(t, llm.Usage{InputTokens: 900, OutputTokens: 12, Pages: 1}, usage)
		stub.Lock()
		defer stub.Unlock()
		assert.Equal(t, "deepseek-ocr:latest", stub.request.Model)
		assert.Equal(t, "-1ns", stub.request.KeepAlive)
		assert.Equal(t, float64(0), stub.request.Options["temperature"])
		if assert.Len(t, stub.request.Messages, 1) {
			assert.Equal(t, "Convert the document to markdown.", stub.request.Messages[0].Content)
			assert.Equal(t, [][]byte{image.data}, stub.request.Messages[0].Images)
		}
	})

	t.Run("Errors", func(t *testing.T) {
		// Ollama reads images, which have one page, zero, and tables in
		// Markdown, and not other options
		ocr := model("deepseek-ocr:latest")
		for _, test := range []struct {
			resource llm.Resource
			opts     []llm.Opt
		}{
			{media{contentType: "application/pdf", data: []byte("%PDF")}, nil},
			{media{contentType: "image/png"}, nil},
			{image, []llm.Opt{llm.WithPages(1)}},
			{image, []llm.Opt{llm.WithTableFormat("html")}},
			{image, []llm.Opt{llm.WithTemperature(0.5)}},
		} {
			_, _, err := c.Recognize(ctx, ocr, test.resource, test.opts...)
			assert.ErrorIs(t, err, llm.ErrBadParameter, test.resource.Type())
		}
		_, _, err := c.Recognize(ctx, ocr, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

func Test_Recognize_Live(t *testing.T) {
	// The text of an image, with deepseek-ocr, or the model in
	// OLLAMA_OCR_MODEL, which is slow, so it's skipped with -short
	if testing.Short() {
		t.Skip("skipping recognition with -short")
	}
	c := requireClient(t)
	name := os.Getenv("OLLAMA_OCR_MODEL")
	if name == "" {
		name = "deepseek-ocr:latest"
	}
	model, err := c.GetModel(context.Background(), name)
	if err != nil {
		t.Skipf("%s isn't available: %v", name, err)
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ocr.png"))
	require.NoError(t, err)
	message, usage, err := c.Recognize(context.Background(), model, &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/png", Data: data}})
	require.NoError(t, err)
	t.Logf("%q, %+v", message.Text(), usage)
	assert.Contains(t, message.Text(), "Hello, World")
	assert.Equal(t, uint(1), usage.Pages)
}
