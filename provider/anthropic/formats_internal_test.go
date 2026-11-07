package anthropic

import (
	"context"
	"net/url"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// Test_ImageFormats checks an image in a format the API doesn't take, such
// as an SVG, is a note, with its URI, rather than the request failing, and
// one it does is sent
func Test_ImageFormats(t *testing.T) {
	uri, err := url.Parse("resource://" + "a1b2")
	require.NoError(t, err)
	svg := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/svg+xml", Data: []byte("<svg/>"), URL: uri}}
	png := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/png", Data: []byte("PNG")}}
	message := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "user", Content: []schema.ContentBlock{{Resource: svg}, {Resource: png}, {Text: types.Ptr("What are these?")}}}}
	messages, _, err := newMessages(context.Background(), []llm.Message{message})
	require.NoError(t, err)
	require.Len(t, messages, 1)
	blocks := messages[0].Content
	require.Len(t, blocks, 3)
	assert.Equal(t, "text", blocks[0].Type)
	assert.Contains(t, blocks[0].Text, "resource://a1b2 is image/svg+xml, which this model can't read")
	assert.Equal(t, "image", blocks[1].Type)
	assert.Equal(t, "What are these?", blocks[2].Text)
}
