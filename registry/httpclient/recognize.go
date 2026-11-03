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

// Recognize returns the text in the image or document of the request, as a
// message with a text block for each page, in order, from its model, with
// the model's canonical name, and the pages and tokens used. A model is its
// name, or the provider and name separated by ":", such as
// "mistral:mistral-ocr-latest". The request isn't timed out, as a document
// can take a while, but is cancelled with the context.
func (c *Client) Recognize(ctx context.Context, req schema.RecognizeRequest) (*schema.RecognizeResponse, error) {
	if strings.TrimSpace(req.Model) == "" {
		return nil, llm.ErrBadParameter.With("missing model")
	} else if _, err := req.Resource(); err != nil {
		return nil, err
	}
	body, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}
	var response schema.RecognizeResponse
	if err := c.DoWithContext(ctx, body, &response, client.OptPath("llm", "recognize"), client.OptNoTimeout()); err != nil {
		return nil, err
	}
	return &response, nil
}
