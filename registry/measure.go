package registry

import (
	"context"
	"errors"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The registry measures text with the providers of the models
var _ llm.Measurer = (*Registry)(nil)

// bytesPerToken is how many bytes of text a token is, as an estimate, for
// models whose providers don't measure text
const bytesPerToken = 4

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Measure returns how many tokens the text is, for the model, from the
// model's provider, which must be in the registry, when it measures text,
// or otherwise an estimate, of a token for every bytesPerToken bytes, or
// part of one. Returns the provider's error as providerErr does.
func (r *Registry) Measure(ctx context.Context, model llm.Model, text string) (_ uint, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Measure",
		attribute.String("arg.model", r.modelName(model)),
		attribute.Int("arg.bytes", len(text)),
	)
	defer func() { endSpan(err) }()

	measurer, err := capability[llm.Measurer](r, model, "measure text")
	if errors.Is(err, llm.ErrNotImplemented) {
		return estimateTokens(text), nil
	} else if err != nil {
		return 0, err
	}
	tokens, err := measurer.Measure(ctx, model, text)
	if err != nil {
		return 0, providerErr(err)
	}
	return tokens, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// estimateTokens returns an estimate of how many tokens the text is, of a
// token for every bytesPerToken bytes, or part of one
func estimateTokens(text string) uint {
	return uint(len(text)+bytesPerToken-1) / bytesPerToken
}
