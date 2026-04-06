package httphandler

import (
	"context"
	"net/http"

	// Packages
	middleware "github.com/djthorpe/go-auth/pkg/middleware"
	llmmanager "github.com/mutablelogic/go-llm/pkg/manager"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	opts "github.com/mutablelogic/go-server/pkg/openapi"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func ModelHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "model", nil, httprequest.NewPathItem(
		"Model operations",
		"List and download operations on models",
		"Models",
	).Post(
		func(w http.ResponseWriter, r *http.Request) {
			_ = downloadModel(r.Context(), manager, w, r)
		},
		"Download model",
		opts.WithJSONRequest(jsonschema.MustFor[schema.DownloadModelRequest]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Model]()),
		opts.WithTextStreamResponse(200, "SSE stream of progress, error, and result events."),
		opts.WithErrorResponse(400, "Invalid request body or model download failure."),
		opts.WithErrorResponse(406, "Unsupported Accept header."),
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
		"Get and delete operations on models",
		"Models",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = getModel(r.Context(), manager, w, r, "")
		},
		"Get model",
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Model]()),
		opts.WithErrorResponse(404, "Model not found."),
	).Delete(
		func(w http.ResponseWriter, r *http.Request) {
			_ = deleteModel(r.Context(), manager, w, r, "")
		},
		"Delete model",
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Model]()),
		opts.WithErrorResponse(404, "Model not found."),
	)
}

func ModelProviderResourceHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "model/{provider}/{name}", jsonschema.MustFor[schema.ModelProviderSelector](), httprequest.NewPathItem(
		"Model operations",
		"Get and delete operations on models with an explicit provider",
		"Models",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = getModel(r.Context(), manager, w, r, r.PathValue("provider"))
		},
		"Get model for provider",
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Model]()),
		opts.WithErrorResponse(404, "Model not found."),
	).Delete(
		func(w http.ResponseWriter, r *http.Request) {
			_ = deleteModel(r.Context(), manager, w, r, r.PathValue("provider"))
		},
		"Delete model for provider",
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

func deleteModel(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request, provider string) error {
	req := schema.DeleteModelRequest{
		Provider: provider,
		Name:     r.PathValue("name"),
	}

	model, err := manager.DeleteModel(ctx, req, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}
	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), model)
}

func downloadModel(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.DownloadModelRequest
	if err := httprequest.Read(r, &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	// Determine the accepted response content type
	accept, err := types.AcceptContentType(r)
	if err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	// Respond
	switch accept {
	case types.ContentTypeTextStream:
		stream := httpresponse.NewTextStream(w)
		if stream == nil {
			return httpresponse.Error(w, httpresponse.ErrInternalError)
		}
		defer stream.Close()

		progressFn := opt.ProgressFn(func(status string, percent float64) {
			stream.Write(schema.EventProgress, schema.ProgressEvent{Status: status, Percent: percent})
		})

		model, err := manager.DownloadModel(ctx, req, middleware.UserFromContext(ctx), opt.WithProgress(progressFn))
		if err != nil {
			stream.Write(schema.EventError, schema.StreamError{Error: err.Error()})
			return nil
		}
		stream.Write(schema.EventResult, model)
		return nil
	case types.ContentTypeJSON, types.ContentTypeAny:
		model, err := manager.DownloadModel(ctx, req, middleware.UserFromContext(ctx))
		if err != nil {
			return httpresponse.Error(w, schema.HTTPErr(err))
		}
		return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), model)
	default:
		return httpresponse.Error(w, httpresponse.Err(http.StatusNotAcceptable))
	}
}
