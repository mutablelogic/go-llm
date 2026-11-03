package registry

import (
	"context"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The registry generates, embeds, transcribes and recognizes with the
// providers of the models
var _ llm.Generator = (*Registry)(nil)
var _ llm.Embedder = (*Registry)(nil)
var _ llm.Transcriber = (*Registry)(nil)
var _ llm.Recognizer = (*Registry)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithoutSession returns the model's reply to the messages, from the
// model's provider, which must be in the registry, and generate messages
func (r *Registry) WithoutSession(ctx context.Context, model llm.Model, messages []llm.Message, tools []llm.Tool, stream llm.GeneratorStreamFn, opts ...llm.Opt) (_ llm.Message, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.WithoutSession",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	generator, err := capability[llm.Generator](r, model, "generate messages")
	if err != nil {
		return nil, llm.Usage{}, err
	}
	return generator.WithoutSession(ctx, model, messages, tools, stream, opts...)
}

// Embed returns a vector for each of the messages, from the model's
// provider, which must be in the registry, and embed messages
func (r *Registry) Embed(ctx context.Context, model llm.Model, messages []llm.Message, opts ...llm.Opt) (_ []llm.Vector, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Embed",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	embedder, err := capability[llm.Embedder](r, model, "embed messages")
	if err != nil {
		return nil, llm.Usage{}, err
	}
	return embedder.Embed(ctx, model, messages, opts...)
}

// Transcribe returns the speech in the audio, from the model's provider,
// which must be in the registry, and transcribe audio
func (r *Registry) Transcribe(ctx context.Context, model llm.Model, audio llm.Resource, opts ...llm.Opt) (_ *llm.Transcript, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Transcribe",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	transcriber, err := capability[llm.Transcriber](r, model, "transcribe audio")
	if err != nil {
		return nil, llm.Usage{}, err
	}
	return transcriber.Transcribe(ctx, model, audio, opts...)
}

// Translate returns the speech in the audio as English text, from the
// model's provider, which must be in the registry, and transcribe audio.
// Returns ErrNotImplemented when the model can't translate.
func (r *Registry) Translate(ctx context.Context, model llm.Model, audio llm.Resource, opts ...llm.Opt) (_ *llm.Transcript, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Translate",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	transcriber, err := capability[llm.Transcriber](r, model, "translate audio")
	if err != nil {
		return nil, llm.Usage{}, err
	}
	return transcriber.Translate(ctx, model, audio, opts...)
}

// Recognize returns the text in the resource, such as an image or a PDF, as
// a message, from the model's provider, which must be in the registry, and
// recognize text
func (r *Registry) Recognize(ctx context.Context, model llm.Model, resource llm.Resource, opts ...llm.Opt) (_ llm.Message, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Recognize",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	recognizer, err := capability[llm.Recognizer](r, model, "recognize text")
	if err != nil {
		return nil, llm.Usage{}, err
	}
	return recognizer.Recognize(ctx, model, resource, opts...)
}
