package httphandler

import (
	"errors"
	"net/http"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// providerName is the path parameter naming a provider
type providerName struct {
	Name string `json:"name" help:"Provider name in the registry"`
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func providerHandlers(r *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	router.Spec().AddTag("Providers", documentation.Section(2, "Provider Operations").Body)
	return errors.Join(
		providerCreateListHandlers(r, router, documentation),
		providerGetDeleteHandlers(r, router, documentation),
	)
}

func providerCreateListHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/provider", nil, func(path httprequest.PathItem) {
		path.Tag("Providers")

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			var req schema.ProviderListRequest
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if list, err := reg.ListProviders(r.Context(), req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), req.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Providers")
			op.Description(documentation.Section(3, "GET /llm/provider").Body)
			op.Query(jsonschema.MustFor[schema.ProviderListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.ProviderList](), "List of providers")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
		})

		// POST
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			var create schema.Provider
			if err := httprequest.Read(r, &create); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
			} else if provider, err := reg.CreateProvider(r.Context(), create); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), create.Name)
			} else {
				httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), provider)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Create Provider")
			op.Description(documentation.Section(3, "POST /llm/provider").Body)
			op.RequestBody(jsonschema.MustFor[schema.Provider]())
			op.JSONResponse(http.StatusCreated, jsonschema.MustFor[schema.ProviderStatus](), "Created provider")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, unknown provider type, or a credential which isn't set")
			op.ErrorResponse(http.StatusConflict, "A provider with that name exists already")
			op.ErrorResponse(http.StatusServiceUnavailable, "Provider can't be reached")
		})
	})
}

func providerGetDeleteHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/provider/{name}", jsonschema.MustFor[providerName](), func(path httprequest.PathItem) {
		path.Tag("Providers")

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			if provider, err := reg.GetProvider(r.Context(), name); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), provider)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Provider")
			op.Description(documentation.Section(3, "GET /llm/provider/{name}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.ProviderStatus](), "Provider")
			op.ErrorResponse(http.StatusNotFound, "Provider not found")
		})

		// PATCH
		path.Patch(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			var meta schema.ProviderMeta
			if err := httprequest.Read(r, &meta); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
			} else if provider, err := reg.UpdateProvider(r.Context(), name, meta); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), provider)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Update Provider")
			op.Description(documentation.Section(3, "PATCH /llm/provider/{name}").Body)
			op.RequestBody(jsonschema.MustFor[schema.ProviderMeta]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.ProviderStatus](), "Updated provider")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
			op.ErrorResponse(http.StatusNotFound, "Provider not found")
			op.ErrorResponse(http.StatusConflict, "The provider was added without a definition, so can't be updated")
		})

		// DELETE
		path.Delete(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			if provider, err := reg.DeleteProvider(r.Context(), name); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), provider)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Delete Provider")
			op.Description(documentation.Section(3, "DELETE /llm/provider/{name}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.ProviderStatus](), "Deleted provider")
			op.ErrorResponse(http.StatusNotFound, "Provider not found")
		})
	})
}
