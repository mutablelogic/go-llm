package openai

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestRecognizeOther(t *testing.T) {
	// Only Mistral recognizes text, so there's no request
	c, err := New(DeepSeekEndpoint, "sk-test")
	require.NoError(t, err)
	model := &Model{provider: c, data: apiModel{ID: "deepseek-chat"}}
	image := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/png", Data: []byte("png")}}
	_, _, err = c.Recognize(context.Background(), model, image)
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
}

func TestOCRRequestApply(t *testing.T) {
	t.Run("Options", func(t *testing.T) {
		// The pages, in order, from a string or numbers, and the format of
		// tables
		var r ocrRequest
		require.NoError(t, r.apply(llm.WithOpt(llm.PagesKey, json.RawMessage(`"2,0-1"`)), llm.WithTableFormat("html")))
		assert.Equal(t, []uint{0, 1, 2}, r.Pages)
		assert.Equal(t, "html", r.TableFormat)
	})

	t.Run("Errors", func(t *testing.T) {
		// Pages, a format of tables, and options which aren't for
		// recognition, aren't valid
		for _, opt := range []llm.Opt{
			llm.WithOpt(llm.PagesKey, "a"),
			llm.WithOpt(llm.TableFormatKey, "csv"),
			llm.WithTemperature(0.5),
		} {
			var r ocrRequest
			assert.ErrorIs(t, r.apply(opt), llm.ErrBadParameter)
		}
	})
}
