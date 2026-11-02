package registry

import (
	"context"

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

// GetProvider returns the provider with the name, which is pinged, so it's
// known whether it can be reached, or ErrNotFound when there's none
func (r *Registry) GetProvider(ctx context.Context, name string) (_ *schema.Provider, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.GetProvider",
		attribute.String("arg.name", name),
	)
	defer func() { endSpan(err) }()

	provider := r.Provider(name)
	if provider == nil {
		return nil, llm.ErrNotFound.Withf("provider %q", name)
	}
	return describePing(ctx, entry{name: name, provider: provider}), nil
}

// CreateProvider creates a provider of the type, such as ollama, and adds it
// to the registry, with its name, or its type when it has none, once it's
// validated: its API key's credentials, such as ${OPENAI_KEY}, are expanded
// with the registry's credentials provider, its client is created, with its
// endpoint, which is the type's default when it's empty, and the API key,
// which is sent as a bearer token, and it's pinged. Eliza has no endpoint or
// API key, as it makes no requests. Returns ErrBadParameter when the type is
// unknown, ErrServiceUnavailable when it can't be reached, as it can't be
// used, and ErrConflict when a provider with its name is added already.
func (r *Registry) CreateProvider(ctx context.Context, req schema.CreateProviderRequest) (_ *schema.Provider, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.CreateProvider",
		attribute.String("arg.req", req.String()),
	)
	defer func() { endSpan(err) }()

	// Expand the API key's credentials
	apiKey, err := r.credentials.Expand(ctx, req.APIKey)
	if err != nil {
		return nil, err
	}

	// Create the provider's client
	var provider llm.Provider
	switch req.Provider {
	case llm.OpenAI:
		provider, err = openai.New(req.URL, apiKey)
	case llm.Ollama:
		var opts []client.ClientOpt
		if apiKey != "" {
			opts = append(opts, client.OptReqToken(client.Token{Scheme: client.Bearer, Value: apiKey}))
		}
		provider, err = ollama.New(req.URL, opts...)
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
	if err := provider.Ping(ctx); err != nil {
		return nil, llm.ErrServiceUnavailable.Withf("provider %q can't be reached: %w", name, err)
	}

	// Add it to the registry
	if err := r.Add(name, provider); err != nil {
		return nil, err
	}
	return describe(entry{name: name, provider: provider}), nil
}

// DeleteProvider removes the provider with the name from the registry, and
// returns it, or returns ErrNotFound when there's none. It isn't pinged.
func (r *Registry) DeleteProvider(ctx context.Context, name string) (_ *schema.Provider, err error) {
	_, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.DeleteProvider",
		attribute.String("arg.name", name),
	)
	defer func() { endSpan(err) }()

	provider, err := r.Remove(name)
	if err != nil {
		return nil, err
	}
	return describe(entry{name: name, provider: provider}), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// describePing returns the provider, as describe does, which is pinged, so
// it's known whether it can be reached
func describePing(ctx context.Context, e entry) *schema.Provider {
	result := describe(e)
	if err := e.provider.Ping(ctx); err != nil {
		result.Error = err.Error()
	}
	return result
}

// describe returns the provider, with its name in the registry, and what it
// can do
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
