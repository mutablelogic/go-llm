package manager

import (
	"context"
	"errors"
	"sort"
	"sync"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
	errgroup "golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (m *Manager) ListModels(ctx context.Context, req schema.ModelListRequest, user *auth.User) (_ *schema.ModelList, err error) {
	// Otel
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "ListModels",
		attribute.String("req", req.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Get candidate providers for user, or all candidates if no user is provided.
	providers, err := m.providersForUser(ctx, req.Provider, "", user)
	if err != nil {
		return nil, err
	}

	// Make the list of provider names for the response
	providerNames := make([]string, 0, len(providers))
	for _, provider := range providers {
		providerNames = append(providerNames, provider.Name)
	}

	// Get all models for the candidate providers, then page the result for the response.
	models, err := m.modelsForProviders(ctx, providers)
	if err != nil {
		return nil, err
	}

	// Scope to the offset and limit
	count := uint(len(models))
	start := min(req.Offset, uint64(count))
	end := uint64(count)
	if req.Limit != nil {
		end = min(start+types.Value(req.Limit), uint64(count))
	}

	// Return success
	return &schema.ModelList{
		ModelListRequest: req,
		Provider:         providerNames,
		Count:            count,
		Body:             models[start:end],
	}, nil
}

func (m *Manager) GetModel(ctx context.Context, req schema.GetModelRequest, user *auth.User) (_ *schema.Model, err error) {
	// Otel
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "GetModel",
		attribute.String("req", req.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Get candidate providers for user, or all candidates if no user is provided.
	providers, err := m.providersForUser(ctx, req.Provider, req.Name, user)
	if err != nil {
		return nil, err
	}

	// Get all models for the candidate providers, to require exactly one named match.
	models, err := m.modelsByName(ctx, providers, req.Name)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, schema.ErrNotFound.Withf("model %q not found", req.Name)
	} else if len(models) > 1 {
		return nil, schema.ErrConflict.Withf("multiple models named %q found; specify a provider", req.Name)
	}
	return types.Ptr(models[0]), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (m *Manager) providersForUser(ctx context.Context, provider, model string, user *auth.User) ([]schema.Provider, error) {
	providerReq := schema.ProviderListRequest{
		Name:    provider,
		Enabled: types.Ptr(true),
	}
	if user != nil && len(user.Groups) > 0 {
		providerReq.Groups = user.Groups
	}

	var result []schema.Provider
	for {
		providers, err := m.ListProviders(ctx, providerReq)
		if err != nil {
			return nil, err
		} else if len(providers.Body) == 0 {
			break
		} else {
			result = append(result, providers.Body...)
			providerReq.OffsetLimit.Offset += uint64(len(providers.Body))
		}
	}
	if model == "" {
		return result, nil
	}

	var (
		mu       sync.Mutex
		filtered []schema.Provider
	)
	group, ctx := errgroup.WithContext(ctx)
	for _, provider := range result {
		provider := provider
		group.Go(func() error {
			if _, err := m.Registry.GetModel(ctx, &provider, model); err != nil {
				if errors.Is(err, schema.ErrNotFound) {
					return nil
				}
				return err
			}

			mu.Lock()
			filtered = append(filtered, provider)
			mu.Unlock()
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	return filtered, nil
}

func (m *Manager) modelsForProviders(ctx context.Context, providers []schema.Provider) ([]schema.Model, error) {
	var mu sync.Mutex
	var result []schema.Model

	// Fetch models from all providers in parallel, and aggregate results
	group, ctx := errgroup.WithContext(ctx)
	for _, provider := range providers {
		provider := provider
		group.Go(func() error {
			models, err := m.Registry.GetModels(ctx, &provider)
			if err != nil {
				return err
			}

			mu.Lock()
			result = append(result, models...)
			mu.Unlock()
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}

	// Sort models by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	// Return all models
	return result, nil
}

func (m *Manager) modelsByName(ctx context.Context, providers []schema.Provider, name string) ([]schema.Model, error) {
	var mu sync.Mutex
	var result []schema.Model

	// Fetch models from all providers in parallel, and aggregate results
	group, ctx := errgroup.WithContext(ctx)
	for _, provider := range providers {
		provider := provider
		group.Go(func() error {
			model, err := m.Registry.GetModel(ctx, &provider, name)
			if err != nil {
				return err
			}

			mu.Lock()
			result = append(result, model)
			mu.Unlock()
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}

	// Return matched models
	return result, nil
}
