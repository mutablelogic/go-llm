package registry

import (
	"context"
	"fmt"
	"slices"
	"strings"
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
// which must be in the registry, and speak text, in the voice, by its ID, or
// its name, such as "Matilda", which is found in the model's voices. Returns ErrNotImplemented
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
	if opts, err = r.withVoiceID(ctx, speaker, model, opts); err != nil {
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
// provider can't say, a page at a time, as the options say, with how many
// there are, and the token of the next page. Returns ErrNotImplemented when the model is known not
// to speak, and the provider's error as providerErr does.
func (r *Registry) ListVoices(ctx context.Context, model llm.Model, opts ...llm.Opt) (_ *llm.VoiceList, err error) {
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
	voices, err := speaker.ListVoices(ctx, model, opts...)
	if err != nil {
		return nil, providerErr(err)
	}
	return voices, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// voicesTTL is how long a model's voices are kept, to find a voice by its
// name
const voicesTTL = time.Minute

// voicesKey is a model's, of a provider, which is the provider itself,
// rather than its name, so a provider which is replaced, with its name,
// such as with another account, has voices of its own
type voicesKey struct {
	provider llm.Provider
	model    string
}

// cachedVoices is a model's voices, and when they were listed
type cachedVoices struct {
	voices []llm.Voice
	at     time.Time
}

// withVoiceID returns the options, with the voice, by its ID, when it's
// given by its name: the ID of the model's voice with the ID, or else whose
// title is the name, or starts with it, before a description, such as
// "Matilda" for "Matilda - Knowledgable, Professional", ignoring case, or
// the name, as it is, when it's none of them, or the model's voices can't be
// listed, so its provider says whether it's one. Returns ErrBadParameter
// when the name is more than one of them.
func (r *Registry) withVoiceID(ctx context.Context, speaker llm.Speaker, model llm.Model, opts []llm.Opt) ([]llm.Opt, error) {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}
	name, err := llm.GetOpt[string](o, llm.VoiceKey)
	if name = strings.TrimSpace(name); err != nil || name == "" {
		return opts, nil
	}
	voices := r.modelVoices(ctx, speaker, model)
	var found []llm.Voice
	for _, voice := range voices {
		if voice.ID == name {
			return opts, nil
		}
		title := strings.TrimSpace(voice.Title)
		short, _, _ := strings.Cut(title, " - ")
		if strings.EqualFold(voice.ID, name) || strings.EqualFold(title, name) || strings.EqualFold(strings.TrimSpace(short), name) {
			found = append(found, voice)
		}
	}
	switch len(found) {
	case 0:
		return opts, nil
	case 1:
		return append(slices.Clone(opts), llm.WithOpt(llm.VoiceKey, found[0].ID)), nil
	default:
		titles := make([]string, 0, len(found))
		for _, voice := range found {
			titles = append(titles, fmt.Sprintf("%s (%s)", voice.Title, voice.ID))
		}
		return nil, llm.ErrBadParameter.Withf("voice %q is more than one of the model's: %s, so give its id", name, strings.Join(titles, ", "))
	}
}

// modelVoices returns the model's voices, which are kept for voicesTTL, or
// none, when they can't be listed
func (r *Registry) modelVoices(ctx context.Context, speaker llm.Speaker, model llm.Model) []llm.Voice {
	key := voicesKey{provider: model.Provider().Self(), model: model.Name()}
	if cached, ok := r.voices.Load(key); ok && time.Since(cached.(cachedVoices).at) < voicesTTL {
		return cached.(cachedVoices).voices
	}
	voices, err := speaker.ListVoices(ctx, model)
	if err != nil {
		return nil
	}
	r.voices.Store(key, cachedVoices{voices: voices.Body, at: time.Now()})
	return voices.Body
}
