package registry

import (
	"context"
	"slices"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
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
		if req.Provider == "" || e.provider.Name() == req.Provider {
			entries = append(entries, e)
		}
	}
	body := make([]*schema.Provider, len(entries))
	each(entries, func(i int, e entry) {
		body[i] = describePing(ctx, e)
	})
	return &schema.ProviderList{ProviderListRequest: req, Count: uint(len(body)), Body: body}, nil
}

// GetProvider returns the provider with the name, with whether it can be
// reached, when it was last pinged, within PingInterval, or ErrNotFound
// when there's none
func (r *Registry) GetProvider(ctx context.Context, name string) (_ *schema.Provider, err error) {
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
// validated: its API key's credentials, such as ${OPENAI_KEY}, are expanded
// with the registry's credentials provider, its client is created, with its
// endpoint, which is the type's default when it's empty, the registry's
// client options, and the API key,
// which is sent as a bearer token, and it's pinged. Eliza has no endpoint or
// API key, as it makes no requests. Returns ErrBadParameter when the type is
// unknown, ErrServiceUnavailable when it can't be reached, as it can't be
// used, and ErrConflict when a provider with its name is added already.
func (r *Registry) CreateProvider(ctx context.Context, req schema.CreateProviderRequest) (_ *schema.Provider, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.CreateProvider",
		attribute.String("arg.req", req.String()),
	)
	defer func() { endSpan(err) }()

	// The patterns of the names of the models which are included, and
	// excluded, must be valid
	opts := []ProviderOpt{WithInclude(req.Include...), WithExclude(req.Exclude...)}
	if _, err := newCached(nil, opts...); err != nil {
		return nil, err
	}

	// Expand the API key's credentials
	apiKey, err := r.credentials.Expand(ctx, req.APIKey)
	if err != nil {
		return nil, err
	}

	// Create the provider's client, with the registry's client options
	var provider llm.Provider
	clientOpts := slices.Clone(r.clientopts)
	switch req.Provider {
	case llm.OpenAI:
		provider, err = openai.New(req.URL, apiKey, clientOpts...)
	case llm.Ollama:
		if apiKey != "" {
			clientOpts = append(clientOpts, client.OptReqToken(client.Token{Scheme: client.Bearer, Value: apiKey}))
		}
		provider, err = ollama.New(req.URL, clientOpts...)
	case llm.Eliza:
		if req.URL != "" || apiKey != "" {
			return nil, llm.ErrBadParameter.With("eliza has no endpoint or API key")
		}
		provider, err = eliza.New()
	default:
		return nil, llm.ErrBadParameter.Withf("unknown provider type %q, which is one of %v", req.Provider, schema.ProviderTypes())
	}
	if err != nil {
		return nil, err
	}

	// Ping it
	name := req.Name
	if name == "" {
		name = provider.Name()
	}
	pingAt := time.Now()
	if err := provider.Ping(ctx); err != nil {
		return nil, llm.ErrServiceUnavailable.Withf("provider %q can't be reached: %w", name, err)
	}

	// Add it to the registry, with the result of its ping, so it isn't
	// pinged again until it's stale
	if err := r.Add(name, provider, opts...); err != nil {
		return nil, err
	}
	c := r.cached(name)
	if c == nil || c.provider != provider {
		return nil, llm.ErrConflict.Withf("provider %q was replaced as it was added", name)
	}
	c.setPing(pingAt, nil)
	return describe(entry{name: name, provider: provider, cache: c}), nil
}

// DeleteProvider removes the provider with the name from the registry, and
// returns it, or returns ErrNotFound when there's none. It isn't pinged.
func (r *Registry) DeleteProvider(ctx context.Context, name string) (_ *schema.Provider, err error) {
	_, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.DeleteProvider",
		attribute.String("arg.name", name),
	)
	defer func() { endSpan(err) }()

	c := r.cached(name)
	if _, err := r.Remove(name); err != nil {
		return nil, err
	}
	return describe(entry{name: name, provider: c.provider, cache: c}), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// describePing returns the provider, as describe does, with whether it can
// be reached, when it was last pinged, within PingInterval, or otherwise
// once it's pinged
func describePing(ctx context.Context, e entry) *schema.Provider {
	result := describe(e)
	if err := e.cache.ping(ctx, PingInterval); err != nil {
		result.Error = err.Error()
	}
	return result
}

// describe returns the provider, with its name in the registry, what it
// can do, and the patterns of the names of its models which are included,
// and excluded
func describe(e entry) *schema.Provider {
	self := e.provider.Self()
	_, generator := self.(llm.Generator)
	_, embedder := self.(llm.Embedder)
	_, downloader := self.(llm.Downloader)
	_, transcriber := self.(llm.Transcriber)
	_, recognizer := self.(llm.Recognizer)
	result := &schema.Provider{
		Name:     e.name,
		Provider: e.provider.Name(),
		URL:      schema.RedactURL(e.provider.URL()),
		Include:  e.cache.include,
		Exclude:  e.cache.exclude,
		ProviderCapabilities: schema.ProviderCapabilities{
			Generator:   generator,
			Embedder:    embedder,
			Downloader:  downloader,
			Transcriber: transcriber,
			Recognizer:  recognizer,
		},
	}
	return result
}
