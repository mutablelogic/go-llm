package lmstudio_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	lmstudio "github.com/mutablelogic/go-llm/provider/lmstudio"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// TEST SET-UP

// The live tests ask the LM Studio server at LMSTUDIO_ADDR, such as
// "http://localhost:1234/v1", and are skipped without it. They load the
// models, LMSTUDIO_MODEL, which calls tools, LMSTUDIO_EMBED, which embeds,
// and LMSTUDIO_OCR_MODEL, which recognizes text, when it's stored, and unload
// them.
var (
	lmstudioAddr  = os.Getenv("LMSTUDIO_ADDR")
	lmstudioModel = envOr("LMSTUDIO_MODEL", "google/gemma-4-12b-qat")
	lmstudioEmbed = envOr("LMSTUDIO_EMBED", "text-embedding-nomic-embed-text-v1.5")
	lmstudioOCR   = envOr("LMSTUDIO_OCR_MODEL", "allenai/olmocr-2-7b")
)

func envOr(key, value string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return value
}

// requireClient returns a client of the server at LMSTUDIO_ADDR, or skips
// the test, when it isn't set
func requireClient(t *testing.T) *lmstudio.Client {
	t.Helper()
	if lmstudioAddr == "" {
		t.Skip("LMSTUDIO_ADDR not set, skipping")
	}
	c, err := lmstudio.New(lmstudioAddr, os.Getenv("LMSTUDIO_API_KEY"))
	require.NoError(t, err)
	return c
}

// message is a message of the person's
func message(text string) llm.Message {
	return &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr(text)}}}}
}

// weather is a tool which says the weather in a city
type weather struct{}

type weatherInput struct {
	City string `json:"city" help:"The city"`
}

func (weather) Name() string                                      { return "get_weather" }
func (weather) Title() string                                     { return "" }
func (weather) Description() string                               { return "Returns the weather in a city" }
func (weather) InputSchema() *jsonschema.Schema                   { return jsonschema.MustFor[weatherInput]() }
func (weather) OutputSchema() *jsonschema.Schema                  { return nil }
func (weather) Timeout() time.Duration                            { return 0 }
func (weather) Run(context.Context, json.RawMessage) (any, error) { return "4°C and cloudy", nil }

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_Live(t *testing.T) {
	c := requireClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	require.NoError(t, c.Ping(ctx))

	t.Run("Chat", func(t *testing.T) {
		// The model is loaded, calls the tool, and is unloaded
		model, err := c.LoadModel(ctx, lmstudioModel, func(status string, percent float64) {
			t.Logf("%s %.0f%%", status, percent)
		})
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, c.UnloadModel(context.Background(), model)) })
		assert.True(t, model.(*lmstudio.Model).Loaded())
		t.Logf("%s: %s, %d tokens", model.Name(), model.Capabilities(), types.Value(model.InputTokenLimit()))

		reply, usage, err := c.WithoutSession(ctx, model, []llm.Message{message("What's the weather in Berlin? Use the tool.")}, []llm.Tool{weather{}}, nil)
		require.NoError(t, err)
		require.NotEmpty(t, reply.ToolCalls(), reply.Text())
		assert.Equal(t, "get_weather", reply.ToolCalls()[0].Name)
		assert.Contains(t, string(reply.ToolCalls()[0].Input), "Berlin")
		assert.NotZero(t, usage.InputTokens)
	})

	t.Run("Recognize", func(t *testing.T) {
		// The OCR model is only loaded when it's stored, as it isn't
		// downloaded for a test
		if _, err := c.GetModel(ctx, lmstudioOCR); err != nil {
			t.Skipf("%s isn't stored: %v", lmstudioOCR, err)
		}
		model, err := c.LoadModel(ctx, lmstudioOCR, nil)
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, c.UnloadModel(context.Background(), model)) })
		assert.True(t, types.Value(model.Capabilities().OCR))

		data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ocr.png"))
		require.NoError(t, err)
		image := &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "image/png", Data: data}}
		document, usage, err := c.Recognize(ctx, model, image)
		require.NoError(t, err)
		require.Len(t, document.Pages, 1)
		t.Logf("%q, %+v", document.Pages[0].Text, usage)
		assert.Contains(t, document.Pages[0].Text, "Hello, World")
		assert.NotContains(t, document.Pages[0].Text, "primary_language", "without its front matter")
		assert.Equal(t, uint(1), usage.Pages)
	})

	t.Run("Embed", func(t *testing.T) {
		model, err := c.LoadModel(ctx, lmstudioEmbed, nil)
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, c.UnloadModel(context.Background(), model)) })
		vectors, _, err := c.Embed(ctx, model, []llm.Message{message("The weather in Berlin")})
		require.NoError(t, err)
		require.Len(t, vectors, 1)
		assert.NotEmpty(t, vectors[0])
	})
}
