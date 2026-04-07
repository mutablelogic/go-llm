package manager

import (
	"context"
	"errors"
	"slices"
	"sort"
	"sync"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
	errgroup "golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type downloaderCandidate struct {
	provider   schema.Provider
	model      schema.Model
	clientName string
	downloader llm.Downloader
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (m *Manager) ListModels(ctx context.Context, req schema.ModelListRequest, user *auth.User) (_ *schema.ModelList, err error) {
	// Otel
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "ListModels",
		attribute.String("req", types.Stringify(req)),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Get candidate providers for user, or all candidates if no user is provided.
	providers, err := m.providersForUser(ctx, req.Provider, user)
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
		attribute.String("req", types.Stringify(req)),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Get candidate providers for user, or all candidates if no user is provided.
	providers, err := m.providersForUser(ctx, req.Provider, user)
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

func (m *Manager) DownloadModel(ctx context.Context, req schema.DownloadModelRequest, user *auth.User, opts ...opt.Opt) (result *schema.Model, err error) {
	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "DownloadModel",
		attribute.String("req", types.Stringify(req)),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Get downloader-capable providers for user, or all candidates if no user is provided.
	downloaders, err := m.downloaderCandidates(ctx, req.Provider, user)
	if err != nil {
		return nil, err
	}

	switch len(downloaders) {
	case 1:
		model, err := downloaders[0].downloader.DownloadModel(ctx, req.Name, opts...)
		if err != nil {
			return nil, err
		}
		if model != nil {
			model.OwnedBy = downloaders[0].provider.Name
		}
		return model, nil
	default:
		return nil, schema.ErrConflict.With("multiple providers support model downloads; specify a provider")
	}
}

func (m *Manager) DeleteModel(ctx context.Context, req schema.DeleteModelRequest, user *auth.User) (result *schema.Model, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "DeleteModel",
		attribute.String("req", types.Stringify(req)),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Get candidate providers for user, or all candidates if no user is provided.
	downloaders, err := m.downloaderCandidates(ctx, req.Provider, user)
	if err != nil {
		return nil, err
	}

	// Resolve the named model across candidate providers only.
	models, err := m.modelsByName(ctx, providersFromDownloaderCandidates(downloaders), req.Name)
	if err != nil {
		return nil, err
	}

	// Collect candidates that can delete the model
	deletions := deleteCandidatesForModels(models, downloaders)
	switch len(deletions) {
	case 0:
		return nil, schema.ErrNotFound.Withf("model %q not found", req.Name)
	case 1:
		model := deletions[0].model
		runtimeModel := model
		if deletions[0].clientName != "" {
			runtimeModel.OwnedBy = deletions[0].clientName
		}
		if err := deletions[0].downloader.DeleteModel(ctx, runtimeModel); err != nil {
			return nil, err
		}
		return types.Ptr(model), nil
	default:
		return nil, schema.ErrConflict.With("multiple providers own this model; specify a provider")
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (m *Manager) providersForUser(ctx context.Context, provider string, user *auth.User) ([]schema.Provider, error) {
	providerReq := schema.ProviderListRequest{
		Name:    provider,
		Enabled: types.Ptr(true),
	}

	var result []schema.Provider
	for {
		providers, err := m.listProviders(ctx, providerReq, user)
		if err != nil {
			return nil, err
		} else if len(providers.Body) == 0 {
			break
		} else {
			for _, p := range providers.Body {
				result = append(result, *p)
			}
			providerReq.OffsetLimit.Offset += uint64(len(providers.Body))
		}
	}

	return result, nil
}

func (m *Manager) downloaderCandidates(ctx context.Context, provider string, user *auth.User) ([]downloaderCandidate, error) {
	providers, err := m.providersForUser(ctx, provider, user)
	if err != nil {
		return nil, err
	}
	candidates := downloaderCandidatesForProviders(providers, m.Registry.Get)
	if len(candidates) == 0 {
		if provider != "" {
			return nil, schema.ErrNotFound.Withf("provider %q not found or does not support model operations", provider)
		}
		return nil, schema.ErrNotFound.With("no provider found that supports model operations")
	}
	return candidates, nil
}

func filterProvidersForUser(providers []schema.Provider, user *auth.User) []schema.Provider {
	if user == nil {
		return providers
	}

	filtered := make([]schema.Provider, 0, len(providers))
	for _, provider := range providers {
		if providerAccessibleToUser(provider, user) {
			filtered = append(filtered, provider)
		}
	}
	return filtered
}

func providerAccessibleToUser(provider schema.Provider, user *auth.User) bool {
	if user == nil || len(provider.Groups) == 0 {
		return true
	}
	for _, group := range user.Groups {
		if slices.Contains(provider.Groups, group) {
			return true
		}
	}
	return false
}

func downloaderCandidatesForProviders(providers []schema.Provider, getClient func(string) llm.Client) []downloaderCandidate {
	result := make([]downloaderCandidate, 0, len(providers))
	for _, provider := range providers {
		client := getClient(provider.Name)
		if client == nil {
			continue
		}
		downloader, ok := client.(llm.Downloader)
		if !ok {
			continue
		}
		result = append(result, downloaderCandidate{provider: provider, clientName: client.Name(), downloader: downloader})
	}
	return result
}

func providersFromDownloaderCandidates(candidates []downloaderCandidate) []schema.Provider {
	providers := make([]schema.Provider, 0, len(candidates))
	for _, candidate := range candidates {
		providers = append(providers, candidate.provider)
	}
	return providers
}

func deleteCandidatesForModels(models []schema.Model, candidates []downloaderCandidate) []downloaderCandidate {
	byProvider := make(map[string]llm.Downloader, len(candidates))
	for _, candidate := range candidates {
		byProvider[candidate.provider.Name] = candidate.downloader
	}

	result := make([]downloaderCandidate, 0, len(models))
	for _, model := range models {
		downloader, ok := byProvider[model.OwnedBy]
		if !ok {
			continue
		}
		result = append(result, downloaderCandidate{
			provider:   schema.Provider{Name: model.OwnedBy},
			downloader: downloader,
			model:      model,
			clientName: candidateClientName(candidates, model.OwnedBy),
		})
	}
	return result
}

func candidateClientName(candidates []downloaderCandidate, provider string) string {
	for _, candidate := range candidates {
		if candidate.provider.Name == provider {
			return candidate.clientName
		}
	}
	return ""
}

func isModelNotFound(err error) bool {
	return errors.Is(err, schema.ErrNotFound) || errors.Is(err, httpresponse.ErrNotFound)
}

func isIgnorableGetModelError(err error) bool {
	if err == nil {
		return false
	}
	if isModelNotFound(err) {
		return true
	}

	var httpErr httpresponse.Err
	if errors.As(err, &httpErr) {
		return int(httpErr) >= 400 && int(httpErr) < 500
	}

	if coerced := schema.HTTPErr(err); errors.As(coerced, &httpErr) {
		return int(httpErr) >= 400 && int(httpErr) < 500
	}

	return false
}

func (m *Manager) modelsForProviders(ctx context.Context, providers []schema.Provider) ([]schema.Model, error) {
	var mu sync.Mutex
	var result []schema.Model

	// Fetch models from all providers in parallel, and aggregate results
	group, ctx := errgroup.WithContext(ctx)
	for _, provider := range providers {
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
				if isIgnorableGetModelError(err) {
					models, listErr := m.Registry.GetModels(ctx, &provider)
					if listErr != nil {
						return listErr
					}
					for _, candidate := range models {
						if candidate.Name != name {
							continue
						}

						mu.Lock()
						result = append(result, candidate)
						mu.Unlock()
						return nil
					}
					return nil
				}
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
