package manager

import (
	"context"
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

	// Get all models for the candidate providers, then require exactly one named match.
	models, err := m.modelsForProviders(ctx, providers)
	if err != nil {
		return nil, err
	}
	matches := modelsByName(models, req.Name)
	return singleModel(matches, req.Name)
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
		group.Go(func() error {
			models, err := m.Registry.GetModels(ctx, provider.Name, provider.Include, provider.Exclude)
			if err != nil {
				return err
			}
			for _, candidate := range models {
				if candidate.Name == model {
					mu.Lock()
					filtered = append(filtered, provider)
					mu.Unlock()
					break
				}
			}
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
		group.Go(func() error {
			models, err := m.Registry.GetModels(ctx, provider.Name, provider.Include, provider.Exclude)
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

func modelsByName(models []schema.Model, name string) []schema.Model {
	result := make([]schema.Model, 0, len(models))
	for _, model := range models {
		if model.Name == name {
			result = append(result, model)
		}
	}
	return result
}

func singleModel(models []schema.Model, name string) (*schema.Model, error) {
	switch len(models) {
	case 0:
		return nil, schema.ErrNotFound.Withf("model %q not found", name)
	case 1:
		return &models[0], nil
	default:
		return nil, schema.ErrConflict.Withf("multiple models named %q found; specify a provider", name)
	}
}
