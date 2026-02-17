package manager

import (
	"context"

	// Packages
	"github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	"go.opentelemetry.io/otel/attribute"
)

func (m *Manager) Embedding(ctx context.Context, request *schema.EmbeddingRequest) (response *schema.EmbeddingResponse, err error) {
	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "Embedding",
		attribute.String("request", request.String()),
	)
	defer func() { endSpan(err) }()

	// Get the model from the request
	model, err := m.getModel(ctx, request.Provider, request.Model)
	if err != nil {
		return nil, err
	}

	// Get the client for the model
	client := m.clientForModel(model)
	if client == nil {
		return nil, llm.ErrNotFound.Withf("no provider found for model: %s", request.Model)
	} else if _, ok := client.(llm.Embedder); !ok {
		return nil, llm.ErrNotImplemented.Withf("provider %q does not support embeddings", client.Name())
	} else if len(request.Input) == 0 {
		return nil, llm.ErrBadParameter.With("input text is required for embedding")
	}

	// Create options for the embedding request
	if request.TaskType == "" {
		request.TaskType = schema.EmbeddingTaskTypeDefault
	}
	opts, err := convertOptsForClient([]opt.Opt{
		WithOutputDimensionality(request.OutputDimensionality),
		WithTitle(request.Title),
		WithTaskType(request.TaskType),
	}, client)
	if err != nil {
		return nil, err
	}

	// Create a response
	response = types.Ptr(schema.EmbeddingResponse{
		EmbeddingRequest: schema.EmbeddingRequest{
			Provider: client.Name(),
			Model:    model.Name,
			Input:    request.Input,
			TaskType: request.TaskType,
			Title:    request.Title,
		},
	})

	// Use Embedding or BatchEmbedding based on the number of input texts
	if len(request.Input) == 1 {
		var embedding []float64
		embedding, err = client.(llm.Embedder).Embedding(ctx, types.Value(model), request.Input[0], opts...)
		if err != nil {
			return nil, err
		}
		response.OutputDimensionality = uint(len(embedding))
		response.Output = [][]float64{embedding}
	} else if len(request.Input) > 1 {
		var embeddings [][]float64
		embeddings, err = client.(llm.Embedder).BatchEmbedding(ctx, types.Value(model), request.Input, opts...)
		if err != nil {
			return nil, err
		}
		if len(embeddings) > 0 {
			response.OutputDimensionality = uint(len(embeddings[0]))
		}
		response.Output = embeddings
	}

	// Return success
	return response, nil
}
