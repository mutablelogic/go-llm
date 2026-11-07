package openai

import (
	"context"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// Test_ImageFormats checks an attachment which isn't an image in a format
// the API takes, such as an SVG, is a note, rather than the request
// failing, and an image which is, is sent
func Test_ImageFormats(t *testing.T) {
	svg := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/svg+xml", Data: []byte("<svg/>")}}
	png := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/png", Data: []byte("PNG")}}
	message := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "user", Content: []schema.ContentBlock{{Text: types.Ptr("What are these?")}, {Resource: svg}, {Resource: png}}}}
	chat, err := newChatMessages(context.Background(), message, false)
	require.NoError(t, err)
	require.Len(t, chat, 1)
	parts := chat[0].Content.([]chatPart)
	require.Len(t, parts, 3)
	assert.Equal(t, "text", parts[1].Type)
	assert.Contains(t, parts[1].Text, "image/svg+xml, which this model can't read")
	assert.Equal(t, "image_url", parts[2].Type)
}

// Test_AssistantImages checks what another model made, such as an image,
// in a reply, isn't sent, as the API doesn't take it, but its text is
func Test_AssistantImages(t *testing.T) {
	png := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/png", Data: []byte("PNG")}}
	message := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "assistant", Content: []schema.ContentBlock{{Text: types.Ptr("Here it is.")}, {Resource: png}}}}
	chat, err := newChatMessages(context.Background(), message, false)
	require.NoError(t, err)
	require.Len(t, chat, 1)
	assert.Equal(t, "Here it is.", chat[0].Content)
}
