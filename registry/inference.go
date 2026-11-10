package registry

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"slices"
	"strings"
	"time"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
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
var _ llm.Speaker = (*Registry)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithoutSession returns the model's reply to the messages, from the
// model's provider, which must be in the registry, and generate messages,
// or the provider's error as providerErr does.
// Returns ErrNotImplemented, without asking the provider, when the model is
// known not to reply, or there are tools, and it's known not to call them,
// or the messages have images, and it's known not to see them. The options
// a model is known not to take, such as thinking, for a model which doesn't
// think, and the temperature, for one which doesn't take one, aren't sent,
// as a session keeps its options as its model changes.
func (r *Registry) WithoutSession(ctx context.Context, model llm.Model, messages []llm.Message, tools []llm.Tool, stream llm.GeneratorStreamFn, opts ...llm.Opt) (_ llm.Message, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.WithoutSession",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	generator, err := capability[llm.Generator](r, model, "generate messages")
	if err != nil {
		return nil, llm.Usage{}, err
	} else if err := canGenerate(model, messages, tools); err != nil {
		return nil, llm.Usage{}, err
	}
	opts = withoutSampling(model, withoutThinking(model, opts...)...)

	// What the system says of the messages, such as where one was sent, is
	// sent as text, in a system reminder, as providers have no annotations
	messages = providerschema.AnnotationsAsText(messages)
	start := time.Now()
	reply, usage, err := generator.WithoutSession(ctx, model, messages, tools, stream, opts...)
	usage = timed(usage, start)
	if err != nil {
		return nil, llm.Usage{}, providerErr(err)
	}
	return reply, usage, nil
}

// Embed returns a vector for each of the messages, from the model's
// provider, which must be in the registry, and embed messages, whose
// attachments which refer to resources on their own, by their URIs, are
// read. Returns
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
	if messages, err = r.withContent(ctx, uuid.Nil(), messages, false, nil); err != nil {
		return nil, llm.Usage{}, err
	}
	start := time.Now()
	vectors, usage, err := embedder.Embed(ctx, model, messages, opts...)
	usage = timed(usage, start)
	if err != nil {
		return nil, llm.Usage{}, providerErr(err)
	}
	return vectors, usage, nil
}

// Transcribe returns the speech in the audio, from the model's provider,
// which must be in the registry, and transcribe audio, and streams its
// pieces to the function, when it isn't nil. Returns
// ErrNotImplemented when the model is known not to transcribe, and the
// provider's error as providerErr does.
func (r *Registry) Transcribe(ctx context.Context, model llm.Model, audio llm.Resource, stream llm.TranscriberStreamFn, opts ...llm.Opt) (_ llm.Transcript, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Transcribe",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	transcriber, err := capability[llm.Transcriber](r, model, "transcribe audio")
	if err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	} else if err := can(model, model.Capabilities().Transcription, "transcribe audio"); err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}
	if audio, err = r.streamed(ctx, audio); err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}
	start := time.Now()
	transcript, usage, err := transcriber.Transcribe(ctx, model, audio, stream, opts...)
	usage = timed(usage, start)
	if err != nil {
		return llm.Transcript{}, llm.Usage{}, providerErr(err)
	}
	return transcript, usage, nil
}

// Translate returns the speech in the audio as English text, from the
// model's provider, which must be in the registry, and transcribe audio.
// Returns ErrNotImplemented when the model can't translate, or is known not
// to, and the provider's error as providerErr does.
func (r *Registry) Translate(ctx context.Context, model llm.Model, audio llm.Resource, stream llm.TranscriberStreamFn, opts ...llm.Opt) (_ llm.Transcript, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Translate",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	transcriber, err := capability[llm.Transcriber](r, model, "translate audio")
	if err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	} else if err := can(model, model.Capabilities().Translation, "translate audio"); err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}
	if audio, err = r.streamed(ctx, audio); err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}
	start := time.Now()
	transcript, usage, err := transcriber.Translate(ctx, model, audio, stream, opts...)
	usage = timed(usage, start)
	if err != nil {
		return llm.Transcript{}, llm.Usage{}, providerErr(err)
	}
	return transcript, usage, nil
}

// Illustrate returns the images the model makes from the prompt, or by
// changing, or combining, the images, which can refer to resources, by
// their URIs, as the prompt says, from the model's provider, which must be
// in the registry, and make images. Returns ErrNotImplemented when the
// model is known not to make images, ErrBadParameter when one of the
// images isn't an image, or they're larger than MaxAttachmentsSize,
// together, and the provider's error as providerErr does.
func (r *Registry) Illustrate(ctx context.Context, model llm.Model, prompt string, images []llm.Resource, opts ...llm.Opt) (_ []llm.Resource, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Illustrate",
		attribute.String("arg.model", r.modelName(model)),
		attribute.Int("arg.prompt", len(prompt)),
		attribute.Int("arg.images", len(images)),
	)
	defer func() { endSpan(err) }()

	illustrator, err := capability[llm.Illustrator](r, model, "make images")
	if err != nil {
		return nil, llm.Usage{}, err
	} else if err := can(model, model.Capabilities().ImageOutput, "make images"); err != nil {
		return nil, llm.Usage{}, err
	}
	read, err := r.withImages(ctx, images)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	start := time.Now()
	result, usage, err := illustrator.Illustrate(ctx, model, prompt, read, opts...)
	usage = timed(usage, start)
	if err != nil {
		return nil, llm.Usage{}, providerErr(err)
	}
	return result, usage, nil
}

// Recognize returns the text in the resource, such as an image or a PDF, as
// a document, from the model's provider, which must be in the registry, and
// recognize text. Returns ErrNotImplemented when the model is known not to
// recognize text, and the provider's error as providerErr does.
func (r *Registry) Recognize(ctx context.Context, model llm.Model, resource llm.Resource, opts ...llm.Opt) (_ llm.Document, _ llm.Usage, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Recognize",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	recognizer, err := capability[llm.Recognizer](r, model, "recognize text")
	if err != nil {
		return llm.Document{}, llm.Usage{}, err
	} else if err := can(model, model.Capabilities().OCR, "recognize text"); err != nil {
		return llm.Document{}, llm.Usage{}, err
	}
	if resource, err = r.withResource(ctx, resource); err != nil {
		return llm.Document{}, llm.Usage{}, err
	}
	start := time.Now()
	document, usage, err := recognizer.Recognize(ctx, model, resource, opts...)
	usage = timed(usage, start)
	if err != nil {
		return llm.Document{}, llm.Usage{}, providerErr(err)
	}
	return document, usage, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// canGenerate returns ErrNotImplemented when the model is known not to
// reply, or there are tools, and it's known not to call them, or the
// messages have images, and it's known not to see them
func canGenerate(model llm.Model, messages []llm.Message, tools []llm.Tool) error {
	capabilities := model.Capabilities()
	if err := can(model, capabilities.Chat, "generate messages"); err != nil {
		return err
	} else if len(tools) > 0 {
		if err := can(model, capabilities.Tools, "call tools"); err != nil {
			return err
		}
	}
	if hasImage(messages) {
		if err := can(model, capabilities.ImageInput, "see images"); err != nil {
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
	if status, ok := llm.HTTPStatus(err); ok {
		switch status {
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

// withoutThinking returns the options, without thinking, and its effort,
// when the model is known not to think, such as GPT-4.1, rather than fail,
// as a session keeps its options as its model changes, so they're used
// again when it's changed back, or the options, as they are, when they
// can't be applied, so the provider says why
func withoutThinking(model llm.Model, opts ...llm.Opt) []llm.Opt {
	if thinking := model.Capabilities().Thinking; thinking == nil || *thinking {
		return opts
	}
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return opts
	}
	keys := []string{llm.ThinkingKey, llm.ThinkingEffortKey}
	if !slices.ContainsFunc(keys, o.Has) {
		return opts
	}
	for _, key := range keys {
		o.Delete(key)
	}
	return []llm.Opt{o.Opt()}
}

// withoutSampling returns the options, without the sampling options, which
// are the temperature, top_p and top_k, when the model is known not to take
// a temperature, such as Claude's models, rather than fail, as a session
// keeps its options as its model changes, or the options, as they are, when
// they can't be applied, so the provider says why
func withoutSampling(model llm.Model, opts ...llm.Opt) []llm.Opt {
	if temperature := model.Capabilities().Temperature; temperature == nil || *temperature {
		return opts
	}
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return opts
	}
	sampling := []string{llm.TemperatureKey, llm.TopPKey, llm.TopKKey}
	if !slices.ContainsFunc(sampling, o.Has) {
		return opts
	}
	for _, key := range sampling {
		o.Delete(key)
	}
	return []llm.Opt{o.Opt()}
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

// timed returns the usage, with how long the model took, from the start,
// unless its provider says
func timed(usage llm.Usage, start time.Time) llm.Usage {
	if usage.Duration == 0 {
		usage.Duration = time.Since(start)
	}
	return usage
}
