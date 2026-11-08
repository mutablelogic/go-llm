package registry

import (
	"context"
	"errors"
	"slices"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	anthropic "github.com/mutablelogic/go-llm/provider/anthropic"
	elevenlabs "github.com/mutablelogic/go-llm/provider/elevenlabs"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	gemini "github.com/mutablelogic/go-llm/provider/gemini"
	gowhisper "github.com/mutablelogic/go-llm/provider/gowhisper"
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	openai "github.com/mutablelogic/go-llm/provider/openai"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListProviders returns the providers, in order of their names, of the
// type which the request selects, or every type, which are pinged at once,
// so it's known which can't be reached
func (r *Registry) ListProviders(ctx context.Context, req schema.ProviderListRequest) (_ *schema.ProviderList, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.ListProviders",
		attribute.String("arg.req", req.String()),
	)
	defer func() { endSpan(err) }()

	var entries []entry
	for _, e := range r.entries() {
		if req.Type == "" || e.provider.Name() == req.Type {
			entries = append(entries, e)
		}
	}
	body := make([]*schema.ProviderStatus, len(entries))
	each(entries, func(i int, e entry) {
		body[i] = describePing(ctx, e)
	})
	return &schema.ProviderList{ProviderListRequest: req, Count: uint(len(body)), Body: body}, nil
}

// GetProvider returns the provider with the name, with whether it could be
// reached, when it was last pinged, as describePing does, or ErrNotFound
// when there's none
func (r *Registry) GetProvider(ctx context.Context, name string) (_ *schema.ProviderStatus, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.GetProvider",
		attribute.String("arg.name", name),
	)
	defer func() { endSpan(err) }()

	c := r.cached(name)
	if c == nil {
		return nil, llm.ErrNotFound.Withf("provider %q", name)
	}
	return describePing(ctx, entry{name: name, provider: c.provider, cache: c}), nil
}

// CreateProvider creates a provider of the type, such as ollama, and adds it
// to the registry, with its name, or its type when it has none, once it's
// validated, as newProvider does, and pinged. The request is kept, as the
// provider's definition, with its API key's placeholder, rather than its
// credentials, and its serial is 1, whatever the request's. Returns
// ErrBadParameter when the type is unknown,
// ErrServiceUnavailable when it can't be reached, as it can't be used, and
// ErrConflict when a provider with its name is added already.
func (r *Registry) CreateProvider(ctx context.Context, req schema.Provider) (_ *schema.ProviderStatus, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.CreateProvider",
		attribute.String("arg.req", req.String()),
	)
	defer func() { endSpan(err) }()
	req.Serial = 0
	return r.createProvider(ctx, req, true)
}

// UpdateProvider changes the provider with the name: its fields which can be
// set are replaced with the meta, apart from its API key, which is kept when
// the meta has none, as it isn't returned. Its type and name aren't changed.
// The provider is created again, as CreateProvider does, and replaces it,
// whether it can be reached or not, which it says, with its serial
// incremented, and watchers are sent that it was updated. A provider whose
// fields are the same is left as it is. Returns ErrNotFound when there's no provider with the name,
// ErrBadParameter when the meta isn't valid, and ErrConflict when the
// provider was added without a definition, such as with Add.
func (r *Registry) UpdateProvider(ctx context.Context, name string, meta schema.ProviderMeta) (_ *schema.ProviderStatus, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.UpdateProvider",
		attribute.String("arg.name", name),
		attribute.String("arg.meta", meta.String()),
	)
	defer func() { endSpan(err) }()
	return r.updateProvider(ctx, name, meta)
}

// DeleteProvider removes the provider with the name from the registry, and
// returns it, or returns ErrNotFound when there's none. It isn't pinged.
func (r *Registry) DeleteProvider(ctx context.Context, name string) (_ *schema.ProviderStatus, err error) {
	_, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.DeleteProvider",
		attribute.String("arg.name", name),
	)
	defer func() { endSpan(err) }()

	c, err := r.remove(name)
	if err != nil {
		return nil, err
	}
	return describe(entry{name: name, provider: c.provider, cache: c}), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// createProvider creates the provider of the request, and adds it to the
// registry, as CreateProvider does, when it can be reached, or when it
// needn't be, so a provider which is defined elsewhere, such as in a store,
// is added while it can't be reached, with the error of its ping. Its serial
// is the request's, such as a store's, or 1 when it has none.
func (r *Registry) createProvider(ctx context.Context, req schema.Provider, reachable bool) (*schema.ProviderStatus, error) {
	// Create and ping it
	if req.Serial == 0 {
		req.Serial = 1
	}
	name, provider, opts, err := r.newProvider(ctx, req)
	if err != nil {
		return nil, err
	}
	pingAt := time.Now()
	pingErr := pingProvider(ctx, provider, PingTimeout)
	if pingErr != nil && reachable {
		return nil, llm.ErrServiceUnavailable.Withf("provider %q can't be reached: %w", name, pingErr)
	}

	// Add it to the registry, with the result of its ping, which is its
	// first, so it isn't pinged again until it's stale
	if err := r.Add(name, provider, opts...); err != nil {
		return nil, err
	}
	c := r.cached(name)
	if c == nil || c.provider != provider {
		return nil, llm.ErrConflict.Withf("provider %q was replaced as it was added", name)
	}
	c.setPing(pingAt, pingErr)
	return describePing(ctx, entry{name: name, provider: provider, cache: c}), nil
}

// definition returns the request the provider with the name was created
// with, with its name, and its API key's placeholder, rather than its
// credentials, or false when there's no provider with the name, or it was
// added without one, such as with Add
func (r *Registry) definition(name string) (schema.Provider, bool) {
	c := r.cached(name)
	if c == nil || c.definition == nil {
		return schema.Provider{}, false
	}
	result := *c.definition
	result.Include, result.Exclude = slices.Clone(result.Include), slices.Clone(result.Exclude)
	result.Serial = c.serial.Load()
	return result, true
}

// setSerial sets the serial of the provider with the name, such as when a
// store has its definition, which isn't a change to it, so watchers aren't
// sent it
func (r *Registry) setSerial(name string, serial uint64) {
	if c := r.cached(name); c != nil && c.definition != nil {
		c.serial.Store(serial)
	}
}

// updateProvider replaces the fields of the provider with the name with the
// meta, as UpdateProvider does, keeping its API key when the meta has none,
// with its serial incremented. A provider whose fields are the same is left
// as it is.
func (r *Registry) updateProvider(ctx context.Context, name string, meta schema.ProviderMeta) (*schema.ProviderStatus, error) {
	existing := r.cached(name)
	if existing == nil {
		return nil, llm.ErrNotFound.Withf("provider %q", name)
	} else if existing.definition == nil {
		return nil, llm.ErrConflict.Withf("provider %q was added without a definition", name)
	}

	// Its definition, with the meta, which is left as it is when it's the
	// same
	req := *existing.definition
	if meta.APIKey == "" {
		meta.APIKey = req.APIKey
	}

	// A URL which is the provider's, as the API returns it, without its
	// password, and as the provider normalizes it, is the URL it has, so an
	// update which sends it back, such as to change only its patterns,
	// keeps its password, rather than replacing it with the redacted one
	if meta.URL != "" && meta.URL == schema.RedactURL(existing.provider.URL()) {
		meta.URL = req.URL
	}
	req.ProviderMeta = meta
	if req.Equal(*existing.definition) {
		return describePing(ctx, entry{name: name, provider: existing.provider, cache: existing}), nil
	}
	req.Serial = existing.serial.Load() + 1
	return r.redefine(ctx, name, existing, req)
}

// setProvider makes the provider with the definition's name the definition,
// with its serial, such as a store's: it's created, when there's none,
// whether it can be reached or not, or it's replaced, when its definition is
// different, including its type, or it has the definition's serial, when
// it's the same. Returns ErrConflict when the provider with the name was
// added without a definition, such as with Add.
func (r *Registry) setProvider(ctx context.Context, def schema.Provider) error {
	existing := r.cached(def.Name)
	switch {
	case existing == nil:
		_, err := r.createProvider(ctx, def, false)
		return err
	case existing.definition == nil:
		return llm.ErrConflict.Withf("provider %q was added without a definition", def.Name)
	case existing.definition.Equal(def):
		existing.serial.Store(def.Serial)
		return nil
	default:
		_, err := r.redefine(ctx, def.Name, existing, def)
		return err
	}
}

// redefine creates the provider of the request again, pings it, and
// replaces the existing provider with the name, with the result of its
// ping, which is its first, so it isn't an update, and watchers are sent
// that it was updated. Returns ErrConflict when the provider with the name
// was changed since it was got.
func (r *Registry) redefine(ctx context.Context, name string, existing *cached, req schema.Provider) (*schema.ProviderStatus, error) {
	_, provider, opts, err := r.newProvider(ctx, req)
	if err != nil {
		return nil, err
	}
	c, err := newCached(provider, opts...)
	if err != nil {
		return nil, err
	}
	pingAt := time.Now()
	pingErr := pingProvider(ctx, provider, PingTimeout)
	if err := r.replace(name, existing, c); err != nil {
		return nil, err
	}
	c.setPing(pingAt, pingErr)
	return describePing(ctx, entry{name: name, provider: provider, cache: c}), nil
}

// newProvider returns the name of the provider of the request, which is its
// type when it has none, and the provider, once the request is validated:
// the patterns of the names of its models must be valid, its API key's
// credentials, such as ${OPENAI_KEY}, are expanded with the registry's
// credentials provider, and its client is created, with its endpoint, which
// is the type's default when it's empty, the registry's client options, and
// the API key, which is sent as a bearer token. Eliza has no endpoint or API
// key, as it makes no requests, gowhisper has no API key, and elevenlabs
// needs one, which is sent as its own header. It returns the options of the provider,
// with its patterns, and the request, as its definition. A credential which
// doesn't exist is ErrBadParameter.
func (r *Registry) newProvider(ctx context.Context, req schema.Provider) (string, llm.Provider, []ProviderOpt, error) {
	// The patterns of the names of the models which are included, and
	// excluded, must be valid
	opts := []ProviderOpt{WithInclude(req.Include...), WithExclude(req.Exclude...)}
	if _, err := newCached(nil, opts...); err != nil {
		return "", nil, nil, err
	}

	// Expand the API key's credentials, of which one which doesn't exist is
	// a request which isn't valid, rather than a provider which isn't found
	apiKey, err := r.credentials.Expand(ctx, req.APIKey)
	if errors.Is(err, llm.ErrNotFound) {
		return "", nil, nil, llm.ErrBadParameter.Withf("api key: %v", err)
	} else if err != nil {
		return "", nil, nil, err
	}

	// Create the provider's client, with the registry's client options
	var provider llm.Provider
	clientOpts := slices.Clone(r.clientopts)
	switch req.Type {
	case llm.OpenAI:
		provider, err = openai.New(req.URL, apiKey, clientOpts...)
	case llm.Ollama:
		if apiKey != "" {
			clientOpts = append(clientOpts, client.OptReqToken(client.Token{Scheme: client.Bearer, Value: apiKey}))
		}
		provider, err = ollama.New(req.URL, clientOpts...)
	case llm.Anthropic:
		if apiKey == "" {
			return "", nil, nil, llm.ErrBadParameter.With("anthropic needs an API key")
		}
		provider, err = anthropic.New(req.URL, apiKey, clientOpts...)
	case llm.Gemini:
		if apiKey == "" {
			return "", nil, nil, llm.ErrBadParameter.With("gemini needs an API key")
		}
		provider, err = gemini.New(req.URL, apiKey, clientOpts...)
	case llm.ElevenLabs:
		if apiKey == "" {
			return "", nil, nil, llm.ErrBadParameter.With("elevenlabs needs an API key")
		}
		provider, err = elevenlabs.New(req.URL, apiKey, clientOpts...)
	case llm.GoWhisper:
		if apiKey != "" {
			return "", nil, nil, llm.ErrBadParameter.With("gowhisper has no API key")
		}
		provider, err = gowhisper.New(req.URL, clientOpts...)
	case llm.Eliza:
		if req.URL != "" || apiKey != "" {
			return "", nil, nil, llm.ErrBadParameter.With("eliza has no endpoint or API key")
		}
		provider, err = eliza.New()
	default:
		return "", nil, nil, llm.ErrBadParameter.Withf("unknown provider type %q, which is one of %v", req.Type, schema.ProviderTypes())
	}
	if err != nil {
		return "", nil, nil, err
	}

	// Its name is its type, when it has none, which is its definition's
	if req.Name == "" {
		req.Name = provider.Name()
	}
	return req.Name, provider, append(opts, withDefinition(req)), nil
}

// describePing returns the provider, as describe does, with whether it
// could be reached, when it was last pinged, without waiting for it, or once
// it's pinged, when it never was. A result older than PingInterval is
// returned while it's pinged again, in the background.
func describePing(ctx context.Context, e entry) *schema.ProviderStatus {
	result := describe(e)
	if err := e.cache.status(ctx, PingInterval); err != nil {
		result.Error = err.Error()
	}
	return result
}

// describe returns the provider, with its name in the registry, what it
// can do, and the patterns of the names of its models which are included,
// and excluded
func describe(e entry) *schema.ProviderStatus {
	self := e.provider.Self()
	_, generator := self.(llm.Generator)
	_, embedder := self.(llm.Embedder)
	_, downloader := self.(llm.Downloader)
	_, transcriber := self.(llm.Transcriber)
	_, recognizer := self.(llm.Recognizer)
	return &schema.ProviderStatus{
		Name:        e.name,
		Type:        e.provider.Name(),
		URL:         schema.RedactURL(e.provider.URL()),
		Include:     e.cache.include,
		Exclude:     e.cache.exclude,
		Serial:      e.cache.serial.Load(),
		Generator:   generator,
		Embedder:    embedder,
		Downloader:  downloader,
		Transcriber: transcriber,
		Recognizer:  recognizer,
	}
}
