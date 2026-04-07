package httphandler

import (
	"context"
	"net/http"

	// Packages
	llmmanager "github.com/mutablelogic/go-llm/kernel/manager"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	opts "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func ProviderHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "provider", nil, httprequest.NewPathItem(
		"Provider operations",
		"List and create operations on providers",
		"Providers",
	).Post(
		func(w http.ResponseWriter, r *http.Request) {
			_ = createProvider(r.Context(), manager, w, r)
		},
		"Create provider",
		opts.WithJSONRequest(jsonschema.MustFor[schema.ProviderInsert]()),
		opts.WithJSONResponse(201, jsonschema.MustFor[schema.Provider]()),
		opts.WithErrorResponse(400, "Invalid request body or provider creation failure."),
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = listProviders(r.Context(), manager, w, r)
		},
		"List providers",
		opts.WithQuery(jsonschema.MustFor[schema.ProviderListRequest]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.ProviderList]()),
		opts.WithErrorResponse(400, "Invalid request parameters or provider listing failure."),
	)
}

func ProviderResourceHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "provider/{name}", nil, httprequest.NewPathItem(
		"Provider operations",
		"Get, update, and delete operations on providers",
		"Providers",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = getProvider(r.Context(), manager, w, r)
		},
		"Get provider",
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Provider]()),
		opts.WithErrorResponse(404, "Provider not found."),
	).Patch(
		func(w http.ResponseWriter, r *http.Request) {
			_ = updateProvider(r.Context(), manager, w, r)
		},
		"Update provider",
		opts.WithJSONRequest(jsonschema.MustFor[schema.ProviderMeta]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Provider]()),
		opts.WithErrorResponse(400, "Invalid request body or provider update failure."),
	).Delete(
		func(w http.ResponseWriter, r *http.Request) {
			_ = deleteProvider(r.Context(), manager, w, r)
		},
		"Delete provider",
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Provider]()),
		opts.WithErrorResponse(404, "Provider not found."),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func listProviders(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.ProviderListRequest
	if err := httprequest.Query(r.URL.Query(), &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	if providers, err := manager.ListProviders(ctx, req); err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	} else {
		return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), providers)
	}
}

func createProvider(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	// Read the request
	var req schema.ProviderInsert
	if err := httprequest.Read(r, &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	// Create the provider
	if provider, err := manager.CreateProvider(ctx, req); err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	} else {
		return httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), provider)
	}
}

func getProvider(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	if provider, err := manager.GetProvider(ctx, r.PathValue("name")); err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	} else {
		return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), provider)
	}
}

func updateProvider(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.ProviderMeta
	if err := httprequest.Read(r, &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	if provider, err := manager.UpdateProvider(ctx, r.PathValue("name"), req); err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	} else {
		return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), provider)
	}
}

func deleteProvider(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	if provider, err := manager.DeleteProvider(ctx, r.PathValue("name")); err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	} else {
		return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), provider)
	}
}
