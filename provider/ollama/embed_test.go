package ollama_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// embedServer is a stub Ollama server, whose models embed text, images and
// audio, only text, or nothing, which embeds each input as a vector of its
// text's length, whether it has an image and audio, and its position, and
// records the last request
type embedServer struct {
	sync.Mutex
	request map[string]any
}

func (s *embedServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/show":
		var req struct {
			Name string `json:"name"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		capabilities := `["embedding","vision","audio"]`
		switch req.Name {
		case "textonly:latest":
			capabilities = `["embedding"]`
		case "llama3.2:latest":
			capabilities = `["completion","tools"]`
		case "unknown:latest", "short:latest":
			capabilities = `[]`
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"details":{"family":"gemma3"},"capabilities":` + capabilities + `,"model_info":{"gemma3.context_length":2048}}`))
	case r.Method == http.MethodPost && r.URL.Path == "/api/embed":
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		s.Lock()
		s.request = req
		s.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if req["model"] == "short:latest" {
			w.Write([]byte(`{"model":"short:latest","embeddings":[[1,2]],"prompt_eval_count":3}`))
			return
		}
		inputs, _ := req["input"].([]any)
		embeddings := make([][]float32, len(inputs))
		for i, input := range inputs {
			switch v := input.(type) {
			case string:
				embeddings[i] = []float32{float32(len(v)), 0, 0, float32(i)}
			case map[string]any:
				text, _ := v["text"].(string)
				embeddings[i] = []float32{float32(len(text)), boolean(v["image"] != nil), boolean(v["audio"] != nil), float32(i)}
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"model": req["model"], "embeddings": embeddings, "prompt_eval_count": 12})
	default:
		http.NotFound(w, r)
	}
}

// boolean returns one when the value is true, and zero otherwise
func boolean(value bool) float32 {
	if value {
		return 1
	}
	return 0
}

// mediaMessage is a message to embed, with resources
type mediaMessage struct {
	text      string
	resources []llm.Resource
}

func (m mediaMessage) Role() string                  { return "user" }
func (m mediaMessage) Text() string                  { return m.text }
func (m mediaMessage) Resources() []llm.Resource     { return m.resources }
func (m mediaMessage) ToolCalls() []llm.ToolCall     { return nil }
func (m mediaMessage) ToolResults() []llm.ToolResult { return nil }

// media is a resource with its type and data
type media struct {
	contentType string
	data        []byte
}

func (r media) URI() string         { return "" }
func (r media) Name() string        { return "" }
func (r media) Description() string { return "" }
func (r media) Type() string        { return r.contentType }
func (r media) Read(context.Context) (llm.ReadFunc, error) {
	return llm.ReadBytes(r.data), nil
}

// link is a resource referred to by a URL, which can't be read
type link string

func (r link) URI() string         { return "" }
func (r link) Name() string        { return "" }
func (r link) Description() string { return "" }
func (r link) Type() string        { return string(r) }
func (r link) Read(context.Context) (llm.ReadFunc, error) {
	return nil, llm.ErrNotImplemented.With("reading resource url")
}

func newEmbedClient(t *testing.T) (*ollama.Client, *embedServer) {
	t.Helper()
	stub := new(embedServer)
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	c, err := ollama.New(server.URL)
	require.NoError(t, err)
	return c, stub
}

func Test_ImplementsEmbedder(t *testing.T) {
	assert.Implements(t, (*llm.Embedder)(nil), &ollama.Client{})
}

func Test_Embed(t *testing.T) {
	c, stub := newEmbedClient(t)
	ctx := context.Background()
	model, err := c.GetModel(ctx, "embeddinggemma-2:latest")
	require.NoError(t, err)
	messages := []llm.Message{message{role: "user", text: "Hello"}, message{role: "user", text: "Goodbye, world"}}

	// request returns the inputs of the last request
	request := func() []any {
		stub.Lock()
		defer stub.Unlock()
		inputs, _ := stub.request["input"].([]any)
		return inputs
	}

	t.Run("Vectors", func(t *testing.T) {
		// A vector for each message, in order, in one request, with the
		// tokens in their texts, which are sent as text
		vectors, usage, err := c.Embed(ctx, model, messages)
		require.NoError(t, err)
		assert.Equal(t, []llm.Vector{{5, 0, 0, 0}, {14, 0, 0, 1}}, vectors)
		assert.Equal(t, llm.Usage{InputTokens: 12}, usage)
		assert.Equal(t, []any{"Hello", "Goodbye, world"}, request())
		stub.Lock()
		defer stub.Unlock()
		assert.Equal(t, "embeddinggemma-2:latest", stub.request["model"])
		assert.NotContains(t, stub.request, "dimensions")
	})

	t.Run("Media", func(t *testing.T) {
		// A message with an image and audio is an input with its text and
		// media, as base64, and a message with only media has no text
		vectors, _, err := c.Embed(ctx, model, []llm.Message{
			mediaMessage{text: "A cat", resources: []llm.Resource{media{"image/png", []byte("png")}, media{"audio/wav; codecs=1", []byte("wav")}}},
			mediaMessage{resources: []llm.Resource{media{"image/jpeg", []byte("jpeg")}}},
			message{role: "user", text: "Text"},
		})
		require.NoError(t, err)
		assert.Equal(t, []llm.Vector{{5, 1, 1, 0}, {0, 1, 0, 1}, {4, 0, 0, 2}}, vectors)
		assert.Equal(t, []any{
			map[string]any{"text": "A cat", "image": base64.StdEncoding.EncodeToString([]byte("png")), "audio": base64.StdEncoding.EncodeToString([]byte("wav"))},
			map[string]any{"image": base64.StdEncoding.EncodeToString([]byte("jpeg"))},
			"Text",
		}, request())
	})

	t.Run("MediaErrors", func(t *testing.T) {
		// A model without the modality can't embed it, nor can any model a
		// document, two images, or empty media
		textOnly, err := c.GetModel(ctx, "textonly:latest")
		require.NoError(t, err)
		image := mediaMessage{resources: []llm.Resource{media{"image/png", []byte("png")}}}
		_, _, err = c.Embed(ctx, textOnly, []llm.Message{image})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		for _, resources := range [][]llm.Resource{
			{media{"application/pdf", []byte("pdf")}},
			{media{"image/png", []byte("a")}, media{"image/png", []byte("b")}},
			{media{"audio/mpeg", nil}},
		} {
			_, _, err = c.Embed(ctx, model, []llm.Message{mediaMessage{resources: resources}})
			assert.ErrorIs(t, err, llm.ErrBadParameter, resources)
		}

		// A resource which can't be read keeps its error
		_, _, err = c.Embed(ctx, model, []llm.Message{mediaMessage{resources: []llm.Resource{link("image/png")}}})
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
		assert.NotErrorIs(t, err, llm.ErrBadParameter)

		// A model whose capabilities Ollama didn't report is sent the media
		unknown, err := c.GetModel(ctx, "unknown:latest")
		require.NoError(t, err)
		_, _, err = c.Embed(ctx, unknown, []llm.Message{image})
		assert.NoError(t, err)
	})

	t.Run("Options", func(t *testing.T) {
		// The dimensions, context window and keep-alive are Ollama's, and
		// the task type and title, which Ollama hasn't, are ignored
		_, _, err := c.Embed(ctx, model, messages, llm.WithDimensions(256), llm.WithContextWindow(1024), llm.WithKeepAlive(time.Minute), llm.WithTaskType("retrieval_document"), llm.WithTitle("Notes"))
		require.NoError(t, err)
		stub.Lock()
		defer stub.Unlock()
		assert.Equal(t, float64(256), stub.request["dimensions"])
		assert.Equal(t, map[string]any{"num_ctx": float64(1024)}, stub.request["options"])
		assert.Equal(t, "1m0s", stub.request["keep_alive"])
		assert.NotContains(t, stub.request, "task_type")

		// A generation option isn't supported
		_, _, err = c.Embed(ctx, model, messages, llm.WithTemperature(0.5))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Errors", func(t *testing.T) {
		// Messages are needed, each with text or media, and an Ollama model
		// which embeds
		_, _, err := c.Embed(ctx, model, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, _, err = c.Embed(ctx, model, []llm.Message{message{role: "user", text: ""}})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, _, err = c.Embed(ctx, model, []llm.Message{nil})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		other, err := eliza.New()
		require.NoError(t, err)
		elizaModel, err := other.GetModel(ctx, eliza.ModelName)
		require.NoError(t, err)
		_, _, err = c.Embed(ctx, elizaModel, messages)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		chat, err := c.GetModel(ctx, "llama3.2:latest")
		require.NoError(t, err)
		_, _, err = c.Embed(ctx, chat, messages)
		assert.ErrorIs(t, err, llm.ErrBadParameter)

		// A reply without a vector for each message is an error
		short, err := c.GetModel(ctx, "short:latest")
		require.NoError(t, err)
		_, _, err = c.Embed(ctx, short, messages)
		assert.ErrorIs(t, err, llm.ErrInternalServerError)
	})
}

// liveEmbedModel returns the embedding model of a running Ollama, which is
// OLLAMA_EMBED_MODEL or embeddinggemma-2, or skips the test
func liveEmbedModel(t *testing.T) (*ollama.Client, llm.Model) {
	t.Helper()
	c := requireClient(t)
	name := os.Getenv("OLLAMA_EMBED_MODEL")
	if name == "" {
		name = "embeddinggemma-2:latest"
	}
	model, err := c.GetModel(context.Background(), name)
	if err != nil {
		t.Skipf("%s isn't available: %v", name, err)
	}
	return c, model
}

// cosine returns the cosine similarity of two vectors
func cosine(a, b llm.Vector) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func Test_Embed_Live(t *testing.T) {
	// Texts which mean the same are nearer each other than one which doesn't
	c, model := liveEmbedModel(t)
	ctx := context.Background()
	vectors, usage, err := c.Embed(ctx, model, []llm.Message{
		message{role: "user", text: "The cat sat on the mat"},
		message{role: "user", text: "A kitten is resting on a rug"},
		message{role: "user", text: "Quarterly revenue grew by four percent"},
	})
	require.NoError(t, err)
	require.Len(t, vectors, 3)
	assert.NotEmpty(t, vectors[0])
	assert.Positive(t, usage.InputTokens)
	t.Logf("%d dimensions, %d tokens; similar %.3f, different %.3f", len(vectors[0]), usage.InputTokens, cosine(vectors[0], vectors[1]), cosine(vectors[0], vectors[2]))
	assert.Greater(t, cosine(vectors[0], vectors[1]), cosine(vectors[0], vectors[2]))

	// Fewer dimensions, which embeddinggemma supports
	vectors, _, err = c.Embed(ctx, model, []llm.Message{message{role: "user", text: "Hello"}}, llm.WithDimensions(256))
	require.NoError(t, err)
	assert.Len(t, vectors[0], 256)
}

func Test_Embed_Live_Media(t *testing.T) {
	// An image of a red square is nearer the text "a red square" than an
	// image of a blue one is, and audio is embedded, for a model with vision
	// and audio, such as embeddinggemma-2 on Ollama's MLX runner
	c, model := liveEmbedModel(t)
	ctx := context.Background()
	square := func(fill color.Color) []byte {
		img := image.NewRGBA(image.Rect(0, 0, 224, 224))
		for y := range 224 {
			for x := range 224 {
				img.Set(x, y, color.White)
				if x > 48 && x < 176 && y > 48 && y < 176 {
					img.Set(x, y, fill)
				}
			}
		}
		var buf bytes.Buffer
		require.NoError(t, png.Encode(&buf, img))
		return buf.Bytes()
	}
	vectors, _, err := c.Embed(ctx, model, []llm.Message{
		message{role: "user", text: "a red square"},
		mediaMessage{resources: []llm.Resource{media{"image/png", square(color.RGBA{220, 0, 0, 255})}}},
		mediaMessage{resources: []llm.Resource{media{"image/png", square(color.RGBA{0, 0, 220, 255})}}},
		mediaMessage{resources: []llm.Resource{media{"audio/wav", tone(t, 440, time.Second)}}},
	})
	require.NoError(t, err)
	require.Len(t, vectors, 4)
	t.Logf("text to red %.3f, to blue %.3f; audio %d dimensions", cosine(vectors[0], vectors[1]), cosine(vectors[0], vectors[2]), len(vectors[3]))
	assert.Greater(t, cosine(vectors[0], vectors[1]), cosine(vectors[0], vectors[2]))
	assert.Len(t, vectors[3], len(vectors[0]))
}

// tone returns a WAV of a sine wave at the frequency, for the duration, as
// 16-bit mono PCM at 16 kHz
func tone(t *testing.T, frequency float64, duration time.Duration) []byte {
	t.Helper()
	const rate = 16000
	samples := int(duration.Seconds() * rate)
	var data bytes.Buffer
	for i := range samples {
		binary.Write(&data, binary.LittleEndian, int16(math.Sin(2*math.Pi*frequency*float64(i)/rate)*16000))
	}
	var buf bytes.Buffer
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+data.Len()))
	buf.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(rate), uint32(rate * 2), uint16(2), uint16(16)} {
		binary.Write(&buf, binary.LittleEndian, v)
	}
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(data.Len()))
	buf.Write(data.Bytes())
	return buf.Bytes()
}
