package httphandler

import (
	"context"
	"net/http"

	// Packages
	middleware "github.com/djthorpe/go-auth/pkg/middleware"
	llmmanager "github.com/mutablelogic/go-llm/pkg/llmmanager"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	opts "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func EmbeddingHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "embedding", nil, httprequest.NewPathItem(
		"Embedding operations",
		"Generate embedding vectors for text input",
		"Respond",
	).Post(
		func(w http.ResponseWriter, r *http.Request) {
			_ = embedding(r.Context(), manager, w, r)
		},
		"Create embeddings",
		opts.WithJSONRequest(jsonschema.MustFor[schema.EmbeddingRequest]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.EmbeddingResponse]()),
		opts.WithErrorResponse(400, "Invalid request body or embedding failure."),
		opts.WithErrorResponse(404, "Model or provider not found."),
		opts.WithErrorResponse(409, "Multiple models matched; specify a provider."),
		opts.WithErrorResponse(501, "Provider does not support embeddings."),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func embedding(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.EmbeddingRequest
	if err := httprequest.Read(r, &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	resp, err := manager.Embedding(ctx, req, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
}
