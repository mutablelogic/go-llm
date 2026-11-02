package registry

import (
	"context"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The registry generates, downloads, embeds, transcribes and recognizes
// with the providers of the models
var _ llm.Generator = (*Registry)(nil)
var _ llm.Downloader = (*Registry)(nil)
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

// DownloadModel downloads the model with the name, with its provider, which
// must download models, and returns it. As the model may not exist yet, the
// name must be its canonical name, such as ollama:llama3.2:latest, unless
// only one of the providers downloads models.
func (r *Registry) DownloadModel(ctx context.Context, name string, progress llm.DownloadProgressFn) (_ llm.Model, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.DownloadModel",
		attribute.String("arg.name", name),
	)
	defer func() { endSpan(err) }()

	if name == "" {
		return nil, llm.ErrBadParameter.With("model name is required")
	}

	// A canonical name is its provider's name, and its name
	if provider, model, ok := strings.Cut(name, separator); ok {
		if p := r.Provider(provider); p != nil {
			downloader, ok := p.Self().(llm.Downloader)
			if !ok {
				return nil, llm.ErrNotImplemented.Withf("provider %q can't download models", provider)
			}
			return downloader.DownloadModel(ctx, model, progress)
		}
	}

	// Otherwise, the one provider which downloads models
	var downloaders []llm.Downloader
	var names []string
	for _, e := range r.entries() {
		if downloader, ok := e.provider.Self().(llm.Downloader); ok {
			downloaders = append(downloaders, downloader)
			names = append(names, e.name)
		}
	}
	switch len(downloaders) {
	case 0:
		return nil, llm.ErrNotImplemented.With("no provider can download models")
	case 1:
		return downloaders[0].DownloadModel(ctx, name, progress)
	default:
		return nil, llm.ErrBadParameter.Withf("model %q needs its provider, as %s can download models, such as %s%s%s", name, strings.Join(names, " and "), names[0], separator, name)
	}
}

// DeleteModel deletes the model, with its provider, which must be in the
// registry, and download models
func (r *Registry) DeleteModel(ctx context.Context, model llm.Model) (err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.DeleteModel",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	downloader, err := capability[llm.Downloader](r, model, "delete models")
	if err != nil {
		return err
	}
	return downloader.DeleteModel(ctx, model)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// modelName returns the canonical name of the model, for its span, or an
// empty string when there's no model
func (r *Registry) modelName(model llm.Model) string {
	if model == nil {
		return ""
	}
	return r.Name(model)
}

// capability returns the provider of the model, as T, which is what it can
// do, such as llm.Generator. Returns ErrNotFound when the model's provider
// isn't in the registry, and ErrNotImplemented when it can't do it, which
// the action says, such as "generate messages".
func capability[T any](r *Registry, model llm.Model, action string) (T, error) {
	var result T
	if model == nil {
		return result, llm.ErrBadParameter.With("model is required")
	}
	provider := model.Provider()
	if provider == nil {
		return result, llm.ErrBadParameter.Withf("model %q has no provider", model.Name())
	}
	name, ok := r.nameOf(provider)
	if !ok {
		return result, llm.ErrNotFound.Withf("provider %q of model %q isn't in the registry", provider.Name(), model.Name())
	}
	result, ok = provider.Self().(T)
	if !ok {
		return result, llm.ErrNotImplemented.Withf("provider %q can't %s", name, action)
	}
	return result, nil
}
