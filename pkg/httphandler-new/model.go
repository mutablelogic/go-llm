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

func ModelHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "model", nil, httprequest.NewPathItem(
		"Model operations",
		"List operations on models",
		"Model",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = listModels(r.Context(), manager, w, r)
		},
		"List models",
		opts.WithQuery(jsonschema.MustFor[schema.ModelListRequest]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.ModelList]()),
		opts.WithErrorResponse(400, "Invalid request parameters or model listing failure."),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func listModels(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.ModelListRequest
	if err := httprequest.Query(r.URL.Query(), &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	if models, err := manager.ListModels(ctx, req, middleware.UserFromContext(ctx)); err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	} else {
		return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), models)
	}
}
