package manager

import (
	"context"
	"strings"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	google "github.com/mutablelogic/go-llm/provider/google"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Embedding resolves an embedding-capable model for the user-scoped request and
// returns one output vector per input string.
func (m *Manager) Embedding(ctx context.Context, request schema.EmbeddingRequest, user *auth.User) (_ *schema.EmbeddingResponse, err error) {
	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "Embedding",
		attribute.String("request", request.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	if len(request.Input) == 0 {
		return nil, schema.ErrBadParameter.With("input text is required for embedding")
	}

	// Get candidate providers for user, or all candidates if no user is provided.
	providers, err := m.providersForUser(ctx, request.Provider, user)
	if err != nil {
		return nil, err
	}

	// Resolve the model to exactly one provider-scoped match.
	models, err := m.modelsByName(ctx, providers, request.Model)
	if err != nil {
		return nil, err
	}
	var model *schema.Model
	var provider *schema.Provider
	if len(models) == 0 {
		return nil, schema.ErrNotFound.Withf("model %q not found", request.Model)
	} else if len(models) > 1 {
		return nil, schema.ErrConflict.Withf("multiple models named %q found; specify a provider", request.Model)
	} else {
		model = types.Ptr(models[0])
		for i := range providers {
			if providers[i].Name == model.OwnedBy {
				provider = &providers[i]
				break
			}
		}
	}
	if provider == nil {
		return nil, schema.ErrNotFound.Withf("provider %q not found for model: %s", model.OwnedBy, request.Model)
	}

	client := m.Registry.Get(provider.Name)
	if client == nil {
		return nil, schema.ErrNotFound.Withf("no provider found for model: %s", request.Model)
	}
	embedder, ok := client.(llm.Embedder)
	if !ok {
		return nil, schema.ErrNotImplemented.Withf("provider %q does not support embeddings", provider.Name)
	}

	request.TaskType = strings.TrimSpace(request.TaskType)

	opts, err := convertOptsForClient(embeddingOptsFromRequest(request), client)
	if err != nil {
		return nil, err
	}
	if request.TaskType == "" {
		request.TaskType = schema.EmbeddingTaskTypeDefault
	}

	response := &schema.EmbeddingResponse{
		EmbeddingRequest: schema.EmbeddingRequest{
			Provider:             provider.Name,
			Model:                model.Name,
			Input:                request.Input,
			TaskType:             request.TaskType,
			Title:                request.Title,
			OutputDimensionality: request.OutputDimensionality,
		},
	}

	if len(request.Input) == 1 {
		var embedding []float64
		var usage *schema.UsageMeta
		embedding, usage, err = embedder.Embedding(ctx, types.Value(model), request.Input[0], opts...)
		if err != nil {
			return nil, err
		}
		response.OutputDimensionality = uint(len(embedding))
		response.Output = [][]float64{embedding}
		response.Usage = mergeUsageMeta(ctx, usage, provider.Meta, nil)
	} else {
		var embeddings [][]float64
		var usage *schema.UsageMeta
		embeddings, usage, err = embedder.BatchEmbedding(ctx, types.Value(model), request.Input, opts...)
		if err != nil {
			return nil, err
		}
		if len(embeddings) > 0 {
			response.OutputDimensionality = uint(len(embeddings[0]))
		}
		response.Output = embeddings
		response.Usage = mergeUsageMeta(ctx, usage, provider.Meta, nil)
	}

	if response.Usage != nil {
		if _, err := m.CreateUsage(ctx, schema.UsageInsert{
			Type:      schema.UsageTypeEmbedding,
			User:      user.UUID(),
			Model:     model.Name,
			Provider:  types.Ptr(model.OwnedBy),
			UsageMeta: types.Value(response.Usage),
		}); err != nil {
			return nil, err
		}
	}

	return response, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func embeddingOptsFromRequest(request schema.EmbeddingRequest) []opt.Opt {
	var opts []opt.Opt
	if request.OutputDimensionality > 0 {
		opts = append(opts, withEmbeddingOutputDimensionality(request.OutputDimensionality))
	}
	if strings.TrimSpace(request.Title) != "" {
		opts = append(opts, withEmbeddingTitle(request.Title))
	}
	if request.TaskType != "" {
		opts = append(opts, withEmbeddingTaskType(request.TaskType))
	}
	return opts
}

func withEmbeddingOutputDimensionality(dim uint) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithOutputDimensionality(dim)
		default:
			return opt.Error(schema.ErrNotImplemented.Withf("%s: WithOutputDimensionality not supported", provider))
		}
	})
}

func withEmbeddingTitle(title string) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithTitle(title)
		default:
			return opt.Error(schema.ErrNotImplemented.Withf("%s: WithTitle not supported", provider))
		}
	})
}

func withEmbeddingTaskType(taskType string) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithTaskType(taskType)
		default:
			return opt.Error(schema.ErrNotImplemented.Withf("%s: WithTaskType not supported", provider))
		}
	})
}
