package ollama

import (
	"context"
	"errors"
	"fmt"
	"slices"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECK

var _ llm.Embedder = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Embed returns a vector for each of the messages, which must be for an
// Ollama model which embeds, in the same order, and the tokens in their
// inputs, in one request. A message is embedded by its text, and its image
// and audio, at most one of each, for a model with vision or audio, such as
// embeddinggemma-2. A message must have text or media. The request isn't
// timed out, as a model can take a while to load, but is cancelled with the
// context.
func (c *Client) Embed(ctx context.Context, model llm.Model, messages []llm.Message, opts ...llm.Opt) ([]llm.Vector, llm.Usage, error) {
	if model == nil || model.Provider() == nil || model.Provider().Name() != llm.Ollama {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not an ollama model")
	} else if len(messages) == 0 {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing messages")
	} else if !hasCapability(model, "embedding") {
		return nil, llm.Usage{}, llm.ErrBadParameter.Withf("model %q doesn't embed", model.Name())
	}

	// An input for each message: its text, or its text and media
	request := embedRequest{Model: model.Name(), Input: make([]any, 0, len(messages))}
	for i, message := range messages {
		if message == nil {
			return nil, llm.Usage{}, llm.ErrBadParameter.Withf("message %d is missing", i)
		}
		// An error from reading a resource, such as one referred to by a URL
		// which can't be read, keeps its code, and any other is a bad
		// parameter
		input, err := newEmbedInput(ctx, model, message)
		if code := llm.Err(0); errors.As(err, &code) {
			return nil, llm.Usage{}, fmt.Errorf("message %d: %w", i, err)
		} else if err != nil {
			return nil, llm.Usage{}, llm.ErrBadParameter.Withf("message %d: %w", i, err)
		}
		request.Input = append(request.Input, input)
	}
	if err := request.apply(opts...); err != nil {
		return nil, llm.Usage{}, err
	}

	// Send the request, and receive a vector for each input
	req, err := client.NewJSONRequest(request)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	var response embedResponse
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("embed"), client.OptNoTimeout()); err != nil {
		return nil, llm.Usage{}, err
	} else if len(response.Embeddings) != len(messages) {
		return nil, llm.Usage{}, llm.ErrInternalServerError.Withf("%d vectors for %d messages", len(response.Embeddings), len(messages))
	}

	// Return the vectors and usage
	vectors := make([]llm.Vector, len(response.Embeddings))
	for i, embedding := range response.Embeddings {
		vectors[i] = llm.Vector(embedding)
	}
	return vectors, llm.Usage{InputTokens: uint(max(response.PromptEvalCount, 0))}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// newEmbedInput returns the input which embeds the message: its text, when
// it has no resources, or else its text, and its image and audio, which the
// model must have the modalities for. Other resources, or more than one
// image or audio, can't be embedded.
func newEmbedInput(ctx context.Context, model llm.Model, message llm.Message) (any, error) {
	resources := message.Resources()
	if len(resources) == 0 {
		if message.Text() == "" {
			return nil, errors.New("no text or media")
		}
		return message.Text(), nil
	}
	input := embedInput{Text: message.Text()}
	for _, resource := range resources {
		var data *[]byte
		switch {
		case isImage(resource.Type()) && hasCapability(model, "vision"):
			data = &input.Image
		case isAudio(resource.Type()) && hasCapability(model, "audio"):
			data = &input.Audio
		case isImage(resource.Type()), isAudio(resource.Type()):
			return nil, fmt.Errorf("model %q can't embed %q", model.Name(), resource.Type())
		default:
			return nil, fmt.Errorf("%q can't be embedded", resource.Type())
		}
		if *data != nil {
			return nil, fmt.Errorf("more than one %q", resource.Type())
		}
		value, err := llm.ReadAll(ctx, resource)
		if err != nil {
			return nil, err
		} else if len(value) == 0 {
			return nil, fmt.Errorf("%q is empty", resource.Type())
		}
		*data = value
	}
	return input, nil
}

// hasCapability returns true when the model has the capability, such as
// vision, or when Ollama didn't report the model's capabilities
func hasCapability(model llm.Model, capability string) bool {
	m, ok := model.(*Model)
	if !ok || len(m.data.Capabilities) == 0 {
		return true
	}
	return slices.Contains(m.data.Capabilities, capability)
}
