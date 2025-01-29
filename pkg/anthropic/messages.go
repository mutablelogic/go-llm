package anthropic

import (
	"context"

	// Packages
	"github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

///////////////////////////////////////////////////////////////////////////////
// MESSAGES

type reqMessages struct {
	Model string `json:"model"`
	*opt
}

func (anthropic *Client) Messages(ctx context.Context, model llm.Model, context llm.Context, opts ...llm.Opt) error {
	// Apply options
	opt, err := apply(opts...)
	if err != nil {
		return err
	}

	// Request
	req, err := client.NewJSONRequest(reqMessages{
		Model: model.Name(),
		opt:   opt,
	})
	if err != nil {
		return err
	}

	// Response
	if err := anthropic.DoWithContext(ctx, req, nil, client.OptPath("messages")); err != nil {
		return err
	}

	// Return success
	return nil
}
