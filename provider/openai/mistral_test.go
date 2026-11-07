package openai_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	openai "github.com/mutablelogic/go-llm/provider/openai"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// The tests in this file make requests to Mistral's API, so they're skipped
// unless MISTRAL_API_KEY is set, and with -short. As replies vary, they
// check what replies are like, rather than what they say.

const (
	// A cheap model, which reasons when it's asked to, with an effort of
	// "none" or "high"
	mistralModel = "mistral-small-latest"

	// An embedding model, and one which can have fewer dimensions
	mistralEmbeddingModel  = "mistral-embed"
	mistralDimensionsModel = "codestral-embed"

	// A model which recognizes text, and one which transcribes speech
	mistralOCRModel        = "mistral-ocr-latest"
	mistralTranscribeModel = "voxtral-mini-latest"
)

// mistralClient returns a client of Mistral's API, with the key in
// MISTRAL_API_KEY, or skips the test when it isn't set, or with -short
func mistralClient(t *testing.T) *openai.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping requests to Mistral's API with -short")
	}
	key := os.Getenv("MISTRAL_API_KEY")
	if key == "" {
		t.Skip("MISTRAL_API_KEY isn't set")
	}
	c, err := openai.New(openai.MistralEndpoint, key)
	require.NoError(t, err)
	return c
}

func Test_LiveMistral(t *testing.T) {
	c := mistralClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
	defer cancel()
	model := func(t *testing.T, name string) llm.Model {
		t.Helper()
		model, err := c.GetModel(ctx, name)
		require.NoError(t, err, name)
		return model
	}

	t.Run("Models", func(t *testing.T) {
		// A model has its description, context length, and aliases, and
		// some model has the "-latest" name as an alias
		models, err := c.ListModels(ctx)
		require.NoError(t, err)
		aliased := false
		for _, model := range models {
			for _, alias := range model.Aliases() {
				aliased = aliased || alias == mistralModel
			}
		}
		assert.True(t, aliased, "no model has %q as an alias", mistralModel)
		small := model(t, mistralModel)
		assert.NotEmpty(t, small.Description())
		if assert.NotNil(t, small.InputTokenLimit()) {
			assert.Greater(t, *small.InputTokenLimit(), uint(0))
		}

		// And the capabilities Mistral reports, and that its embedding
		// models embed, which it doesn't report
		assert.Equal(t, "chat, tools, thinking, image_input", small.Capabilities().String())
		assert.Contains(t, model(t, "mistral-ocr-latest").Capabilities().String(), "ocr")
		assert.Equal(t, "transcription", model(t, "voxtral-mini-latest").Capabilities().String())
		assert.Equal(t, "embedding", model(t, mistralEmbeddingModel).Capabilities().String())
		assert.Equal(t, "embedding", model(t, mistralDimensionsModel).Capabilities().String())
	})

	t.Run("Reply", func(t *testing.T) {
		// The model replies, with a seed, which Mistral calls random_seed
		reply, usage, err := c.WithoutSession(ctx, model(t, mistralModel), []llm.Message{liveUser(t, "Reply with the single word: pong")}, nil, nil, llm.WithMaxTokens(16), llm.WithSeed(1))
		require.NoError(t, err)
		assert.Contains(t, strings.ToLower(reply.Text()), "pong")
		assert.Equal(t, schema.ResultStop, reply.(*schema.Message).Result)
		assert.NotZero(t, usage.InputTokens)
		assert.NotZero(t, usage.OutputTokens)
	})

	t.Run("Stream", func(t *testing.T) {
		// The reply is streamed in pieces, which make its text, and the
		// usage is in the last piece
		var text strings.Builder
		reply, usage, err := c.WithoutSession(ctx, model(t, mistralModel), []llm.Message{liveUser(t, "Say hello in five words")}, nil, func(kind llm.StreamType, piece string) {
			if kind == llm.StreamText {
				text.WriteString(piece)
			}
		}, llm.WithMaxTokens(32))
		require.NoError(t, err)
		assert.Equal(t, reply.Text(), text.String())
		assert.NotZero(t, usage.OutputTokens)
	})

	t.Run("Thinking", func(t *testing.T) {
		// With thinking, the reply has the model's thinking, and its text,
		// whether it's streamed or not. The thinking is returned, so its
		// tokens are the reply's, rather than reasoning tokens.
		messages := []llm.Message{liveUser(t, "What is 17 times 23? Reply with the number.")}
		reply, usage, err := c.WithoutSession(ctx, model(t, mistralModel), messages, nil, nil, llm.WithThinking(true))
		require.NoError(t, err)
		assert.NotEmpty(t, reply.(*schema.Message).Thinking())
		assert.Contains(t, reply.Text(), "391")
		assert.Zero(t, usage.ReasoningTokens)
		assert.NotZero(t, usage.OutputTokens)
		assert.Equal(t, uint64(usage.OutputTokens), reply.(*schema.Message).Tokens)

		var thinking, text strings.Builder
		reply, usage, err = c.WithoutSession(ctx, model(t, mistralModel), messages, nil, func(kind llm.StreamType, piece string) {
			switch kind {
			case llm.StreamThinking:
				thinking.WriteString(piece)
			case llm.StreamText:
				text.WriteString(piece)
			}
		}, llm.WithThinking(true))
		require.NoError(t, err)
		assert.NotEmpty(t, thinking.String())
		assert.Equal(t, thinking.String(), reply.(*schema.Message).Thinking())
		assert.Equal(t, text.String(), reply.Text())
		assert.Contains(t, reply.Text(), "391")
		assert.Zero(t, usage.ReasoningTokens)
		assert.NotZero(t, usage.OutputTokens)
		assert.Equal(t, uint64(usage.OutputTokens), reply.(*schema.Message).Tokens)

		// And without it, there's none
		reply, usage, err = c.WithoutSession(ctx, model(t, mistralModel), messages, nil, nil, llm.WithThinking(false))
		require.NoError(t, err)
		assert.Empty(t, reply.(*schema.Message).Thinking())
		assert.Zero(t, usage.ReasoningTokens)
	})

	t.Run("ToolCall", func(t *testing.T) {
		// The model calls the tool, with its input as JSON
		reply, _, err := c.WithoutSession(ctx, model(t, mistralModel), []llm.Message{liveUser(t, "What's the weather in Berlin?")}, []llm.Tool{weatherTool{}}, nil, llm.WithToolChoice("any"))
		require.NoError(t, err)
		calls := reply.ToolCalls()
		if assert.Len(t, calls, 1) {
			assert.Equal(t, "get_weather", calls[0].Name)
			var input struct {
				City string `json:"city"`
			}
			require.NoError(t, json.Unmarshal(calls[0].Input, &input))
			assert.Contains(t, input.City, "Berlin")
		}
	})

	t.Run("JSONSchema", func(t *testing.T) {
		// The reply is JSON, of the schema
		reply, _, err := c.WithoutSession(ctx, model(t, mistralModel), []llm.Message{liveUser(t, "Name a colour")}, nil, nil, llm.WithJSONSchema(json.RawMessage(`{"type":"object","properties":{"colour":{"type":"string"}},"required":["colour"]}`)))
		require.NoError(t, err)
		var result struct {
			Colour string `json:"colour"`
		}
		require.NoError(t, json.Unmarshal([]byte(reply.Text()), &result))
		assert.NotEmpty(t, result.Colour)
	})

	t.Run("User", func(t *testing.T) {
		// Mistral has no user, so it isn't sent
		_, _, err := c.WithoutSession(ctx, model(t, mistralModel), []llm.Message{liveUser(t, "Hi")}, nil, nil, llm.WithUser("someone"))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Embed", func(t *testing.T) {
		// Each message is a vector, of the model's dimensions, or as many as
		// are asked for, which Mistral calls output_dimension
		messages := []llm.Message{liveUser(t, "The cat sat on the mat"), liveUser(t, "A dog barked")}
		vectors, usage, err := c.Embed(ctx, model(t, mistralEmbeddingModel), messages)
		require.NoError(t, err)
		if assert.Len(t, vectors, 2) {
			assert.NotEmpty(t, vectors[0])
			assert.Len(t, vectors[1], len(vectors[0]))
		}
		assert.NotZero(t, usage.InputTokens)

		vectors, _, err = c.Embed(ctx, model(t, mistralDimensionsModel), messages[:1], llm.WithDimensions(256))
		require.NoError(t, err)
		if assert.Len(t, vectors, 1) {
			assert.Len(t, vectors[0], 256)
		}
	})
}

func Test_LiveMistral_Recognize(t *testing.T) {
	c := mistralClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
	defer cancel()
	model, err := c.GetModel(ctx, mistralOCRModel)
	require.NoError(t, err)
	assert.True(t, types.Value(model.Capabilities().OCR))
	read := func(name, contentType string) llm.Resource {
		data, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
		require.NoError(t, err)
		return &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: contentType, Data: data}}
	}

	t.Run("Document", func(t *testing.T) {
		// The text of each of the document's pages, in order, and the pages
		document, usage, err := c.Recognize(ctx, model, read("ocr.pdf", "application/pdf"))
		require.NoError(t, err)
		if assert.Len(t, document.Pages, 2) {
			assert.Contains(t, document.Pages[0].Text, "Hello, World")
			assert.Contains(t, document.Pages[1].Text, "Page two")
			assert.Empty(t, document.Pages[0].Regions)
		}
		assert.Equal(t, uint(2), usage.Pages)
	})

	t.Run("Pages", func(t *testing.T) {
		// Only the pages asked for
		document, usage, err := c.Recognize(ctx, model, read("ocr.pdf", "application/pdf"), llm.WithPages(1))
		require.NoError(t, err)
		if assert.Len(t, document.Pages, 1) {
			assert.Contains(t, document.Pages[0].Text, "Page two")
		}
		assert.Equal(t, uint(1), usage.Pages)
	})

	t.Run("Image", func(t *testing.T) {
		// An image is one page
		document, usage, err := c.Recognize(ctx, model, read("ocr.png", "image/png"))
		require.NoError(t, err)
		if assert.Len(t, document.Pages, 1) {
			assert.Contains(t, document.Pages[0].Text, "Hello, World")
		}
		assert.Equal(t, uint(1), usage.Pages)
	})

	t.Run("Tables", func(t *testing.T) {
		// A table, in the table format, in place of the link to it, which
		// is Markdown without one
		for format, want := range map[string]string{"": "Apple | 1.20", "markdown": "Apple | 1.20", "html": "<td>Apple</td>"} {
			var opts []llm.Opt
			if format != "" {
				opts = append(opts, llm.WithTableFormat(format))
			}
			document, _, err := c.Recognize(ctx, model, read("table.png", "image/png"), opts...)
			require.NoError(t, err, format)
			require.Len(t, document.Pages, 1)
			assert.Contains(t, document.Pages[0].Text, want, format)
			assert.NotContains(t, document.Pages[0].Text, "](tbl-", format)
		}
	})

	t.Run("Regions", func(t *testing.T) {
		// Each page's regions, with their boxes as fractions of the page
		document, _, err := c.Recognize(ctx, model, read("ocr.pdf", "application/pdf"), llm.WithRegions(true))
		require.NoError(t, err)
		require.Len(t, document.Pages, 2)
		t.Logf("%+v", document.Pages[1])
		if assert.NotEmpty(t, document.Pages[1].Regions) {
			region := document.Pages[1].Regions[0]
			assert.Contains(t, region.Text, "Page two")
			assert.NotEmpty(t, region.Type)
			for _, value := range region.Box {
				assert.GreaterOrEqual(t, value, 0.0)
				assert.LessOrEqual(t, value, 1.0)
			}
			assert.Less(t, region.Box[0], region.Box[2])
			assert.Less(t, region.Box[1], region.Box[3])
		}
	})
}

func Test_LiveMistral_Transcribe(t *testing.T) {
	c := mistralClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
	defer cancel()
	model, err := c.GetModel(ctx, mistralTranscribeModel)
	require.NoError(t, err)
	assert.True(t, types.Value(model.Capabilities().Transcription))
	assert.False(t, types.Value(model.Capabilities().Translation))
	read := func(name string) llm.Resource {
		data, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
		require.NoError(t, err)
		return &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: schema.ResourceType(data, name), Data: data}}
	}

	t.Run("Transcribe", func(t *testing.T) {
		// The text, in the language set, without segments, and the seconds
		// and tokens
		transcript, usage, err := c.Transcribe(ctx, model, read("speech-fr.wav"), nil, llm.WithLanguage("fr"))
		require.NoError(t, err)
		t.Logf("%+v, %+v", transcript, usage)
		assert.Contains(t, strings.ToLower(transcript.Text), "bonjour")
		assert.Equal(t, "fr", transcript.Language)
		assert.Empty(t, transcript.Segments)
		assert.Positive(t, usage.Seconds)
		assert.Positive(t, usage.InputTokens)
	})

	t.Run("Stream", func(t *testing.T) {
		// The pieces of the text, and the segments, as they're transcribed
		var pieces []string
		var segments []llm.TranscriptSegment
		transcript, _, err := c.Transcribe(ctx, model, read("speech-en.m4a"), func(text string, segment *llm.TranscriptSegment) {
			if segment != nil {
				segments = append(segments, *segment)
			} else {
				pieces = append(pieces, text)
			}
		}, llm.WithSegments(true))
		require.NoError(t, err)
		t.Logf("%d pieces, %d segments", len(pieces), len(segments))
		assert.Greater(t, len(pieces), 1)
		assert.Equal(t, transcript.Text, strings.TrimSpace(strings.Join(pieces, "")))
		assert.Equal(t, transcript.Segments, segments)
	})

	t.Run("Speakers", func(t *testing.T) {
		// The segments, with their speakers
		transcript, _, err := c.Transcribe(ctx, model, read("speech-en.m4a"), nil, llm.WithSpeakers(true))
		require.NoError(t, err)
		t.Logf("%+v", transcript)
		if assert.NotEmpty(t, transcript.Segments) {
			assert.NotEmpty(t, transcript.Segments[0].Speaker)
			assert.Greater(t, transcript.Segments[0].End, transcript.Segments[0].Start)
		}
	})
}
