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

func ToolHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "tool", nil, httprequest.NewPathItem(
		"Tool operations",
		"List operations on tools",
		"Tool",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = listTools(r.Context(), manager, w, r)
		},
		"List tools",
		opts.WithQuery(jsonschema.MustFor[schema.ToolListRequest]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.ToolList]()),
		opts.WithErrorResponse(400, "Invalid request parameters or tool listing failure."),
	)
}

func ToolResourceHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "tool/{name}", nil, httprequest.NewPathItem(
		"Tool operations",
		"Get operations on tools",
		"Tool",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = getTool(r.Context(), manager, w, r)
		},
		"Get tool",
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.ToolMeta]()),
		opts.WithErrorResponse(404, "Tool not found."),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func listTools(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.ToolListRequest
	if err := httprequest.Query(r.URL.Query(), &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	tools, err := manager.ListTools(ctx, req, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), tools)
}

func getTool(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	tool, err := manager.GetTool(ctx, r.PathValue("name"), middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), tool)
}
