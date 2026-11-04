package httpclient_test

import (
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ClientAsk(t *testing.T) {
	ctx := t.Context()
	client := newClient(t)
	req := schema.AskRequest{
		Model:    types.Ptr(eliza.ModelName),
		Messages: []providerschema.MessageMeta{{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr("I feel sad")}}}},
	}

	t.Run("Ask", func(t *testing.T) {
		response, err := client.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.Equal(t, "eliza:eliza-1966-en", response.Model)
		assert.NotEmpty(t, response.Message.Text())
	})

	t.Run("Stream", func(t *testing.T) {
		// The reply's text is streamed as it's generated
		var streamed strings.Builder
		response, err := client.Ask(ctx, req, func(what llm.StreamType, text string) {
			if what == llm.StreamText {
				streamed.WriteString(text)
			}
		})
		require.NoError(t, err)
		assert.NotEmpty(t, streamed.String())
		assert.Equal(t, streamed.String(), response.Message.Text())
	})

	t.Run("Errors", func(t *testing.T) {
		// A request needs a model and messages, before it's sent, and a
		// model which exists, which is an error event when it's streamed
		_, err := client.Ask(ctx, schema.AskRequest{Messages: req.Messages}, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = client.Ask(ctx, schema.AskRequest{Model: types.Ptr(eliza.ModelName)}, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		missing := req
		missing.Model = types.Ptr("missing")
		_, err = client.Ask(ctx, missing, nil)
		assert.Error(t, err)
		_, err = client.Ask(ctx, missing, func(llm.StreamType, string) {})
		assert.ErrorContains(t, err, "missing")
	})
}
