package lmstudio

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

var _ llm.Generator = (*Client)(nil)
var _ llm.Embedder = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithoutSession returns the model's reply to the messages, which must be
// for a model of this client, and the tokens used, with LM Studio's
// OpenAI-compatible Chat Completions API, as the OpenAI provider does, with
// its thinking, and its effort, as the model reasons. A model which isn't loaded is loaded as it's asked, when LM Studio is set to
// load models as they're needed.
func (c *Client) WithoutSession(ctx context.Context, model llm.Model, messages []llm.Message, tools []llm.Tool, stream llm.GeneratorStreamFn, opts ...llm.Opt) (llm.Message, llm.Usage, error) {
	compatible, err := c.compatible(model)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	if opts, err = model.(*Model).reasoningOpts(opts); err != nil {
		return nil, llm.Usage{}, err
	}
	return c.openai.WithoutSession(ctx, compatible, messages, tools, stream, opts...)
}

// Embed returns a vector for each of the messages, which must be for an
// embedding model of this client, with LM Studio's OpenAI-compatible
// embeddings API, as the OpenAI provider does
func (c *Client) Embed(ctx context.Context, model llm.Model, messages []llm.Message, opts ...llm.Opt) ([]llm.Vector, llm.Usage, error) {
	compatible, err := c.compatible(model)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	return c.openai.Embed(ctx, compatible, messages, opts...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// compatible returns the model, which must be this client's, as a model of
// the client of the OpenAI-compatible API, by its name
func (c *Client) compatible(model llm.Model) (llm.Model, error) {
	if m, ok := model.(*Model); !ok || m.provider != c {
		return nil, llm.ErrBadParameter.With("not a model of this provider")
	}
	return c.openai.Model(model.Name()), nil
}
