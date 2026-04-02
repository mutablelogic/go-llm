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

func ModelResourceHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "model/{name}", jsonschema.MustFor[schema.ModelNameSelector](), httprequest.NewPathItem(
		"Model operations",
		"Get operations on models",
		"Model",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = getModel(r.Context(), manager, w, r, "")
		},
		"Get model",
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Model]()),
		opts.WithErrorResponse(404, "Model not found."),
	)
}

func ModelProviderResourceHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "model/{provider}/{name}", jsonschema.MustFor[schema.ModelProviderSelector](), httprequest.NewPathItem(
		"Model operations",
		"Get operations on models with an explicit provider",
		"Model",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = getModel(r.Context(), manager, w, r, r.PathValue("provider"))
		},
		"Get model for provider",
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Model]()),
		opts.WithErrorResponse(404, "Model not found."),
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

func getModel(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request, provider string) error {
	req := schema.GetModelRequest{
		Provider: provider,
		Name:     r.PathValue("name"),
	}

	if model, err := manager.GetModel(ctx, req, middleware.UserFromContext(ctx)); err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	} else {
		return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), model)
	}
}
