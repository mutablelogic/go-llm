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

// Illustrate returns the images the request's model, which is found by its
// name, or the provider and name separated by ":", makes from its prompt,
// and its images, kept as resources of its session, or on their own, with
// the model and the usage. The request isn't timed out, as making images
// can take a while.
func (c *Client) Illustrate(ctx context.Context, req schema.IllustrateRequest) (*schema.IllustrateResponse, error) {
	if strings.TrimSpace(req.Model) == "" {
		return nil, llm.ErrBadParameter.With("missing model")
	} else if strings.TrimSpace(req.Prompt) == "" {
		return nil, llm.ErrBadParameter.With("missing prompt")
	}
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}
	var response schema.IllustrateResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("llm", "illustrate"), client.OptNoTimeout()); err != nil {
		return nil, err
	}
	return &response, nil
}
