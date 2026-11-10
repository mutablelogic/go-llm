package registry

import (
	"context"
	"time"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

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
// provider can't say. Returns ErrNotImplemented when the model is known not
// to speak, and the provider's error as providerErr does.
func (r *Registry) ListVoices(ctx context.Context, model llm.Model) (_ []llm.Voice, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.ListVoices",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	speaker, err := capability[llm.Speaker](r, model, "speak text")
	if err != nil {
		return nil, err
	} else if err := can(model, model.Capabilities().SpeechOutput, "speak text"); err != nil {
		return nil, err
	}
	voices, err := speaker.ListVoices(ctx, model)
	if err != nil {
		return nil, providerErr(err)
	}
	return voices, nil
}

// ListModelVoices returns the voices the model with the name speaks in,
// from the offset, and at most the limit of them, with how many there are,
// as ListVoices returns them. Returns ErrNotFound when there's no model
// with the name.
func (r *Registry) ListModelVoices(ctx context.Context, name string, req schema.VoiceListRequest) (_ *schema.VoiceList, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.ListModelVoices",
		attribute.String("arg.name", name),
		attribute.String("arg.req", types.Stringify(req)),
	)
	defer func() { endSpan(err) }()

	model, err := r.GetModel(ctx, name)
	if err != nil {
		return nil, err
	}
	voices, err := r.ListVoices(ctx, model)
	if err != nil {
		return nil, err
	}
	count := uint(len(voices))
	offset := min(req.Offset, count)
	body := voices[offset:]
	if req.Limit != nil && *req.Limit < uint(len(body)) {
		body = body[:*req.Limit]
	}
	list := &schema.VoiceList{VoiceListRequest: req, Model: r.modelName(model), Count: count, Body: body}
	list.Offset = offset
	return list, nil
}
