package openai

import (
	"context"
	"errors"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECK

var _ llm.Embedder = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Embed returns a vector for each of the messages, which must be for a model
// of this client which embeds, in the same order, and the tokens in their
// texts, in one request to the Embeddings API, which OpenAI-compatible
// servers such as Mistral, Gemini and Ollama have, but DeepSeek hasn't. A
// message is embedded by its text, which can't be empty, as the API embeds
// only text. The request isn't timed out, but is cancelled with the context.
func (c *Client) Embed(ctx context.Context, model llm.Model, messages []llm.Message, opts ...llm.Opt) ([]llm.Vector, llm.Usage, error) {
	if m, ok := model.(*Model); !ok || m.provider != c {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if len(messages) == 0 {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing messages")
	} else if c.isDeepSeek() {
		return nil, llm.Usage{}, llm.ErrNotImplemented.With("deepseek has no embeddings")
	}

	// The texts of the messages, which can't be empty or have media
	request := embedRequest{Model: model.Name(), Input: make([]string, 0, len(messages)), EncodingFormat: "float"}
	for i, message := range messages {
		switch {
		case message == nil || message.Text() == "":
			return nil, llm.Usage{}, llm.ErrBadParameter.Withf("message %d has no text", i)
		case len(message.Resources()) > 0:
			return nil, llm.Usage{}, llm.ErrBadParameter.Withf("message %d has media, which the embeddings API can't embed", i)
		}
		request.Input = append(request.Input, message.Text())
	}
	if err := request.apply(c.server(), opts...); err != nil {
		return nil, llm.Usage{}, err
	}

	// Send the request, and receive a vector for each text
	req, err := client.NewJSONRequest(request)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	var response embedResponse
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("embeddings"), client.OptNoTimeout()); err != nil {
		return nil, llm.Usage{}, err
	} else if response.Error != nil {
		return nil, llm.Usage{}, errors.New(response.Error.Message)
	}

	// The vectors, in the order of their messages, by their indexes
	vectors := make([]llm.Vector, len(messages))
	for _, data := range response.Data {
		if data.Index < 0 || data.Index >= len(vectors) || vectors[data.Index] != nil {
			return nil, llm.Usage{}, llm.ErrInternalServerError.Withf("a vector with index %d for %d messages", data.Index, len(messages))
		}
		vectors[data.Index] = llm.Vector(data.Embedding)
	}
	for i, vector := range vectors {
		if vector == nil {
			return nil, llm.Usage{}, llm.ErrInternalServerError.Withf("no vector for message %d", i)
		}
	}
	return vectors, llm.Usage{InputTokens: response.Usage.PromptTokens}, nil
}
