package registry

import (
	"context"
	"time"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

var _ llm.Speaker = (*Registry)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Speak returns the text, spoken, as audio, from the model's provider,
// which must be in the registry, and speak text. Returns ErrNotImplemented
// when the model is known not to speak, and the provider's error as
// providerErr does.
func (r *Registry) Speak(ctx context.Context, model llm.Model, text string, opts ...llm.Opt) (_ llm.Resource, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Speak",
		attribute.String("arg.model", r.modelName(model)),
		attribute.Int("arg.text", len(text)),
	)
	defer func() { endSpan(err) }()

	speaker, err := capability[llm.Speaker](r, model, "speak text")
	if err != nil {
		return nil, llm.Usage{}, err
	} else if err := can(model, model.Capabilities().SpeechOutput, "speak text"); err != nil {
		return nil, llm.Usage{}, err
	}
	start := time.Now()
	audio, usage, err := speaker.Speak(ctx, model, text, opts...)
	usage = timed(usage, start)
	if err != nil {
		return nil, llm.Usage{}, providerErr(err)
	}
	return audio, usage, nil
}

// ListVoices returns the voices the model speaks in, from the model's
// provider, which must be in the registry, and speak text, or none, when the
// provider can't say, a page at a time, as the options say, with the token
// of the next page. Returns ErrNotImplemented when the model is known not
// to speak, and the provider's error as providerErr does.
func (r *Registry) ListVoices(ctx context.Context, model llm.Model, opts ...llm.Opt) (_ []llm.Voice, _ string, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.ListVoices",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	speaker, err := capability[llm.Speaker](r, model, "speak text")
	if err != nil {
		return nil, "", err
	} else if err := can(model, model.Capabilities().SpeechOutput, "speak text"); err != nil {
		return nil, "", err
	}
	voices, next, err := speaker.ListVoices(ctx, model, opts...)
	if err != nil {
		return nil, "", providerErr(err)
	}
	return voices, next, nil
}
