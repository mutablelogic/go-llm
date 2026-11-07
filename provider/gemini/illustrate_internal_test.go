package gemini

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Illustrate(t *testing.T) {
	ctx := context.Background()
	api, c, model := chatClient(t)

	// An image, from the prompt, and the images it changes, with the aspect
	// ratio, and size, apart from the drafts it makes as it thinks
	api.made = `{"candidates":[{"content":{"role":"model","parts":[
		{"inlineData":{"mimeType":"image/png","data":"RFJBRlQ="},"thought":true},
		{"text":"Here's your banana."},
		{"inlineData":{"mimeType":"image/jpeg","data":"SlBFRw=="}}
	]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":1290}}`
	before := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/png", Data: []byte("BEFORE")}}
	images, usage, err := c.Illustrate(ctx, model, "Put it at sunset", []llm.Resource{before}, llm.WithOpt(llm.AspectRatioKey, "16:9"), llm.WithOpt(llm.ImageSizeKey, "2k"))
	require.NoError(t, err)
	require.Len(t, images, 1)
	assert.Equal(t, "image/jpeg", images[0].Type())
	data, err := llm.ReadAll(ctx, images[0])
	require.NoError(t, err)
	assert.Equal(t, "JPEG", string(data))
	assert.Equal(t, uint(20), usage.InputTokens)
	assert.Equal(t, uint(1290), usage.OutputTokens)

	// The request has the images, then the prompt, and asks for images
	parts := api.asked.Contents[0].Parts
	require.Len(t, parts, 2)
	assert.Equal(t, []byte("BEFORE"), parts[0].InlineData.Data)
	assert.Equal(t, "Put it at sunset", parts[1].Text)
	config := api.asked.GenerationConfig
	assert.Equal(t, []string{"TEXT", "IMAGE"}, config.ResponseModalities)
	assert.Equal(t, &imageConfig{AspectRatio: "16:9", ImageSize: "2K"}, config.ImageConfig)

	// No image is an error, with what the model said, as is a blocked
	// prompt, no prompt, and an option which isn't for images
	api.made = `{"candidates":[{"content":{"parts":[{"text":"I can't make that."}]},"finishReason":"STOP"}]}`
	_, _, err = c.Illustrate(ctx, model, "Something", nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	assert.ErrorContains(t, err, "I can't make that.")
	api.made = `{"promptFeedback":{"blockReason":"SAFETY"}}`
	_, _, err = c.Illustrate(ctx, model, "Something", nil)
	assert.ErrorContains(t, err, "blocked: SAFETY")
	_, _, err = c.Illustrate(ctx, model, " ", nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, _, err = c.Illustrate(ctx, model, "Something", nil, llm.WithTemperature(0.5))
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// Imagen, which predicts images, rather than generating content, isn't
	// asked
	imagen := &Model{provider: c, data: apiModel{Name: "models/imagen-4.0-generate-001", SupportedGenerationMethods: []string{"predict"}}}
	_, _, err = c.Illustrate(ctx, imagen, "Something", nil)
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
}
