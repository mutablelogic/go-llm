package manager

import (
	"context"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (m *Manager) ListModels(ctx context.Context, req schema.ModelListRequest, user *auth.User) (result *schema.ModelList, err error) {
	// Otel
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "ListModels",
		attribute.String("request", req.String()),
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

	// Return success
	return &schema.ModelList{
		ModelListRequest: req,
		Provider:         providerNames,
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (m *Manager) providersForUser(ctx context.Context, provider string, user *auth.User) ([]schema.Provider, error) {
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
	return result, nil
}
