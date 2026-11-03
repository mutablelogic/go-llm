package registry

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
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
// model's provider, which must be in the registry, and generate messages.
// Returns ErrNotImplemented, without asking the provider, when the model is
// known not to reply, or there are tools, and it's known not to call them,
// or it's asked to think, and it's known not to, or the messages have
// images, and it's known not to see them.
func (r *Registry) WithoutSession(ctx context.Context, model llm.Model, messages []llm.Message, tools []llm.Tool, stream llm.GeneratorStreamFn, opts ...llm.Opt) (_ llm.Message, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.WithoutSession",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	generator, err := capability[llm.Generator](r, model, "generate messages")
	if err != nil {
		return nil, llm.Usage{}, err
	} else if err := canGenerate(model, messages, tools, opts...); err != nil {
		return nil, llm.Usage{}, err
	}
	return generator.WithoutSession(ctx, model, messages, tools, stream, opts...)
}

// Embed returns a vector for each of the messages, from the model's
// provider, which must be in the registry, and embed messages. Returns
// ErrNotImplemented when the model is known not to embed, and the provider's
// error as providerErr does.
func (r *Registry) Embed(ctx context.Context, model llm.Model, messages []llm.Message, opts ...llm.Opt) (_ []llm.Vector, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Embed",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	embedder, err := capability[llm.Embedder](r, model, "embed messages")
	if err != nil {
		return nil, llm.Usage{}, err
	} else if err := can(model, model.Capabilities().Embedding, "embed messages"); err != nil {
		return nil, llm.Usage{}, err
	}
	vectors, usage, err := embedder.Embed(ctx, model, messages, opts...)
	if err != nil {
		return nil, llm.Usage{}, providerErr(err)
	}
	return vectors, usage, nil
}

// Transcribe returns the speech in the audio, from the model's provider,
// which must be in the registry, and transcribe audio. Returns
// ErrNotImplemented when the model is known not to transcribe.
func (r *Registry) Transcribe(ctx context.Context, model llm.Model, audio llm.Resource, opts ...llm.Opt) (_ *llm.Transcript, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Transcribe",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	transcriber, err := capability[llm.Transcriber](r, model, "transcribe audio")
	if err != nil {
		return nil, llm.Usage{}, err
	} else if err := can(model, model.Capabilities().Transcription, "transcribe audio"); err != nil {
		return nil, llm.Usage{}, err
	}
	return transcriber.Transcribe(ctx, model, audio, opts...)
}

// Translate returns the speech in the audio as English text, from the
// model's provider, which must be in the registry, and transcribe audio.
// Returns ErrNotImplemented when the model can't translate, or is known not
// to.
func (r *Registry) Translate(ctx context.Context, model llm.Model, audio llm.Resource, opts ...llm.Opt) (_ *llm.Transcript, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Translate",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	transcriber, err := capability[llm.Transcriber](r, model, "translate audio")
	if err != nil {
		return nil, llm.Usage{}, err
	} else if err := can(model, model.Capabilities().Translation, "translate audio"); err != nil {
		return nil, llm.Usage{}, err
	}
	return transcriber.Translate(ctx, model, audio, opts...)
}

// Recognize returns the text in the resource, such as an image or a PDF, as
// a message, from the model's provider, which must be in the registry, and
// recognize text. Returns ErrNotImplemented when the model is known not to
// recognize text.
func (r *Registry) Recognize(ctx context.Context, model llm.Model, resource llm.Resource, opts ...llm.Opt) (_ llm.Message, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Recognize",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	recognizer, err := capability[llm.Recognizer](r, model, "recognize text")
	if err != nil {
		return nil, llm.Usage{}, err
	} else if err := can(model, model.Capabilities().OCR, "recognize text"); err != nil {
		return nil, llm.Usage{}, err
	}
	return recognizer.Recognize(ctx, model, resource, opts...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// canGenerate returns ErrNotImplemented when the model is known not to
// reply, or there are tools, and it's known not to call them, or it's asked
// to think, with thinking on, or an effort, and it's known not to, or the
// messages have images, and it's known not to see them
func canGenerate(model llm.Model, messages []llm.Message, tools []llm.Tool, opts ...llm.Opt) error {
	capabilities := model.Capabilities()
	if err := can(model, capabilities.Completion, "generate messages"); err != nil {
		return err
	} else if len(tools) > 0 {
		if err := can(model, capabilities.Tools, "call tools"); err != nil {
			return err
		}
	}
	if thinks(opts...) {
		if err := can(model, capabilities.Thinking, "think"); err != nil {
			return err
		}
	}
	if hasImage(messages) {
		if err := can(model, capabilities.Vision, "see images"); err != nil {
			return err
		}
	}
	return nil
}

// providerErr returns the error of a model's provider as the registry's: an
// llm.Err, such as ErrBadParameter for an option the provider doesn't take,
// as it is; the provider's rejection of the request, which is its status
// 400, 413 or 422, such as for an option the model doesn't support, as
// ErrBadParameter, with its message; and any other, such as its status 401,
// 429 or 500, or a connection which failed, as ErrServiceUnavailable, so
// the provider's status, such as of its own authentication, isn't the
// registry's
func providerErr(err error) error {
	if _, ok := errors.AsType[llm.Err](err); ok {
		return err
	}
	if status, ok := errors.AsType[httpresponse.Err](err); ok {
		switch int(status) {
		case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
			return llm.ErrBadParameter.Withf("%w", err)
		}
	}
	return llm.ErrServiceUnavailable.Withf("%w", err)
}

// can returns ErrNotImplemented when the capability is known to be false,
// which the action says, such as "embed messages", or nil when it's true, or
// isn't known
func can(model llm.Model, capability *bool, action string) error {
	if capability != nil && !*capability {
		return llm.ErrNotImplemented.Withf("model %q can't %s", model.Name(), action)
	}
	return nil
}

// thinks returns true when the options ask the model to think, with
// thinking on, or a reasoning effort. Options which aren't valid are left
// to the provider.
func thinks(opts ...llm.Opt) bool {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return false
	}
	if thinking, err := llm.GetOpt[bool](o, llm.ThinkingKey); err == nil {
		return thinking
	} else if !errors.Is(err, llm.ErrNotFound) {
		return false
	}
	effort, err := llm.GetOpt[string](o, llm.ThinkingEffortKey)
	return err == nil && effort != "" && effort != "none"
}

// hasImage returns true when one of the messages has an image
func hasImage(messages []llm.Message) bool {
	for _, message := range messages {
		if message == nil {
			continue
		}
		for _, resource := range message.Resources() {
			if resource == nil {
				continue
			}
			if mediaType, _, err := mime.ParseMediaType(resource.Type()); err == nil && strings.HasPrefix(mediaType, "image/") {
				return true
			}
		}
	}
	return false
}
