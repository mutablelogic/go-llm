package httpclient

import (
	"context"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Embed returns a vector for each of the messages in the request, in the
// same order, from its model, with the model's canonical name, and the
// tokens used. A model is its name, or the provider and name separated by
// ":", such as "ollama:embeddinggemma-2:latest". The request isn't timed
// out, as a model can take a while to load, but is cancelled with the
// context.
func (c *Client) Embed(ctx context.Context, req schema.EmbedRequest) (*schema.EmbedResponse, error) {
	if strings.TrimSpace(req.Model) == "" {
		return nil, llm.ErrBadParameter.With("missing model")
	} else if len(req.Messages) == 0 {
		return nil, llm.ErrBadParameter.With("missing messages")
	}
	body, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}
	var response schema.EmbedResponse
	if err := c.DoWithContext(ctx, body, &response, client.OptPath("llm", "embed"), client.OptNoTimeout()); err != nil {
		return nil, err
	}
	return &response, nil
}
